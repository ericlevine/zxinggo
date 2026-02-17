package decoder

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/internal"
)

// Text compaction sub-modes
type textMode int

const (
	textModeAlpha      textMode = iota
	textModeLower
	textModeMixed
	textModePunct
	textModeAlphaShift
	textModePunctShift
)

// Mode latch and shift constants
const (
	textCompactionModeLatch          = 900
	byteCompactionModeLatch          = 901
	numericCompactionModeLatch       = 902
	byteCompactionModeLatch6         = 924
	eciUserDefined                   = 925
	eciGeneralPurpose                = 926
	eciCharset                       = 927
	beginMacroPDF417ControlBlock     = 928
	beginMacroPDF417OptionalField    = 923
	macroPDF417Terminator            = 922
	modeShiftToByteCompactionMode    = 913
	maxNumericCodewords              = 15

	macroPDF417OptionalFieldFileName     = 0
	macroPDF417OptionalFieldSegmentCount = 1
	macroPDF417OptionalFieldTimeStamp    = 2
	macroPDF417OptionalFieldSender       = 3
	macroPDF417OptionalFieldAddressee    = 4
	macroPDF417OptionalFieldFileSize     = 5
	macroPDF417OptionalFieldChecksum     = 6

	tcPL  = 25
	tcLL  = 27
	tcAS  = 27
	tcML  = 28
	tcAL  = 28
	tcPS  = 29
	tcPAL = 29

	numberOfSequenceCodewords = 2
)

var punctChars = []byte(";<>@[\\]_`~!\r\t,:\n-.$/\"|*()?{}'")
var mixedChars = []byte("0123456789&\r\t,:#-.$/+%*=^")

// exp900 holds powers of 900 as big.Int for numeric compaction decoding.
var exp900 [16]*big.Int

func init() {
	exp900[0] = big.NewInt(1)
	exp900[1] = big.NewInt(900)
	for i := 2; i < len(exp900); i++ {
		exp900[i] = new(big.Int).Mul(exp900[i-1], exp900[1])
	}
}

// PDF417ResultMetadata holds metadata for macro PDF417 barcodes.
type PDF417ResultMetadata struct {
	SegmentIndex int
	FileID       string
	OptionalData []int
	LastSegment  bool
	SegmentCount int
	FileName     string
	Sender       string
	Addressee    string
	Timestamp    int64
	FileSize     int64
	Checksum     int
}

// decodeBitStream decodes PDF417 codewords into a DecoderResult.
func decodeBitStream(codewords []int, ecLevel string) (*internal.DecoderResult, error) {
	var result strings.Builder
	result.Grow(len(codewords) * 2)

	codeIndex, err := textCompaction(codewords, 1, &result)
	if err != nil {
		return nil, err
	}
	resultMetadata := &PDF417ResultMetadata{}
	for codeIndex < codewords[0] {
		code := codewords[codeIndex]
		codeIndex++
		switch code {
		case textCompactionModeLatch:
			codeIndex, err = textCompaction(codewords, codeIndex, &result)
			if err != nil {
				return nil, err
			}
		case byteCompactionModeLatch, byteCompactionModeLatch6:
			codeIndex, err = byteCompaction(code, codewords, codeIndex, &result)
			if err != nil {
				return nil, err
			}
		case modeShiftToByteCompactionMode:
			result.WriteByte(byte(codewords[codeIndex]))
			codeIndex++
		case numericCompactionModeLatch:
			codeIndex, err = numericCompaction(codewords, codeIndex, &result)
			if err != nil {
				return nil, err
			}
		case eciCharset:
			// Skip ECI charset indicator and its value
			codeIndex++
		case eciGeneralPurpose:
			// Skip generic ECI; skip its 2 characters
			codeIndex += 2
		case eciUserDefined:
			// Skip user ECI; skip its 1 character
			codeIndex++
		case beginMacroPDF417ControlBlock:
			codeIndex, err = decodeMacroBlock(codewords, codeIndex, resultMetadata)
			if err != nil {
				return nil, err
			}
		case beginMacroPDF417OptionalField, macroPDF417Terminator:
			// Should not see these outside a macro block
			return nil, zxinggo.ErrFormat
		default:
			// Default to text compaction. During testing numerous barcodes
			// appeared to be missing the starting mode.
			codeIndex--
			codeIndex, err = textCompaction(codewords, codeIndex, &result)
			if err != nil {
				return nil, err
			}
		}
	}
	if result.Len() == 0 && resultMetadata.FileID == "" {
		return nil, zxinggo.ErrFormat
	}
	dr := internal.NewDecoderResult(nil, result.String(), nil, ecLevel)
	dr.Other = resultMetadata
	return dr, nil
}

