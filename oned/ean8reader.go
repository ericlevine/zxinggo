package oned

import (
	"strings"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// EAN8Reader decodes EAN-8 barcodes.
type EAN8Reader struct{}

// NewEAN8Reader creates a new EAN-8 reader.
func NewEAN8Reader() *EAN8Reader {
	return &EAN8Reader{}
}

// BarcodeFormat returns FormatEAN8.
func (r *EAN8Reader) BarcodeFormat() zxinggo.Format {
	return zxinggo.FormatEAN8
}

// DecodeRow decodes an EAN-8 barcode from a single row.
func (r *EAN8Reader) DecodeRow(rowNumber int, row *bitutil.BitArray, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	return DecodeUPCEAN(rowNumber, row, r, opts)
}

// DecodeMiddle decodes the middle portion of an EAN-8 barcode.
func (r *EAN8Reader) DecodeMiddle(row *bitutil.BitArray, startRange [2]int, result *strings.Builder) (int, error) {
	counters := make([]int, 4)
	end := row.Size()
	rowOffset := startRange[1]

	for x := 0; x < 4 && rowOffset < end; x++ {
		bestMatch, err := DecodeUPCEANDigit(row, counters, rowOffset, LPatterns[:])
		if err != nil {
			return 0, err
		}
		result.WriteByte('0' + byte(bestMatch))
		for _, c := range counters {
			rowOffset += c
		}
	}

	middleRange, err := FindUPCEANMiddleGuardPattern(row, rowOffset)
	if err != nil {
		return 0, err
	}
	rowOffset = middleRange[1]

	for x := 0; x < 4 && rowOffset < end; x++ {
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
