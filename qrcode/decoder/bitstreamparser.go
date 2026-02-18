package decoder

import (
	"fmt"
	"strings"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
	"github.com/ericlevine/zxinggo/charset"
	"github.com/ericlevine/zxinggo/internal"
)

const alphanumericChars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ $%*+-./:";

const gb2312Subset = 1

// DecodeBitStream decodes data bytes into a DecoderResult.
func DecodeBitStream(bytes []byte, version *Version, ecLevel ErrorCorrectionLevel, characterSet string) (*internal.DecoderResult, error) {
	bs := bitutil.NewBitSource(bytes)
	var result strings.Builder
	result.Grow(50)
	var byteSegments [][]byte
	symbolSequence := -1
	parityData := -1
	var symbologyModifier int

	var currentCharacterSetECI *charset.ECI
	fc1InEffect := false
	hasFNC1first := false
	hasFNC1second := false

	for {
		var mode Mode
		if bs.Available() < 4 {
			mode = ModeTerminator
		} else {
			modeBits, err := bs.ReadBits(4)
			if err != nil {
				return nil, zxinggo.ErrFormat
			}
			mode, err = ModeForBits(modeBits)
			if err != nil {
				return nil, zxinggo.ErrFormat
			}
		}

		switch mode {
		case ModeTerminator:
			// done
		case ModeFNC1FirstPosition:
			hasFNC1first = true
			fc1InEffect = true
		case ModeFNC1SecondPosition:
			hasFNC1second = true
			fc1InEffect = true
		case ModeStructuredAppend:
			if bs.Available() < 16 {
				return nil, zxinggo.ErrFormat
			}
			seq, _ := bs.ReadBits(8)
			par, _ := bs.ReadBits(8)
			symbolSequence = seq
			parityData = par
		case ModeECI:
			value, err := parseECIValue(bs)
			if err != nil {
				return nil, err
			}
			eci, eciErr := charset.GetECIByValue(value)
			if eciErr != nil {
				return nil, zxinggo.ErrFormat
			}
			currentCharacterSetECI = eci
		case ModeHanzi:
			subsetBits, _ := bs.ReadBits(4)
			countBits := mode.CharacterCountBits(version)
			count, _ := bs.ReadBits(countBits)
			if subsetBits == gb2312Subset {
				if err := decodeHanziSegment(bs, &result, count); err != nil {
					return nil, err
				}
			}
		default:
			countBits := mode.CharacterCountBits(version)
			count, err := bs.ReadBits(countBits)
			if err != nil {
				return nil, zxinggo.ErrFormat
			}
			switch mode {
			case ModeNumeric:
				if err := decodeNumericSegment(bs, &result, count); err != nil {
					return nil, err
				}
			case ModeAlphanumeric:
				if err := decodeAlphanumericSegment(bs, &result, count, fc1InEffect); err != nil {
					return nil, err
				}
			case ModeByte:
				seg, err := decodeByteSegment(bs, &result, count, currentCharacterSetECI, characterSet)
				if err != nil {
					return nil, err
				}
				byteSegments = append(byteSegments, seg)
			case ModeKanji:
				if err := decodeKanjiSegment(bs, &result, count); err != nil {
					return nil, err
				}
			default:
				return nil, zxinggo.ErrFormat
			}
		}

		if mode == ModeTerminator {
			break
		}
	}

	if currentCharacterSetECI != nil {
		if hasFNC1first {
			symbologyModifier = 4
		} else if hasFNC1second {
			symbologyModifier = 6
		} else {
			symbologyModifier = 2
		}
	} else {
		if hasFNC1first {
			symbologyModifier = 3
		} else if hasFNC1second {
			symbologyModifier = 5
		} else {
			symbologyModifier = 1
		}
	}

	ecLevelStr := ecLevel.String()
	return internal.NewDecoderResultFull(bytes, result.String(), byteSegments, ecLevelStr,
		symbolSequence, parityData, symbologyModifier), nil
}

func decodeHanziSegment(bs *bitutil.BitSource, result *strings.Builder, count int) error {
	if count*13 > bs.Available() {
		return zxinggo.ErrFormat
	}
	buf := make([]byte, 2*count)
	offset := 0
	for count > 0 {
		twoBytes, _ := bs.ReadBits(13)
		assembled := ((twoBytes / 0x060) << 8) | (twoBytes % 0x060)
		if assembled < 0x00A00 {
			assembled += 0x0A1A1
		} else {
			assembled += 0x0A6A1
		}
		buf[offset] = byte((assembled >> 8) & 0xFF)
		buf[offset+1] = byte(assembled & 0xFF)
		offset += 2
		count--
	}
	result.WriteString(charset.DecodeBytes(buf[:offset], "GB18030"))
	return nil
}

