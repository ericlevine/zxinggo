package decoder

import (
	"strings"

	zxinggo "github.com/ericlevine/zxinggo"
)

// DecoderResult holds the decoded text and raw bytes from a Data Matrix barcode.
type DecoderResult struct {
	Text     string
	RawBytes []byte
}

// Data Matrix encoding modes
const (
	modeASCII   = iota // default start mode
	modeC40            // C40 encoding
	modeText           // Text encoding
	modeX12            // ANSI X12 encoding
	modeEDIFACT        // EDIFACT encoding
	modeBase256        // Base 256 encoding
	modePad            // padding reached — stop
)

// C40 and Text shift 2 lookup table. Index 0-26 map to printable characters,
// 27 = FNC1, 28-29 reserved, 30 = Upper Shift.
var c40TextShift2 = [32]byte{
	'!', '"', '#', '$', '%', '&', '\'', '(', ')', '*', '+', ',', '-', '.', '/',
	':', ';', '<', '=', '>', '?', '@', '[', '\\', ']', '^', '_',
	0x1D, // 27: FNC1 (GS)
	0,    // 28: reserved (Structured Append)
	0,    // 29: reserved (Upper Shift latch — handled separately for C40/Text)
	0,    // 30: Upper Shift — handled in code
	0,    // 31: padding placeholder
}

// DecodeBitStream decodes the data codewords of a Data Matrix symbol into text.
func DecodeBitStream(bytes []byte) (*DecoderResult, error) {
	var result strings.Builder
	mode := modeASCII
	pos := 0

	for pos < len(bytes) {
		switch mode {
		case modeASCII:
			newMode, err := decodeASCII(&result, bytes, &pos)
			if err != nil {
				return nil, err
			}
			mode = newMode
		case modeC40:
			newMode, err := decodeC40Text(&result, bytes, &pos, false)
			if err != nil {
				return nil, err
			}
			mode = newMode
		case modeText:
			newMode, err := decodeC40Text(&result, bytes, &pos, true)
			if err != nil {
				return nil, err
			}
			mode = newMode
		case modeX12:
			newMode, err := decodeAnsiX12(&result, bytes, &pos)
			if err != nil {
				return nil, err
			}
			mode = newMode
		case modeEDIFACT:
			newMode, err := decodeEdifact(&result, bytes, &pos)
			if err != nil {
				return nil, err
			}
			mode = newMode
		case modeBase256:
			newMode, err := decodeBase256(&result, bytes, &pos)
			if err != nil {
				return nil, err
			}
			mode = newMode
		}
		if mode == modePad {
			break
		}
	}

	return &DecoderResult{
		Text:     result.String(),
		RawBytes: bytes,
	}, nil
}

// decodeASCII processes codewords in ASCII mode. It processes all codewords
// until a mode latch is hit or the data runs out.
func decodeASCII(result *strings.Builder, bytes []byte, pos *int) (int, error) {
	for *pos < len(bytes) {
		b := int(bytes[*pos]) & 0xFF
		*pos++

		switch {
		case b == 0:
			return 0, zxinggo.ErrFormat
		case b <= 128:
			// ASCII data: encoded value is char + 1
			result.WriteByte(byte(b - 1))
		case b == 129:
			// PAD codeword — done
			return modePad, nil
		case b <= 229:
			// Two-digit numeric pair: value 130 encodes "00", 229 encodes "99"
			pair := b - 130
			result.WriteByte(byte('0' + pair/10))
			result.WriteByte(byte('0' + pair%10))
		case b == 230:
			return modeC40, nil
		case b == 231:
			return modeBase256, nil
		case b == 232:
			// FNC1
			result.WriteByte(0x1D)
		case b == 233:
			// Structured Append — read and ignore 2 identifier bytes
			*pos += 2
		case b == 234:
			// Reader Programming — ignore
		case b == 235:
			// Upper Shift: next codeword value + 128
			if *pos >= len(bytes) {
				return 0, zxinggo.ErrFormat
			}
			next := int(bytes[*pos]) & 0xFF
			*pos++
			result.WriteByte(byte(next - 1 + 128))
		case b == 236:
			// 05 Macro header
			result.WriteString("[)>\x1E05\x1D")
		case b == 237:
			// 06 Macro header
			result.WriteString("[)>\x1E06\x1D")
		case b == 238:
			return modeX12, nil
		case b == 239:
			return modeText, nil
		case b == 240:
			return modeEDIFACT, nil
		case b == 241:
			// ECI — not fully supported; skip
		default:
			// 242-255: not used, treated as pad
		}
	}
	return modeASCII, nil
}

