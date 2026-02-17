package decoder

import "math/bits"

const formatInfoMaskQR = 0x5412

// FormatInformation encapsulates a QR code's format info (EC level + data mask).
type FormatInformation struct {
	ECLevel  ErrorCorrectionLevel
	DataMask byte
}

var formatInfoDecodeLookup = [][2]int{
	{0x5412, 0x00}, {0x5125, 0x01}, {0x5E7C, 0x02}, {0x5B4B, 0x03},
	{0x45F9, 0x04}, {0x40CE, 0x05}, {0x4F97, 0x06}, {0x4AA0, 0x07},
	{0x77C4, 0x08}, {0x72F3, 0x09}, {0x7DAA, 0x0A}, {0x789D, 0x0B},
	{0x662F, 0x0C}, {0x6318, 0x0D}, {0x6C41, 0x0E}, {0x6976, 0x0F},
	{0x1689, 0x10}, {0x13BE, 0x11}, {0x1CE7, 0x12}, {0x19D0, 0x13},
	{0x0762, 0x14}, {0x0255, 0x15}, {0x0D0C, 0x16}, {0x083B, 0x17},
	{0x355F, 0x18}, {0x3068, 0x19}, {0x3F31, 0x1A}, {0x3A06, 0x1B},
	{0x24B4, 0x1C}, {0x2183, 0x1D}, {0x2EDA, 0x1E}, {0x2BED, 0x1F},
}

func newFormatInformation(formatInfo int) *FormatInformation {
	ecLevel, _ := ECLevelForBits((formatInfo >> 3) & 0x03)
	return &FormatInformation{
		ECLevel:  ecLevel,
		DataMask: byte(formatInfo & 0x07),
	}
}

// DecodeFormatInformation decodes format information from two masked values.
func DecodeFormatInformation(maskedFormatInfo1, maskedFormatInfo2 int) *FormatInformation {
	fi := doDecodeFormatInformation(maskedFormatInfo1, maskedFormatInfo2)
	if fi != nil {
		return fi
	}
	return doDecodeFormatInformation(maskedFormatInfo1^formatInfoMaskQR, maskedFormatInfo2^formatInfoMaskQR)
}

func doDecodeFormatInformation(maskedFormatInfo1, maskedFormatInfo2 int) *FormatInformation {
	bestDifference := 32
	bestFormatInfo := 0
	for _, entry := range formatInfoDecodeLookup {
		target := entry[0]
		if target == maskedFormatInfo1 || target == maskedFormatInfo2 {
			return newFormatInformation(entry[1])
		}
		bitsDiff := bits.OnesCount(uint(maskedFormatInfo1 ^ target))
		if bitsDiff < bestDifference {
			bestFormatInfo = entry[1]
			bestDifference = bitsDiff
		}
		if maskedFormatInfo1 != maskedFormatInfo2 {
			bitsDiff = bits.OnesCount(uint(maskedFormatInfo2 ^ target))
			if bitsDiff < bestDifference {
				bestFormatInfo = entry[1]
				bestDifference = bitsDiff
			}
		}
	}
	if bestDifference <= 3 {
		return newFormatInformation(bestFormatInfo)
	}
	return nil
}