func decodeMacroBlock(codewords []int, codeIndex int, resultMetadata *PDF417ResultMetadata) (int, error) {
	if codeIndex+numberOfSequenceCodewords > codewords[0] {
		return 0, zxinggo.ErrFormat
	}
	segmentIndexArray := make([]int, numberOfSequenceCodewords)
	for i := 0; i < numberOfSequenceCodewords; i++ {
		segmentIndexArray[i] = codewords[codeIndex]
		codeIndex++
	}
	segmentIndexString, err := decodeBase900toBase10(segmentIndexArray, numberOfSequenceCodewords)
	if err != nil {
		return 0, err
	}
	if segmentIndexString == "" {
		resultMetadata.SegmentIndex = 0
	} else {
		val, err := strconv.Atoi(segmentIndexString)
		if err != nil {
			return 0, zxinggo.ErrFormat
		}
		resultMetadata.SegmentIndex = val
	}

	// Decode the fileId codewords as 0-899 numbers, each 0-filled to width 3.
	var fileID strings.Builder
	for codeIndex < codewords[0] &&
		codeIndex < len(codewords) &&
		codewords[codeIndex] != macroPDF417Terminator &&
		codewords[codeIndex] != beginMacroPDF417OptionalField {
		fileID.WriteString(fmt.Sprintf("%03d", codewords[codeIndex]))
		codeIndex++
	}
	if fileID.Len() == 0 {
		return 0, zxinggo.ErrFormat
	}
	resultMetadata.FileID = fileID.String()

	optionalFieldsStart := -1
	if codeIndex < len(codewords) && codewords[codeIndex] == beginMacroPDF417OptionalField {
		optionalFieldsStart = codeIndex + 1
	}

	for codeIndex < codewords[0] {
		switch codewords[codeIndex] {
		case beginMacroPDF417OptionalField:
			codeIndex++
			switch codewords[codeIndex] {
			case macroPDF417OptionalFieldFileName:
				var fileName strings.Builder
				var err error
				codeIndex, err = textCompaction(codewords, codeIndex+1, &fileName)
				if err != nil {
					return 0, err
				}
				resultMetadata.FileName = fileName.String()
			case macroPDF417OptionalFieldSender:
				var sender strings.Builder
				var err error
				codeIndex, err = textCompaction(codewords, codeIndex+1, &sender)
				if err != nil {
					return 0, err
				}
				resultMetadata.Sender = sender.String()
			case macroPDF417OptionalFieldAddressee:
				var addressee strings.Builder
				var err error
				codeIndex, err = textCompaction(codewords, codeIndex+1, &addressee)
				if err != nil {
					return 0, err
				}
				resultMetadata.Addressee = addressee.String()
			case macroPDF417OptionalFieldSegmentCount:
				var segmentCount strings.Builder
				var err error
				codeIndex, err = numericCompaction(codewords, codeIndex+1, &segmentCount)
				if err != nil {
					return 0, err
				}
				val, err := strconv.Atoi(segmentCount.String())
				if err != nil {
					return 0, zxinggo.ErrFormat
				}
				resultMetadata.SegmentCount = val
			case macroPDF417OptionalFieldTimeStamp:
				var timestamp strings.Builder
				var err error
				codeIndex, err = numericCompaction(codewords, codeIndex+1, &timestamp)
				if err != nil {
					return 0, err
				}
				val, err := strconv.ParseInt(timestamp.String(), 10, 64)
				if err != nil {
					return 0, zxinggo.ErrFormat
				}
				resultMetadata.Timestamp = val
			case macroPDF417OptionalFieldChecksum:
				var checksum strings.Builder
				var err error
				codeIndex, err = numericCompaction(codewords, codeIndex+1, &checksum)
				if err != nil {
					return 0, err
				}
				val, err := strconv.Atoi(checksum.String())
				if err != nil {
					return 0, zxinggo.ErrFormat
				}
				resultMetadata.Checksum = val
			case macroPDF417OptionalFieldFileSize:
				var fileSize strings.Builder
				var err error
				codeIndex, err = numericCompaction(codewords, codeIndex+1, &fileSize)
				if err != nil {
					return 0, err
				}
				val, err := strconv.ParseInt(fileSize.String(), 10, 64)
				if err != nil {
					return 0, zxinggo.ErrFormat
				}
				resultMetadata.FileSize = val
			default:
				return 0, zxinggo.ErrFormat
			}
		case macroPDF417Terminator:
			codeIndex++
			resultMetadata.LastSegment = true
		default:
			return 0, zxinggo.ErrFormat
		}
	}

	// Copy optional fields to additional options.
	if optionalFieldsStart != -1 {
		optionalFieldsLength := codeIndex - optionalFieldsStart
		if resultMetadata.LastSegment {
			optionalFieldsLength--
		}
		if optionalFieldsLength > 0 {
			resultMetadata.OptionalData = make([]int, optionalFieldsLength)
			copy(resultMetadata.OptionalData, codewords[optionalFieldsStart:optionalFieldsStart+optionalFieldsLength])
		}
	}

	return codeIndex, nil
}

