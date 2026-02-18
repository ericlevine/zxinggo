// Package decoder implements the Aztec barcode decoder.
//
// It takes a BitMatrix (the sampled grid from the detector) along with
// structural parameters (compact mode, layer count, data-block count)
// and produces the decoded text.
//
// The algorithm follows the ZXing Java reference implementation:
//  1. Extract raw bits from the concentric data layers.
//  2. Correct errors using Reed-Solomon over the appropriate Galois Field.
//  3. Extract the data bits from the corrected codewords.
//  4. Decode the resulting bit stream using the Aztec 5-mode encoding tables.
package decoder

import (
	"strings"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
	"github.com/ericlevine/zxinggo/charset"
	"github.com/ericlevine/zxinggo/reedsolomon"
)

// ---------------------------------------------------------------------------
// Result types
// ---------------------------------------------------------------------------

// AztecDetectorResult carries the output of the Aztec detector that the
// decoder needs: the sampled bit matrix, the corner/center result points,
// and the structural parameters read from the mode message.
type AztecDetectorResult struct {
	Bits         *bitutil.BitMatrix
	Points       []zxinggo.ResultPoint
	Compact      bool
	NbDataBlocks int
	NbLayers     int
}

// DecoderResult holds the final decoded text and raw bytes.
type DecoderResult struct {
	Text            string
	RawBytes        []byte
	ErrorsCorrected int
}

// ---------------------------------------------------------------------------
// Encoding-mode constants (matching Java ZXing's Table enum)
// ---------------------------------------------------------------------------

const (
	tableUpper  = iota
	tableLower
	tableMixed
	tableDigit
	tablePunct
	tableBinary
)

// String tables matching Java ZXing Decoder exactly.
// CTRL_ prefixed entries are table-change commands:
//   CTRL_XY where X = table initial (U/L/M/D/P/B), Y = S (shift) or L (latch)

var upperTable = [32]string{
	"CTRL_PS", " ", "A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P",
	"Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z", "CTRL_LL", "CTRL_ML", "CTRL_DL", "CTRL_BS",
}

var lowerTable = [32]string{
	"CTRL_PS", " ", "a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p",
	"q", "r", "s", "t", "u", "v", "w", "x", "y", "z", "CTRL_US", "CTRL_ML", "CTRL_DL", "CTRL_BS",
}

var mixedTable = [32]string{
	"CTRL_PS", " ", "\x01", "\x02", "\x03", "\x04", "\x05", "\x06", "\x07", "\b", "\t", "\n",
	"\x0b", "\f", "\r", "\x1b", "\x1c", "\x1d", "\x1e", "\x1f", "@", "\\", "^", "_",
	"`", "|", "~", "\x7f", "CTRL_LL", "CTRL_UL", "CTRL_PL", "CTRL_BS",
}

var punctTable = [32]string{
	"FLG(n)", "\r", "\r\n", ". ", ", ", ": ", "!", "\"", "#", "$", "%", "&", "'", "(", ")",
	"*", "+", ",", "-", ".", "/", ":", ";", "<", "=", ">", "?", "[", "]", "{", "}", "CTRL_UL",
}

var digitTable = [16]string{
	"CTRL_PS", " ", "0", "1", "2", "3", "4", "5", "6", "7", "8", "9", ",", ".", "CTRL_UL", "CTRL_US",
}

// ---------------------------------------------------------------------------
// Public entry point
// ---------------------------------------------------------------------------

// Decode decodes an Aztec symbol described by the given detector result.
func Decode(detectorResult *AztecDetectorResult) (*DecoderResult, error) {
	rawbits := extractBits(detectorResult)

	correctedBits, errorsCorrected, err := correctBits(detectorResult, rawbits)
	if err != nil {
		return nil, err
	}

	text, rawBytes, err := getEncodedData(correctedBits)
	if err != nil {
		return nil, err
	}

	return &DecoderResult{
		Text:            text,
		RawBytes:        rawBytes,
		ErrorsCorrected: errorsCorrected,
	}, nil
}

// ---------------------------------------------------------------------------
// Reed-Solomon error correction
// ---------------------------------------------------------------------------

// codewordSize returns the number of bits per codeword for the symbol.
func codewordSize(nbLayers int) int {
	if nbLayers <= 2 {
		return 6
	}
	if nbLayers <= 8 {
		return 8
	}
	if nbLayers <= 22 {
		return 10
	}
	return 12
}

