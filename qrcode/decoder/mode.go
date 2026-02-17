package decoder

// Mode represents a QR code data encoding mode.
type Mode int

const (
	ModeTerminator        Mode = 0x00
	ModeNumeric           Mode = 0x01
	ModeAlphanumeric      Mode = 0x02
	ModeStructuredAppend  Mode = 0x03
	ModeByte              Mode = 0x04
	ModeFNC1FirstPosition Mode = 0x05
	ModeECI               Mode = 0x07
	ModeKanji             Mode = 0x08
	ModeFNC1SecondPosition Mode = 0x09
	ModeHanzi             Mode = 0x0D
)

// characterCountBitsForVersions contains [v1-9, v10-26, v27-40] bit counts.
var characterCountBits = map[Mode][3]int{
	ModeTerminator:         {0, 0, 0},
	ModeNumeric:            {10, 12, 14},
	ModeAlphanumeric:       {9, 11, 13},
	ModeStructuredAppend:   {0, 0, 0},
	ModeByte:               {8, 16, 16},
	ModeECI:                {0, 0, 0},
	ModeKanji:              {8, 10, 12},
	ModeFNC1FirstPosition:  {0, 0, 0},
	ModeFNC1SecondPosition: {0, 0, 0},
	ModeHanzi:              {8, 10, 12},
}

// ModeForBits returns the Mode for the given 4-bit value.
func ModeForBits(bits int) (Mode, error) {
	switch bits {
	case 0x0:
		return ModeTerminator, nil
	case 0x1:
		return ModeNumeric, nil
	case 0x2:
		return ModeAlphanumeric, nil
	case 0x3:
		return ModeStructuredAppend, nil
	case 0x4:
		return ModeByte, nil
	case 0x5:
		return ModeFNC1FirstPosition, nil
	case 0x7:
		return ModeECI, nil
	case 0x8:
		return ModeKanji, nil
	case 0x9:
		return ModeFNC1SecondPosition, nil
	case 0xD:
		return ModeHanzi, nil
	}
	return 0, errInvalidMode
}

// CharacterCountBits returns the number of bits used to encode the character
// count for this mode in the given version.
func (m Mode) CharacterCountBits(version *Version) int {
	number := version.Number
	var offset int
	if number <= 9 {
		offset = 0
	} else if number <= 26 {
		offset = 1
	} else {
		offset = 2
	}
	return characterCountBits[m][offset]
}

// Bits returns the 4-bit encoding of this mode.
func (m Mode) Bits() int {
	return int(m)
}