// decodeC40Text decodes C40 or Text mode encoded data.
// In C40 mode the basic set encodes: space, 0-9, A-Z.
// In Text mode the basic set encodes: space, 0-9, a-z.
func decodeC40Text(result *strings.Builder, bytes []byte, pos *int, textMode bool) (int, error) {
	shift := 0
	upperShift := false

	for *pos < len(bytes)-1 {
		c1 := int(bytes[*pos]) & 0xFF
		*pos++

		if c1 == 254 {
			// Unlatch to ASCII
			return modeASCII, nil
		}

		c2 := int(bytes[*pos]) & 0xFF
		*pos++

		// Two codewords encode three C40/Text values
		v := c1*256 + c2 - 1
		u := [3]int{
			v / 1600,
			(v / 40) % 40,
			v % 40,
		}

		for i := 0; i < 3; i++ {
			cVal := u[i]

			switch shift {
			case 0: // Basic set
				if cVal < 3 {
					// Shift to set 1, 2, or 3
					shift = cVal + 1
					continue
				}
				if cVal == 3 {
					appendWithShift(result, ' ', upperShift)
					upperShift = false
					continue
				}
				if cVal <= 13 {
					appendWithShift(result, byte('0'+cVal-4), upperShift)
					upperShift = false
					continue
				}
				if textMode {
					appendWithShift(result, byte('a'+cVal-14), upperShift)
				} else {
					appendWithShift(result, byte('A'+cVal-14), upperShift)
				}
				upperShift = false

			case 1: // Shift 1 set: ASCII 0-31
				appendWithShift(result, byte(cVal), upperShift)
				upperShift = false
				shift = 0

			case 2: // Shift 2 set
				if cVal < 27 {
					appendWithShift(result, c40TextShift2[cVal], upperShift)
					upperShift = false
				} else if cVal == 27 {
					// FNC1
					appendWithShift(result, 0x1D, upperShift)
					upperShift = false
				} else if cVal == 30 {
					// Upper Shift — next character gets +128
					upperShift = true
				}
				// 28, 29, 31 are reserved/ignored
				shift = 0

			case 3: // Shift 3 set
				if textMode {
					// Text mode shift 3: ` A-Z { | } ~ DEL
					if cVal == 0 {
						appendWithShift(result, '`', upperShift)
					} else if cVal <= 26 {
						appendWithShift(result, byte('A'+cVal-1), upperShift)
					} else {
						switch cVal {
						case 27:
							appendWithShift(result, '{', upperShift)
						case 28:
							appendWithShift(result, '|', upperShift)
						case 29:
							appendWithShift(result, '}', upperShift)
						case 30:
							appendWithShift(result, '~', upperShift)
						case 31:
							appendWithShift(result, 127, upperShift)
						}
					}
				} else {
					// C40 mode shift 3: ` a-z { | } ~ DEL
					if cVal == 0 {
						appendWithShift(result, '`', upperShift)
					} else if cVal <= 26 {
						appendWithShift(result, byte('a'+cVal-1), upperShift)
					} else {
						switch cVal {
						case 27:
							appendWithShift(result, '{', upperShift)
						case 28:
							appendWithShift(result, '|', upperShift)
						case 29:
							appendWithShift(result, '}', upperShift)
						case 30:
							appendWithShift(result, '~', upperShift)
						case 31:
							appendWithShift(result, 127, upperShift)
						}
					}
				}
				upperShift = false
				shift = 0
			}
		}
	}

	// If we fall through (remaining single byte), it's treated as an ASCII codeword
	// after an implicit unlatch.
	return modeASCII, nil
}

func appendWithShift(result *strings.Builder, ch byte, upperShift bool) {
	if upperShift {
		result.WriteByte(ch + 128)
	} else {
		result.WriteByte(ch)
	}
}

// decodeAnsiX12 decodes ANSI X12 encoded data.
// X12 basic set: CR, *, >, space, 0-9, A-Z
func decodeAnsiX12(result *strings.Builder, bytes []byte, pos *int) (int, error) {
	for *pos < len(bytes)-1 {
		c1 := int(bytes[*pos]) & 0xFF
		*pos++

		if c1 == 254 {
			return modeASCII, nil
		}

		c2 := int(bytes[*pos]) & 0xFF
		*pos++

		v := c1*256 + c2 - 1
		u := [3]int{
			v / 1600,
			(v / 40) % 40,
			v % 40,
		}

		for i := 0; i < 3; i++ {
			cVal := u[i]
			switch {
			case cVal == 0:
				result.WriteByte('\r')
			case cVal == 1:
				result.WriteByte('*')
			case cVal == 2:
				result.WriteByte('>')
			case cVal == 3:
				result.WriteByte(' ')
			case cVal >= 4 && cVal <= 13:
				result.WriteByte(byte('0' + cVal - 4))
			case cVal >= 14 && cVal <= 39:
				result.WriteByte(byte('A' + cVal - 14))
			}
		}
	}
	return modeASCII, nil
}