// textCompaction handles the Text Compaction mode of PDF417.
func textCompaction(codewords []int, codeIndex int, result *strings.Builder) (int, error) {
	// 2 characters per codeword
	size := (codewords[0] - codeIndex) * 2
	if size < 0 {
		size = 0
	}
	textCompactionData := make([]int, size)
	byteCompactionData := make([]int, size)

	index := 0
	end := false
	subMode := textModeAlpha
	for codeIndex < codewords[0] && !end {
		code := codewords[codeIndex]
		codeIndex++
		if code < textCompactionModeLatch {
			textCompactionData[index] = code / 30
			textCompactionData[index+1] = code % 30
			index += 2
		} else {
			switch code {
			case textCompactionModeLatch:
				textCompactionData[index] = textCompactionModeLatch
				index++
			case byteCompactionModeLatch, byteCompactionModeLatch6,
				numericCompactionModeLatch, beginMacroPDF417ControlBlock,
				beginMacroPDF417OptionalField, macroPDF417Terminator:
				codeIndex--
				end = true
			case modeShiftToByteCompactionMode:
				textCompactionData[index] = modeShiftToByteCompactionMode
				code = codewords[codeIndex]
				codeIndex++
				byteCompactionData[index] = code
				index++
			case eciCharset:
				subMode = decodeTextCompaction(textCompactionData, byteCompactionData, index, result, subMode)
				// Skip the ECI value
				codeIndex++
				if codeIndex > codewords[0] {
					return 0, zxinggo.ErrFormat
				}
				newSize := (codewords[0] - codeIndex) * 2
				if newSize < 0 {
					newSize = 0
				}
				textCompactionData = make([]int, newSize)
				byteCompactionData = make([]int, newSize)
				index = 0
			}
		}
	}
	decodeTextCompaction(textCompactionData, byteCompactionData, index, result, subMode)
	return codeIndex, nil
}

// decodeTextCompaction decodes text compaction data and appends to result.
func decodeTextCompaction(textCompactionData, byteCompactionData []int, length int,
	result *strings.Builder, startMode textMode) textMode {

	subMode := startMode
	priorToShiftMode := startMode
	latchedMode := startMode
	i := 0
	for i < length {
		subModeCh := textCompactionData[i]
		var ch byte
		switch subMode {
		case textModeAlpha:
			if subModeCh < 26 {
				ch = byte('A' + subModeCh)
			} else {
				switch subModeCh {
				case 26:
					ch = ' '
				case tcLL:
					subMode = textModeLower
					latchedMode = subMode
				case tcML:
					subMode = textModeMixed
					latchedMode = subMode
				case tcPS:
					priorToShiftMode = subMode
					subMode = textModePunctShift
				case modeShiftToByteCompactionMode:
					result.WriteByte(byte(byteCompactionData[i]))
				case textCompactionModeLatch:
					subMode = textModeAlpha
					latchedMode = subMode
				}
			}

		case textModeLower:
			if subModeCh < 26 {
				ch = byte('a' + subModeCh)
			} else {
				switch subModeCh {
				case 26:
					ch = ' '
				case tcAS:
					priorToShiftMode = subMode
					subMode = textModeAlphaShift
				case tcML:
					subMode = textModeMixed
					latchedMode = subMode
				case tcPS:
					priorToShiftMode = subMode
					subMode = textModePunctShift
				case modeShiftToByteCompactionMode:
					result.WriteByte(byte(byteCompactionData[i]))
				case textCompactionModeLatch:
					subMode = textModeAlpha
					latchedMode = subMode
				}
			}

		case textModeMixed:
			if subModeCh < tcPL {
				ch = mixedChars[subModeCh]
			} else {
				switch subModeCh {
				case tcPL:
					subMode = textModePunct
					latchedMode = subMode
				case 26:
					ch = ' '
				case tcLL:
					subMode = textModeLower
					latchedMode = subMode
				case tcAL, textCompactionModeLatch:
					subMode = textModeAlpha
					latchedMode = subMode
				case tcPS:
					priorToShiftMode = subMode
					subMode = textModePunctShift
				case modeShiftToByteCompactionMode:
					result.WriteByte(byte(byteCompactionData[i]))
				}
			}

		case textModePunct:
			if subModeCh < tcPAL {
				ch = punctChars[subModeCh]
			} else {
				switch subModeCh {
				case tcPAL, textCompactionModeLatch:
					subMode = textModeAlpha
					latchedMode = subMode
				case modeShiftToByteCompactionMode:
					result.WriteByte(byte(byteCompactionData[i]))
				}
			}

		case textModeAlphaShift:
			subMode = priorToShiftMode
			if subModeCh < 26 {
				ch = byte('A' + subModeCh)
			} else {
				switch subModeCh {
				case 26:
					ch = ' '
				case textCompactionModeLatch:
					subMode = textModeAlpha
				}
			}

		case textModePunctShift:
			subMode = priorToShiftMode
			if subModeCh < tcPAL {
				ch = punctChars[subModeCh]
			} else {
				switch subModeCh {
				case tcPAL, textCompactionModeLatch:
					subMode = textModeAlpha
				case modeShiftToByteCompactionMode:
					result.WriteByte(byte(byteCompactionData[i]))
				}
			}
		}
		if ch != 0 {
			result.WriteByte(ch)
		}
		i++
	}
	_ = latchedMode // latchedMode tracks the latched state for return
	return latchedMode
}