func totalBitsInLayer(layers int, compact bool) int {
	base := 112
	if compact {
		base = 88
	}
	return (base + 16*layers) * layers
}

// correctBits applies Reed-Solomon error correction to the raw bit stream
// and unstuffs the data codewords. Matches Java ZXing Decoder.correctBits.
// Returns corrected bits, number of errors corrected, and error.
func correctBits(ddata *AztecDetectorResult, rawbits []bool) ([]bool, int, error) {
	nbLayers := ddata.NbLayers
	nbDataBlocks := ddata.NbDataBlocks

	cwSize := codewordSize(nbLayers)
	numCodewords := len(rawbits) / cwSize

	if nbDataBlocks > numCodewords {
		return nil, 0, zxinggo.ErrFormat
	}

	offset := len(rawbits) % cwSize
	numDataCodewords := nbDataBlocks
	numECCodewords := numCodewords - numDataCodewords

	// Convert raw bits into codeword integers (MSB first, starting after offset).
	dataWords := make([]int, numCodewords)
	for i := 0; i < numCodewords; i++ {
		w := 0
		for j := 0; j < cwSize; j++ {
			w <<= 1
			if rawbits[offset+i*cwSize+j] {
				w |= 1
			}
		}
		dataWords[i] = w
	}

	// Reed-Solomon decode.
	var gf *reedsolomon.GenericGF
	switch cwSize {
	case 6:
		gf = reedsolomon.AztecData6
	case 8:
		gf = reedsolomon.AztecData8
	case 10:
		gf = reedsolomon.AztecData10
	case 12:
		gf = reedsolomon.AztecData12
	default:
		return nil, 0, zxinggo.ErrFormat
	}

	rsDecoder := reedsolomon.NewDecoder(gf)
	errorsCorrected, err := rsDecoder.Decode(dataWords, numECCodewords)
	if err != nil {
		return nil, 0, zxinggo.ErrChecksum
	}

	// Unstuff the corrected data codewords.
	// A codeword of all-zeros or all-ones is illegal (should not appear after stuffing).
	// A codeword of value 1 (0...01) means cwSize-1 zero bits.
	// A codeword of value mask-1 (1...10) means cwSize-1 one bits.
	// All other codewords contribute all cwSize bits unchanged.
	mask := (1 << uint(cwSize)) - 1
	stuffedCount := 0
	for i := 0; i < numDataCodewords; i++ {
		w := dataWords[i]
		if w == 0 || w == mask {
			return nil, 0, zxinggo.ErrFormat
		}
		if w == 1 || w == mask-1 {
			stuffedCount++
		}
	}

	correctedBits := make([]bool, numDataCodewords*cwSize-stuffedCount)
	idx := 0
	for i := 0; i < numDataCodewords; i++ {
		w := dataWords[i]
		if w == 1 || w == mask-1 {
			// Stuffed codeword: output cwSize-1 identical bits.
			fill := w > 1 // true for mask-1 (all ones)
			for j := 0; j < cwSize-1; j++ {
				correctedBits[idx] = fill
				idx++
			}
		} else {
			// Normal codeword: output all cwSize bits.
			for bit := cwSize - 1; bit >= 0; bit-- {
				correctedBits[idx] = (w & (1 << uint(bit))) != 0
				idx++
			}
		}
	}

	return correctedBits, errorsCorrected, nil
}

// ---------------------------------------------------------------------------
// Bit stream decoding (Aztec multi-mode encoding)
// ---------------------------------------------------------------------------

// getTable returns the table constant for the given table initial character.
// Matches Java ZXing Decoder.getTable exactly.
func getTable(t byte) int {
	switch t {
	case 'L':
		return tableLower
	case 'P':
		return tablePunct
	case 'M':
		return tableMixed
	case 'D':
		return tableDigit
	case 'B':
		return tableBinary
	default: // 'U'
		return tableUpper
	}
}

// getCharacter returns the string entry for the given table and code.
// Matches Java ZXing Decoder.getCharacter exactly.
func getCharacter(table, code int) string {
	switch table {
	case tableUpper:
		return upperTable[code]
	case tableLower:
		return lowerTable[code]
	case tableMixed:
		return mixedTable[code]
	case tablePunct:
		return punctTable[code]
	case tableDigit:
		return digitTable[code]
	default:
		return ""
	}
}

