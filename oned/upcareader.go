package oned

import (
	"strings"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// UPCAReader decodes UPC-A barcodes by delegating to EAN-13.
type UPCAReader struct {
	ean13 *EAN13Reader
}

// NewUPCAReader creates a new UPC-A reader.
func NewUPCAReader() *UPCAReader {
	return &UPCAReader{ean13: NewEAN13Reader()}
}

// BarcodeFormat returns FormatUPCA.
func (r *UPCAReader) BarcodeFormat() zxinggo.Format {
	return zxinggo.FormatUPCA
}

// DecodeRow decodes a UPC-A barcode from a single row.
func (r *UPCAReader) DecodeRow(rowNumber int, row *bitutil.BitArray, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	result, err := r.ean13.DecodeRow(rowNumber, row, opts)
	if err != nil {
		return nil, err
	}
	return maybeReturnUPCAResult(result)
}

// DecodeMiddle decodes the middle portion by delegating to EAN-13.
func (r *UPCAReader) DecodeMiddle(row *bitutil.BitArray, startRange [2]int, result *strings.Builder) (int, error) {
	return r.ean13.DecodeMiddle(row, startRange, result)
}

func maybeReturnUPCAResult(result *zxinggo.Result) (*zxinggo.Result, error) {
	text := result.Text
	if len(text) > 0 && text[0] == '0' {
		upcaResult := zxinggo.NewResult(
			text[1:], nil,
			result.Points,
			zxinggo.FormatUPCA,
		)
		for k, v := range result.Metadata {
			upcaResult.PutMetadata(k, v)
		}
		return upcaResult, nil
	}
	return nil, zxinggo.ErrFormat
}