// decodeEdifact decodes EDIFACT encoded data.
// EDIFACT packs four 6-bit values into three codewords (24 bits).
func decodeEdifact(result *strings.Builder, bytes []byte, pos *int) (int, error) {
	for *pos < len(bytes) {
		// We need at least 3 bytes to decode a full EDIFACT triplet (4 values).
		// However, partial decoding at end is also possible.

		// Read up to 3 codewords and unpack 4 x 6-bit EDIFACT values.
		// But the ZXing Java approach reads one byte at a time and extracts 6-bit
		// values. Let me follow the Java approach:
		//
		// Java loops reading single bytes. For each byte, it takes the bottom 6 bits
		// as an EDIFACT value. If the value is 31 (0x1F), it's an unlatch.
		// Four bytes produce 4 values = one "word". But the 4 values are just the
		// bottom 6 bits of 4 consecutive bytes.

		// Actually, looking at the ZXing Java code more carefully:
		// The EDIFACT encoder packs 4 x 6-bit values into 3 bytes.
		// The decoder unpacks 3 bytes into 4 x 6-bit values.

		if *pos+2 > len(bytes) {
			break
		}

		b1 := int(bytes[*pos]) & 0xFF
		*pos++
		b2 := int(bytes[*pos]) & 0xFF
		*pos++
		b3 := int(bytes[*pos]) & 0xFF
		*pos++

		// 3 bytes = 24 bits = 4 x 6-bit values
		val1 := (b1 >> 2) & 0x3F
		val2 := ((b1 & 0x03) << 4) | ((b2 >> 4) & 0x0F)
		val3 := ((b2 & 0x0F) << 2) | ((b3 >> 6) & 0x03)
		val4 := b3 & 0x3F

		vals := [4]int{val1, val2, val3, val4}
		for _, ev := range vals {
			if ev == 31 {
				// Unlatch to ASCII
				return modeASCII, nil
			}
			// EDIFACT values 32-94 map directly to ASCII 32-94
			// Values 0-31 and 95-63 map to ASCII 64-95 and 96-127
			ch := ev
			if (ch & 0x20) == 0 {
				ch |= 0x40
			}
			result.WriteByte(byte(ch))
		}
	}
	return modeASCII, nil
}

// decodeBase256 decodes Base 256 encoded data.
func decodeBase256(result *strings.Builder, bytes []byte, pos *int) (int, error) {
	if *pos >= len(bytes) {
		return 0, zxinggo.ErrFormat
	}

	// First byte is the length field (pseudo-randomized)
	d1 := unRandomize255State(int(bytes[*pos])&0xFF, *pos+1)
	*pos++

	var count int
	if d1 == 0 {
		// Length 0 means the count equals the remaining symbols
		count = len(bytes) - *pos
	} else if d1 < 250 {
		count = d1
	} else {
		// Two-byte length field
		if *pos >= len(bytes) {
			return 0, zxinggo.ErrFormat
		}
		d2 := unRandomize255State(int(bytes[*pos])&0xFF, *pos+1)
		*pos++
		count = 250*(d1-249) + d2
	}

	if count < 0 || *pos+count > len(bytes) {
		return 0, zxinggo.ErrFormat
	}

	for i := 0; i < count; i++ {
		if *pos >= len(bytes) {
			return 0, zxinggo.ErrFormat
		}
		ch := unRandomize255State(int(bytes[*pos])&0xFF, *pos+1)
		*pos++
		result.WriteByte(byte(ch))
	}

	return modeASCII, nil
}

// unRandomize255State removes the 255-state pseudo-random masking used in
// Base 256 mode. codewordPosition is the 1-based position of the codeword
// in the data stream (including the length field).
func unRandomize255State(randomizedBase256Codeword, codewordPosition int) int {
	pseudoRandomNumber := ((149 * codewordPosition) % 255) + 1
	tempVariable := randomizedBase256Codeword - pseudoRandomNumber
	if tempVariable >= 0 {
		return tempVariable
	}
	return tempVariable + 256
}
