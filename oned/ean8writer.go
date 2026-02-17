package oned

import (
	"fmt"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

const ean8CodeWidth = 3 + (7 * 4) + 5 + (7 * 4) + 3 // = 67

// EAN8Writer encodes EAN-8 barcodes.
type EAN8Writer struct{}

// NewEAN8Writer creates a new EAN-8 writer.
func NewEAN8Writer() *EAN8Writer {
	return &EAN8Writer{}
}

// Encode encodes the given contents into an EAN-8 barcode BitMatrix.
func (w *EAN8Writer) Encode(contents string, format zxinggo.Format, width, height int, opts *zxinggo.EncodeOptions) (*bitutil.BitMatrix, error) {
	if format != zxinggo.FormatEAN8 {
		return nil, fmt.Errorf("can only encode EAN_8, but got %s", format)
	}
	code, err := w.EncodeContents(contents)
	if err != nil {
		return nil, err
	}
	return RenderOneDCode(code, width, height), nil
}

// EncodeContents encodes EAN-8 contents into a boolean pattern.
func (w *EAN8Writer) EncodeContents(contents string) ([]bool, error) {
	var err error
	contents, err = CheckUPCEANLength(contents, 7, 8)
	if err != nil {
		return nil, err
	}

	result := make([]bool, ean8CodeWidth)
	pos := 0

	pos += AppendPattern(result, pos, UPCEANStartEndPattern, true)

	for i := 0; i <= 3; i++ {
		digit := int(contents[i] - '0')
		pos += AppendPattern(result, pos, LPatterns[digit], false)
	}

	pos += AppendPattern(result, pos, UPCEANMiddlePattern, false)

	for i := 4; i <= 7; i++ {
		digit := int(contents[i] - '0')
		pos += AppendPattern(result, pos, LPatterns[digit], true)
	}

	AppendPattern(result, pos, UPCEANStartEndPattern, true)
	return result, nil
}
