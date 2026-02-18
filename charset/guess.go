package charset

import (
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// DecodeBytes converts bytes from the given encoding to UTF-8.
// Returns the original bytes if the encoding is already UTF-8/ASCII/ISO-8859-1
// or if conversion fails.
func DecodeBytes(data []byte, encoding string) string {
	switch encoding {
	case "Shift_JIS", "SJIS":
		decoded, _, err := transform.Bytes(japanese.ShiftJIS.NewDecoder(), data)
		if err == nil {
			return string(decoded)
		}
		return string(data)
	case "GB18030", "GB2312", "GBK", "EUC_CN":
		decoded, _, err := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), data)
		if err == nil {
			return string(decoded)
		}
		return string(data)
	default:
		return string(data)
	}
}

// GuessEncoding attempts to guess the encoding of a byte sequence.
// Returns "SJIS", "UTF8", "ISO8859_1", or a fallback.
func GuessEncoding(bytes []byte, characterSet string) string {
	if characterSet != "" {
		return characterSet
	}

	// First try UTF-16 BOM
	if len(bytes) > 2 &&
		((bytes[0] == 0xFE && bytes[1] == 0xFF) ||
			(bytes[0] == 0xFF && bytes[1] == 0xFE)) {
		return "UTF-16"
	}

	length := len(bytes)
	canBeISO88591 := true
	canBeShiftJIS := true
	canBeUTF8 := true
	utf8BytesLeft := 0
	utf2BytesChars := 0
	utf3BytesChars := 0
	utf4BytesChars := 0
	sjisBytesLeft := 0
	sjisKatakanaChars := 0
	sjisCurKatakanaWordLength := 0
	sjisCurDoubleBytesWordLength := 0
	sjisMaxKatakanaWordLength := 0
	sjisMaxDoubleBytesWordLength := 0
	isoHighOther := 0

	utf8bom := len(bytes) > 3 &&
		bytes[0] == 0xEF && bytes[1] == 0xBB && bytes[2] == 0xBF

	for i := 0; i < length && (canBeISO88591 || canBeShiftJIS || canBeUTF8); i++ {
		value := int(bytes[i]) & 0xFF

		// UTF-8 stuff
		if canBeUTF8 {
			if utf8BytesLeft > 0 {
				if (value & 0x80) == 0 {
					canBeUTF8 = false
				} else {
					utf8BytesLeft--
				}
			} else if (value & 0x80) != 0 {
				if (value & 0x40) == 0 {
					canBeUTF8 = false
				} else {
					utf8BytesLeft++
					if (value & 0x20) == 0 {
						utf2BytesChars++
					} else {
						utf8BytesLeft++
						if (value & 0x10) == 0 {
							utf3BytesChars++
						} else {
							utf8BytesLeft++
							if (value & 0x08) == 0 {
								utf4BytesChars++
							} else {
								canBeUTF8 = false
							}
						}
					}
				}
			}
		}

		// ISO-8859-1 stuff
		if canBeISO88591 {
			if value > 0x7F && value < 0xA0 {
				canBeISO88591 = false
			} else if value > 0x9F && (value < 0xC0 || value == 0xD7 || value == 0xF7) {
				isoHighOther++
			}
		}

		// Shift_JIS stuff
		if canBeShiftJIS {
			if sjisBytesLeft > 0 {
				if value < 0x40 || value == 0x7F || value > 0xFC {
					canBeShiftJIS = false
				} else {
					sjisBytesLeft--
				}
			} else if value == 0x80 || value == 0xA0 || value > 0xEF {
				canBeShiftJIS = false
			} else if value > 0xA0 && value < 0xE0 {
				sjisKatakanaChars++
				sjisCurDoubleBytesWordLength = 0
				sjisCurKatakanaWordLength++
				if sjisCurKatakanaWordLength > sjisMaxKatakanaWordLength {
					sjisMaxKatakanaWordLength = sjisCurKatakanaWordLength
				}
			} else if value > 0x7F {
				sjisBytesLeft++
				sjisCurKatakanaWordLength = 0
				sjisCurDoubleBytesWordLength++
				if sjisCurDoubleBytesWordLength > sjisMaxDoubleBytesWordLength {
					sjisMaxDoubleBytesWordLength = sjisCurDoubleBytesWordLength
				}
			} else {
				sjisCurKatakanaWordLength = 0
				sjisCurDoubleBytesWordLength = 0
			}
		}
	}

	if canBeUTF8 && utf8BytesLeft > 0 {
		canBeUTF8 = false
	}
	if canBeShiftJIS && sjisBytesLeft > 0 {
		canBeShiftJIS = false
	}

	if canBeUTF8 && (utf8bom || utf2BytesChars+utf3BytesChars+utf4BytesChars > 0) {
		return "UTF-8"
	}
	if canBeShiftJIS && (sjisMaxKatakanaWordLength >= 3 || sjisMaxDoubleBytesWordLength >= 3) {
		return "Shift_JIS"
	}
	if canBeISO88591 && canBeShiftJIS {
		if (sjisMaxKatakanaWordLength == 2 && sjisKatakanaChars == 2) || isoHighOther*10 >= length {
			return "Shift_JIS"
		}
		return "ISO-8859-1"
	}
	if canBeISO88591 {
		return "ISO-8859-1"
	}
	if canBeShiftJIS {
		return "Shift_JIS"
	}
	if canBeUTF8 {
		return "UTF-8"
	}
	return "UTF-8" // fallback
}