// getEncodedData decodes the corrected data-bit stream into text using the
// Aztec five-mode encoding scheme. This is a faithful port of Java ZXing
// Decoder.getEncodedData, including the shiftTable/latchTable architecture,
// byte accumulation buffer, and ISO-8859-1 default encoding.
func getEncodedData(correctedBits []bool) (string, []byte, error) {
	endIndex := len(correctedBits)
	latchTable := tableUpper // table most recently latched to
	shiftTable := tableUpper // table to use for the next read

	var result strings.Builder

	// Intermediary buffer of decoded bytes, decoded into a string and flushed
	// when character encoding changes (ECI) or input ends.
	var decodedBytes []byte
	var encoding string // empty means ISO-8859-1 (default)

	index := 0
	for index < endIndex {
		if shiftTable == tableBinary {
			if endIndex-index < 5 {
				break
			}
			length := readCodeJava(correctedBits, index, 5)
			index += 5
			if length == 0 {
				if endIndex-index < 11 {
					break
				}
				length = readCodeJava(correctedBits, index, 11) + 31
				index += 11
			}
			for charCount := 0; charCount < length; charCount++ {
				if endIndex-index < 8 {
					index = endIndex // Force outer loop to exit
					break
				}
				code := readCodeJava(correctedBits, index, 8)
				decodedBytes = append(decodedBytes, byte(code))
				index += 8
			}
			// Go back to whatever mode we had been in
			shiftTable = latchTable
		} else {
			size := 5
			if shiftTable == tableDigit {
				size = 4
			}
			if endIndex-index < size {
				break
			}
			code := readCodeJava(correctedBits, index, size)
			index += size
			str := getCharacter(shiftTable, code)
			if str == "FLG(n)" {
				if endIndex-index < 3 {
					break
				}
				n := readCodeJava(correctedBits, index, 3)
				index += 3
				// Flush bytes, FLG changes state
				result.WriteString(encodeBytes(decodedBytes, encoding))
				decodedBytes = decodedBytes[:0]
				switch n {
				case 0:
					result.WriteByte(29) // FNC1 as ASCII 29
				case 7:
					return "", nil, zxinggo.ErrFormat // FLG(7) is reserved and illegal
				default:
					// ECI is decimal integer encoded as 1-6 codes in DIGIT mode
					eci := 0
					if endIndex-index < 4*n {
						break
					}
					for n > 0 {
						nextDigit := readCodeJava(correctedBits, index, 4)
						index += 4
						if nextDigit < 2 || nextDigit > 11 {
							return "", nil, zxinggo.ErrFormat // Not a decimal digit
						}
						eci = eci*10 + (nextDigit - 2)
						n--
					}
					eciObj, err := charset.GetECIByValue(eci)
					if err != nil || eciObj == nil {
						return "", nil, zxinggo.ErrFormat
					}
					encoding = eciObj.GoName
				}
				// Go back to whatever mode we had been in
				shiftTable = latchTable
			} else if len(str) > 5 && str[:5] == "CTRL_" {
				// Table changes
				// ISO/IEC 24778:2008 prescribes ending a shift sequence in the
				// mode from which it was invoked. That's including when that mode
				// is a shift.
				latchTable = shiftTable
				shiftTable = getTable(str[5])
				if str[6] == 'L' {
					latchTable = shiftTable
				}
			} else {
				// Though stored as a table of strings for convenience, codes
				// actually represent 1 or 2 *bytes*.
				decodedBytes = append(decodedBytes, str...)
				// Go back to whatever mode we had been in
				shiftTable = latchTable
			}
		}
	}
	result.WriteString(encodeBytes(decodedBytes, encoding))

	text := result.String()
	rawBytes := []byte(text)
	return text, rawBytes, nil
}

// encodeBytes converts a byte buffer to a string using the given encoding.
// If encoding is empty, the default is ISO-8859-1 (each byte maps to its
// Unicode code point, which is then encoded as UTF-8).
func encodeBytes(data []byte, encoding string) string {
	if len(data) == 0 {
		return ""
	}
	if encoding == "" || encoding == "ISO8859_1" || encoding == "ISO-8859-1" {
		// ISO-8859-1: each byte value IS the Unicode code point
		runes := make([]rune, len(data))
		for i, b := range data {
			runes[i] = rune(b)
		}
		return string(runes)
	}
	return charset.DecodeBytes(data, encoding)
}

