package oned

import (
	"strings"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// UPC-E parity patterns indexed by [numSys][checkDigit].
var upceNumSysAndCheckDigitPatterns = [2][10]int{
	{0x38, 0x34, 0x32, 0x31, 0x2C, 0x26, 0x23, 0x2A, 0x29, 0x25},
	{0x07, 0x0B, 0x0D, 0x0E, 0x13, 0x19, 0x1C, 0x15, 0x16, 0x1A},
}

// UPCEReader decodes UPC-E barcodes.
type UPCEReader struct{}

// NewUPCEReader creates a new UPC-E reader.
func NewUPCEReader() *UPCEReader {
	return &UPCEReader{}
}

// BarcodeFormat returns FormatUPCE.
func (r *UPCEReader) BarcodeFormat() zxinggo.Format {
	return zxinggo.FormatUPCE
}

// DecodeRow decodes a UPC-E barcode from a single row.
func (r *UPCEReader) DecodeRow(rowNumber int, row *bitutil.BitArray, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	return DecodeUPCEAN(rowNumber, row, r, opts)
}

// DecodeMiddle decodes the middle portion of a UPC-E barcode.
func (r *UPCEReader) DecodeMiddle(row *bitutil.BitArray, startRange [2]int, result *strings.Builder) (int, error) {
	counters := make([]int, 4)
	end := row.Size()
	rowOffset := startRange[1]

	lgPatternFound := 0

	for x := 0; x < 6 && rowOffset < end; x++ {
		bestMatch, err := DecodeUPCEANDigit(row, counters, rowOffset, LAndGPatterns[:])
		if err != nil {
			return 0, err
		}
		result.WriteByte('0' + byte(bestMatch%10))
		for _, c := range counters {
			rowOffset += c
		}
		if bestMatch >= 10 {
			lgPatternFound |= 1 << uint(5-x)
		}
	}

	if err := determineUPCENumSysAndCheckDigit(result, lgPatternFound); err != nil {
		return 0, err
	}

	return rowOffset, nil
}

func determineUPCENumSysAndCheckDigit(result *strings.Builder, lgPatternFound int) error {
	for numSys := 0; numSys <= 1; numSys++ {
		for d := 0; d < 10; d++ {
			if lgPatternFound == upceNumSysAndCheckDigitPatterns[numSys][d] {
				s := result.String()
				result.Reset()
				result.WriteByte('0' + byte(numSys))
				result.WriteString(s)
				result.WriteByte('0' + byte(d))
				return nil
			}
		}
	}
	return zxinggo.ErrNotFound
}

// ConvertUPCEtoUPCA expands a UPC-E value back into its full UPC-A equivalent.
func ConvertUPCEtoUPCA(upce string) string {
	if len(upce) < 7 {
		return upce
	}
	upceChars := upce[1:7]
	var result strings.Builder
	result.WriteByte(upce[0])

	lastChar := upceChars[5]
	switch lastChar {
	case '0', '1', '2':
		result.WriteString(upceChars[0:2])
		result.WriteByte(lastChar)
		result.WriteString("0000")
		result.WriteString(upceChars[2:5])
	case '3':
		result.WriteString(upceChars[0:3])
		result.WriteString("00000")
		result.WriteString(upceChars[3:5])
	case '4':
		result.WriteString(upceChars[0:4])
		result.WriteString("00000")
		result.WriteByte(upceChars[4])
	default:
		result.WriteString(upceChars[0:5])
		result.WriteString("0000")
		result.WriteByte(lastChar)
	}
	if len(upce) >= 8 {
		result.WriteByte(upce[7])
	}
	return result.String()
}
