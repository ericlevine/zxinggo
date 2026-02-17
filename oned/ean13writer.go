package oned

import (
	"fmt"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

const ean13CodeWidth = 3 + (7 * 6) + 5 + (7 * 6) + 3 // = 95

// EAN13Writer encodes EAN-13 barcodes.
type EAN13Writer struct{}

// NewEAN13Writer creates a new EAN-13 writer.
func NewEAN13Writer() *EAN13Writer {
	return &EAN13Writer{}
}

// Encode encodes the given contents into an EAN-13 barcode BitMatrix.
func (w *EAN13Writer) Encode(contents string, format zxinggo.Format, width, height int, opts *zxinggo.EncodeOptions) (*bitutil.BitMatrix, error) {
	if format != zxinggo.FormatEAN13 {
		return nil, fmt.Errorf("can only encode EAN_13, but got %s", format)
	}
	code, err := w.EncodeContents(contents)
	if err != nil {
		return nil, err
	}
	return RenderOneDCode(code, width, height), nil
}

// EncodeContents encodes EAN-13 contents into a boolean pattern.
func (w *EAN13Writer) EncodeContents(contents string) ([]bool, error) {
	var err error
	contents, err = CheckUPCEANLength(contents, 12, 13)
	if err != nil {
		return nil, err
	}

	firstDigit := int(contents[0] - '0')
	parities := ean13FirstDigitEncodings[firstDigit]
	result := make([]bool, ean13CodeWidth)
	pos := 0

	pos += AppendPattern(result, pos, UPCEANStartEndPattern, true)

	for i := 1; i <= 6; i++ {
		digit := int(contents[i] - '0')
		if (parities>>(6-i))&1 == 1 {
			digit += 10
		}
		pos += AppendPattern(result, pos, LAndGPatterns[digit], false)
	}

	pos += AppendPattern(result, pos, UPCEANMiddlePattern, false)

	for i := 7; i <= 12; i++ {
		digit := int(contents[i] - '0')
		pos += AppendPattern(result, pos, LPatterns[digit], true)
	}

	AppendPattern(result, pos, UPCEANStartEndPattern, true)
	return result, nil
}