// readCodeJava reads a code of given length at given index in the bit array.
// Matches Java ZXing Decoder.readCode exactly.
func readCodeJava(rawbits []bool, startIndex, length int) int {
	res := 0
	for i := startIndex; i < startIndex+length; i++ {
		res <<= 1
		if rawbits[i] {
			res |= 1
		}
	}
	return res
}

// readCode reads bitsToRead bits starting at index from the corrected bit
// stream and returns the integer value (MSB first) together with the new index.
// Returns -1 if not enough bits remain.
func readCode(correctedBits []bool, index, bitsToRead, endIndex int) (int, int) {
	if index+bitsToRead > endIndex {
		return -1, endIndex
	}
	code := 0
	for i := index; i < index+bitsToRead; i++ {
		code <<= 1
		if correctedBits[i] {
			code |= 1
		}
	}
	return code, index + bitsToRead
}

// ---------------------------------------------------------------------------
// Bit extraction from the Aztec symbol matrix
// ---------------------------------------------------------------------------

// extractBits reads all data modules from the symbol matrix in the correct
// order. Matches Java ZXing Decoder.extractBits exactly.
//
// Layers are read from outermost (i=0, largest rowSize) to innermost.
// Each layer has 4 sides, each side has rowSize 2-module positions.
func extractBits(ddata *AztecDetectorResult) []bool {
	compact := ddata.Compact
	layers := ddata.NbLayers
	matrix := ddata.Bits

	baseMatrixSize := layers*4 + 11
	if !compact {
		baseMatrixSize = layers*4 + 14
	}

	// Build alignment map (same construction as encoder).
	alignmentMap := make([]int, baseMatrixSize)
	if compact {
		for i := 0; i < baseMatrixSize; i++ {
			alignmentMap[i] = i
		}
	} else {
		matrixSize := baseMatrixSize + 1 + 2*((baseMatrixSize/2-1)/15)
		origCenter := baseMatrixSize / 2
		center := matrixSize / 2
		for i := 0; i < origCenter; i++ {
			newOffset := i + i/15
			alignmentMap[origCenter-i-1] = center - newOffset - 1
			alignmentMap[origCenter+i] = center + newOffset + 1
		}
	}

	totalBits := totalBitsInLayer(layers, compact)
	rawbits := make([]bool, totalBits)

	rowOffset := 0
	for i := 0; i < layers; i++ {
		rowSize := (layers-i)*4 + 9
		if !compact {
			rowSize = (layers-i)*4 + 12
		}
		low := i * 2
		high := baseMatrixSize - 1 - low

		for j := 0; j < rowSize; j++ {
			columnOffset := j * 2
			for k := 0; k < 2; k++ {
				// left column
				rawbits[rowOffset+columnOffset+k] =
					readModule(matrix, alignmentMap, low+k, low+j)
				// bottom row
				rawbits[rowOffset+2*rowSize+columnOffset+k] =
					readModule(matrix, alignmentMap, low+j, high-k)
				// right column
				rawbits[rowOffset+4*rowSize+columnOffset+k] =
					readModule(matrix, alignmentMap, high-k, high-j)
				// top row
				rawbits[rowOffset+6*rowSize+columnOffset+k] =
					readModule(matrix, alignmentMap, high-j, low+k)
			}
		}
		rowOffset += rowSize * 8
	}

	return rawbits
}

// readModule reads a single module from the matrix using the alignment map.
// The x,y args are abstract coordinates; alignmentMap maps them to real coords.
// In BitMatrix, Get(x, y) expects x=column, y=row.
func readModule(matrix *bitutil.BitMatrix, alignmentMap []int, x, y int) bool {
	if x < 0 || x >= len(alignmentMap) || y < 0 || y >= len(alignmentMap) {
		return false
	}
	mx := alignmentMap[x]
	my := alignmentMap[y]
	if mx < 0 || mx >= matrix.Width() || my < 0 || my >= matrix.Height() {
		return false
	}
	return matrix.Get(mx, my)
}
