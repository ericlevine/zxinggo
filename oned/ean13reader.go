package oned

import (
	"strings"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// EAN-13 first digit encodings: the first digit is encoded by the parity pattern
// used for the next 6 digits. Odd=0, Even=1.
var ean13FirstDigitEncodings = [10]int{
	0x00, 0x0B, 0x0D, 0x0E, 0x13, 0x19, 0x1C, 0x15, 0x16, 0x1A,
}

// EAN13Reader decodes EAN-13 barcodes.
type EAN13Reader struct{}

// NewEAN13Reader creates a new EAN-13 reader.
func NewEAN13Reader() *EAN13Reader {
	return &EAN13Reader{}
}

// BarcodeFormat returns FormatEAN13.
func (r *EAN13Reader) BarcodeFormat() zxinggo.Format {
	return zxinggo.FormatEAN13
}

// DecodeRow decodes an EAN-13 barcode from a single row.
func (r *EAN13Reader) DecodeRow(rowNumber int, row *bitutil.BitArray, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	return DecodeUPCEAN(rowNumber, row, r, opts)
}

// DecodeMiddle decodes the middle portion of an EAN-13 barcode.
func (r *EAN13Reader) DecodeMiddle(row *bitutil.BitArray, startRange [2]int, result *strings.Builder) (int, error) {
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

	if err := determineEAN13FirstDigit(result, lgPatternFound); err != nil {
		return 0, err
	}

	middleRange, err := FindUPCEANMiddleGuardPattern(row, rowOffset)
	if err != nil {
		return 0, err
	}
	rowOffset = middleRange[1]

	for x := 0; x < 6 && rowOffset < end; x++ {
		bestMatch, err := DecodeUPCEANDigit(row, counters, rowOffset, LPatterns[:])
		if err != nil {
			return 0, err
		}
		result.WriteByte('0' + byte(bestMatch))
		for _, c := range counters {
			rowOffset += c
		}
	}

	return rowOffset, nil
}

func determineEAN13FirstDigit(result *strings.Builder, lgPatternFound int) error {
	for d := 0; d < 10; d++ {
		if lgPatternFound == ean13FirstDigitEncodings[d] {
			s := result.String()
			result.Reset()
			result.WriteByte('0' + byte(d))
			result.WriteString(s)
			return nil
		}
	}
	return zxinggo.ErrNotFound
}
