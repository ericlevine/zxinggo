package oned

import (
	"fmt"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

const upceCodeWidth = 3 + (7 * 6) + 6 // = 51

// UPCEWriter encodes UPC-E barcodes.
type UPCEWriter struct{}

// NewUPCEWriter creates a new UPC-E writer.
func NewUPCEWriter() *UPCEWriter {
	return &UPCEWriter{}
}

// Encode encodes the given contents into a UPC-E barcode BitMatrix.
func (w *UPCEWriter) Encode(contents string, format zxinggo.Format, width, height int, opts *zxinggo.EncodeOptions) (*bitutil.BitMatrix, error) {
	if format != zxinggo.FormatUPCE {
		return nil, fmt.Errorf("can only encode UPC_E, but got %s", format)
	}
	code, err := w.EncodeContents(contents)
	if err != nil {
		return nil, err
	}
	return RenderOneDCode(code, width, height), nil
}

// EncodeContents encodes UPC-E contents into a boolean pattern.
func (w *UPCEWriter) EncodeContents(contents string) ([]bool, error) {
	length := len(contents)
	switch length {
	case 7:
		check := GetStandardUPCEANChecksum(ConvertUPCEtoUPCA(contents))
		if check < 0 {
			return nil, zxinggo.ErrFormat
		}
		contents += string(rune('0' + check))
	case 8:
		if !CheckStandardUPCEANChecksum(ConvertUPCEtoUPCA(contents)) {
			return nil, fmt.Errorf("contents do not pass checksum")
		}
	default:
		return nil, fmt.Errorf("requested contents should be 7 or 8 digits long, but got %d", length)
	}

	if err := CheckUPCEANDigits(contents); err != nil {
		return nil, err
	}

	firstDigit := int(contents[0] - '0')
	if firstDigit != 0 && firstDigit != 1 {
		return nil, fmt.Errorf("number system must be 0 or 1")
	}

	checkDigit := int(contents[7] - '0')
	parities := upceNumSysAndCheckDigitPatterns[firstDigit][checkDigit]

	result := make([]bool, upceCodeWidth)
	pos := AppendPattern(result, 0, UPCEANStartEndPattern, true)

	for i := 1; i <= 6; i++ {
		digit := int(contents[i] - '0')
		if (parities>>(6-i))&1 == 1 {
			digit += 10
		}
		pos += AppendPattern(result, pos, LAndGPatterns[digit], false)
	}

	AppendPattern(result, pos, UPCEANEndPattern, false)
	return result, nil
}
