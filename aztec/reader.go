// Package aztec provides Aztec barcode reading and writing.
package aztec

import (
	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/aztec/decoder"
	"github.com/ericlevine/zxinggo/aztec/detector"
)

// Reader decodes Aztec barcodes from binary images.
type Reader struct{}

// NewReader creates a new Aztec Reader.
func NewReader() *Reader {
	return &Reader{}
}

// Decode locates and decodes an Aztec barcode in the given image.
func (r *Reader) Decode(image *zxinggo.BinaryBitmap, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	matrix, err := image.BlackMatrix()
	if err != nil {
		return nil, err
	}

	detResult, err := detector.Detect(matrix, false)
	if err != nil {
		return nil, err
	}

	// Convert detector result to decoder input.
	ddata := &decoder.AztecDetectorResult{
		Bits:         detResult.Bits,
		Points:       detResult.Points,
		Compact:      detResult.Compact,
		NbDataBlocks: detResult.NbDataBlocks,
		NbLayers:     detResult.NbLayers,
	}

	dr, err := decoder.Decode(ddata)
	if err != nil {
		return nil, err
	}

	result := zxinggo.NewResult(dr.Text, dr.RawBytes, detResult.Points, zxinggo.FormatAztec)
	result.PutMetadata(zxinggo.MetadataSymbologyIdentifier, "]z0")
	return result, nil
}

// Reset resets internal state.
func (r *Reader) Reset() {}

// Compile-time check.
var _ zxinggo.Reader = (*Reader)(nil)