// byteCompaction handles the Byte Compaction mode of PDF417.
func byteCompaction(mode int, codewords []int, codeIndex int, result *strings.Builder) (int, error) {
	end := false

	for codeIndex < codewords[0] && !end {
		// Handle leading ECIs
		for codeIndex < codewords[0] && codewords[codeIndex] == eciCharset {
			codeIndex++ // skip ECI indicator
			codeIndex++ // skip ECI value
		}

		if codeIndex >= codewords[0] || codewords[codeIndex] >= textCompactionModeLatch {
			end = true
		} else {
			// Decode one block of 5 codewords to 6 bytes
			var value int64
			count := 0
			for {
				value = 900*value + int64(codewords[codeIndex])
				codeIndex++
				count++
				if count >= 5 || codeIndex >= codewords[0] || codewords[codeIndex] >= textCompactionModeLatch {
					break
				}
			}
			if count == 5 && (mode == byteCompactionModeLatch6 ||
				(codeIndex < codewords[0] && codewords[codeIndex] < textCompactionModeLatch)) {
				for i := 0; i < 6; i++ {
					result.WriteByte(byte(value >> uint(8*(5-i))))
				}
			} else {
				codeIndex -= count
				for codeIndex < codewords[0] && !end {
					code := codewords[codeIndex]
					codeIndex++
					if code < textCompactionModeLatch {
						result.WriteByte(byte(code))
					} else if code == eciCharset {
						codeIndex++ // skip ECI value
					} else {
						codeIndex--
						end = true
					}
				}
			}
		}
	}
	return codeIndex, nil
}

// numericCompaction handles the Numeric Compaction mode of PDF417.
func numericCompaction(codewords []int, codeIndex int, result *strings.Builder) (int, error) {
	count := 0
	end := false

	numericCodewords := make([]int, maxNumericCodewords)

	for codeIndex < codewords[0] && !end {
		code := codewords[codeIndex]
		codeIndex++
		if codeIndex == codewords[0] {
			end = true
		}
		if code < textCompactionModeLatch {
			numericCodewords[count] = code
			count++
		} else {
			switch code {
			case textCompactionModeLatch, byteCompactionModeLatch,
				byteCompactionModeLatch6, beginMacroPDF417ControlBlock,
				beginMacroPDF417OptionalField, macroPDF417Terminator, eciCharset:
				codeIndex--
				end = true
			}
		}
		if (count%maxNumericCodewords == 0 || code == numericCompactionModeLatch || end) && count > 0 {
			s, err := decodeBase900toBase10(numericCodewords, count)
			if err != nil {
				return 0, err
			}
			result.WriteString(s)
			count = 0
		}
	}
	return codeIndex, nil
}

// decodeBase900toBase10 converts numeric compaction codewords from base 900 to base 10.
func decodeBase900toBase10(codewords []int, count int) (string, error) {
	result := new(big.Int)
	for i := 0; i < count; i++ {
		term := new(big.Int).Mul(exp900[count-i-1], big.NewInt(int64(codewords[i])))
		result.Add(result, term)
	}
	resultString := result.String()
	if len(resultString) == 0 || resultString[0] != '1' {
		return "", zxinggo.ErrFormat
	}
	return resultString[1:], nil
}
