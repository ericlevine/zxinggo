package oned

import (
	"fmt"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// UPCAWriter encodes UPC-A barcodes by delegating to EAN-13.
type UPCAWriter struct {
	ean13 *EAN13Writer
}

// NewUPCAWriter creates a new UPC-A writer.
func NewUPCAWriter() *UPCAWriter {
	return &UPCAWriter{ean13: NewEAN13Writer()}
}

// Encode encodes the given contents into a UPC-A barcode BitMatrix.
func (w *UPCAWriter) Encode(contents string, format zxinggo.Format, width, height int, opts *zxinggo.EncodeOptions) (*bitutil.BitMatrix, error) {
	if format != zxinggo.FormatUPCA {
		return nil, fmt.Errorf("can only encode UPC_A, but got %s", format)
	}
	// Transform UPC-A to EAN-13 by prepending 0
	return w.ean13.Encode("0"+contents, zxinggo.FormatEAN13, width, height, opts)
}
