// Package decoder implements QR code decoding.
package decoder

// ErrorCorrectionLevel represents the four QR code error correction levels.
type ErrorCorrectionLevel int

const (
	ECLevelL ErrorCorrectionLevel = iota // ~7% correction
	ECLevelM                             // ~15% correction
	ECLevelQ                             // ~25% correction
	ECLevelH                             // ~30% correction
)

// Bits returns the 2-bit encoding of this level.
func (ecl ErrorCorrectionLevel) Bits() int {
	switch ecl {
	case ECLevelL:
		return 0x01
	case ECLevelM:
		return 0x00
	case ECLevelQ:
		return 0x03
	case ECLevelH:
		return 0x02
	}
	return 0
}

// Ordinal returns the ordinal position (L=0, M=1, Q=2, H=3).
func (ecl ErrorCorrectionLevel) Ordinal() int {
	return int(ecl)
}

// String returns the level name.
func (ecl ErrorCorrectionLevel) String() string {
	switch ecl {
	case ECLevelL:
		return "L"
	case ECLevelM:
		return "M"
	case ECLevelQ:
		return "Q"
	case ECLevelH:
		return "H"
	}
	return "?"
}

// ECLevelForBits returns the ErrorCorrectionLevel for the given 2-bit value.
func ECLevelForBits(bits int) (ErrorCorrectionLevel, error) {
	// FOR_BITS = {M, L, H, Q}
	switch bits {
	case 0:
		return ECLevelM, nil
	case 1:
		return ECLevelL, nil
	case 2:
		return ECLevelH, nil
	case 3:
		return ECLevelQ, nil
	}
	return 0, errInvalidECLevel
}