func decodeKanjiSegment(bs *bitutil.BitSource, result *strings.Builder, count int) error {
	if count*13 > bs.Available() {
		return zxinggo.ErrFormat
	}
	buf := make([]byte, 2*count)
	offset := 0
	for count > 0 {
		twoBytes, _ := bs.ReadBits(13)
		assembled := ((twoBytes / 0x0C0) << 8) | (twoBytes % 0x0C0)
		if assembled < 0x01F00 {
			assembled += 0x08140
		} else {
			assembled += 0x0C140
		}
		buf[offset] = byte(assembled >> 8)
		buf[offset+1] = byte(assembled)
		offset += 2
		count--
	}
	result.WriteString(charset.DecodeBytes(buf[:offset], "Shift_JIS"))
	return nil
}

func decodeByteSegment(bs *bitutil.BitSource, result *strings.Builder, count int,
	currentECI *charset.ECI, characterSet string) ([]byte, error) {
	if 8*count > bs.Available() {
		return nil, zxinggo.ErrFormat
	}
	readBytes := make([]byte, count)
	for i := 0; i < count; i++ {
		val, _ := bs.ReadBits(8)
		readBytes[i] = byte(val)
	}

	var encoding string
	if currentECI != nil {
		encoding = currentECI.GoName
	} else {
		encoding = charset.GuessEncoding(readBytes, characterSet)
	}
	result.WriteString(charset.DecodeBytes(readBytes, encoding))
	return readBytes, nil
}

func toAlphaNumericChar(value int) (byte, error) {
	if value >= len(alphanumericChars) {
		return 0, zxinggo.ErrFormat
	}
	return alphanumericChars[value], nil
}

func decodeAlphanumericSegment(bs *bitutil.BitSource, result *strings.Builder, count int, fc1InEffect bool) error {
	start := result.Len()
	for count > 1 {
		if bs.Available() < 11 {
			return zxinggo.ErrFormat
		}
		nextTwo, _ := bs.ReadBits(11)
		c1, err := toAlphaNumericChar(nextTwo / 45)
		if err != nil {
			return err
		}
		c2, err := toAlphaNumericChar(nextTwo % 45)
		if err != nil {
			return err
		}
		result.WriteByte(c1)
		result.WriteByte(c2)
		count -= 2
	}
	if count == 1 {
		if bs.Available() < 6 {
			return zxinggo.ErrFormat
		}
		val, _ := bs.ReadBits(6)
		c, err := toAlphaNumericChar(val)
		if err != nil {
			return err
		}
		result.WriteByte(c)
	}
	if fc1InEffect {
		s := result.String()
		// Process FNC1 from start position
		var modified strings.Builder
		modified.WriteString(s[:start])
		for i := start; i < len(s); i++ {
			if s[i] == '%' {
				if i < len(s)-1 && s[i+1] == '%' {
					modified.WriteByte('%')
					i++ // skip next %
				} else {
					modified.WriteByte(0x1D)
				}
			} else {
				modified.WriteByte(s[i])
			}
		}
		result.Reset()
		result.WriteString(modified.String())
	}
	return nil
}

func decodeNumericSegment(bs *bitutil.BitSource, result *strings.Builder, count int) error {
	for count >= 3 {
		if bs.Available() < 10 {
			return zxinggo.ErrFormat
		}
		threeDigits, _ := bs.ReadBits(10)
		if threeDigits >= 1000 {
			return zxinggo.ErrFormat
		}
		result.WriteString(fmt.Sprintf("%03d", threeDigits))
		count -= 3
	}
	if count == 2 {
		if bs.Available() < 7 {
			return zxinggo.ErrFormat
		}
		twoDigits, _ := bs.ReadBits(7)
		if twoDigits >= 100 {
			return zxinggo.ErrFormat
		}
		result.WriteString(fmt.Sprintf("%02d", twoDigits))
	} else if count == 1 {
		if bs.Available() < 4 {
			return zxinggo.ErrFormat
		}
		digit, _ := bs.ReadBits(4)
		if digit >= 10 {
			return zxinggo.ErrFormat
		}
		result.WriteString(fmt.Sprintf("%d", digit))
	}
	return nil
}

func parseECIValue(bs *bitutil.BitSource) (int, error) {
	firstByte, err := bs.ReadBits(8)
	if err != nil {
		return 0, zxinggo.ErrFormat
	}
	if (firstByte & 0x80) == 0 {
		return firstByte & 0x7F, nil
	}
	if (firstByte & 0xC0) == 0x80 {
		secondByte, _ := bs.ReadBits(8)
		return ((firstByte & 0x3F) << 8) | secondByte, nil
	}
	if (firstByte & 0xE0) == 0xC0 {
		secondThirdBytes, _ := bs.ReadBits(16)
		return ((firstByte & 0x1F) << 16) | secondThirdBytes, nil
	}
	return 0, zxinggo.ErrFormat
}
