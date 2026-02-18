// Package maxicode provides MaxiCode barcode reading.
package maxicode

import (
	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
	"github.com/ericlevine/zxinggo/maxicode/decoder"
)

const (
	matrixWidth  = 30
	matrixHeight = 33
)

// Reader decodes MaxiCode barcodes from binary images.
type Reader struct{}

// NewReader creates a new MaxiCode Reader.
func NewReader() *Reader {
	return &Reader{}
}

// Decode locates and decodes a MaxiCode in the given image.
// MaxiCode always operates in "pure barcode" mode — it extracts the symbol
// directly from the image with no detector.
func (r *Reader) Decode(image *zxinggo.BinaryBitmap, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	matrix, err := image.BlackMatrix()
	if err != nil {
		return nil, err
	}

	bits, err := extractPureBits(matrix)
	if err != nil {
		return nil, err
	}

	dr, err := decoder.Decode(bits)
	if err != nil {
		return nil, err
	}

	result := zxinggo.NewResult(dr.Text, dr.RawBytes, nil, zxinggo.FormatMaxiCode)
	result.PutMetadata(zxinggo.MetadataErrorsCorrected, dr.ErrorsCorrected)
	if dr.ECLevel != "" {
		result.PutMetadata(zxinggo.MetadataErrorCorrectionLevel, dr.ECLevel)
	}
	return result, nil
}

// Reset resets internal state.
func (r *Reader) Reset() {}

// Compile-time check.
var _ zxinggo.Reader = (*Reader)(nil)

// extractPureBits extracts the 30x33 MaxiCode grid from the image.
// MaxiCode uses a hexagonal layout where odd rows are shifted by half a module.
func extractPureBits(image *bitutil.BitMatrix) (*bitutil.BitMatrix, error) {
	enclosingRect := image.EnclosingRectangle()
	if enclosingRect == nil {
		return nil, zxinggo.ErrNotFound
	}

	left := enclosingRect[0]
	top := enclosingRect[1]
	width := enclosingRect[2]
	height := enclosingRect[3]

	bits := bitutil.NewBitMatrixWithSize(matrixWidth, matrixHeight)
	for y := 0; y < matrixHeight; y++ {
		iy := top + min((y*height+height/2)/matrixHeight, height-1)
		for x := 0; x < matrixWidth; x++ {
			// Odd rows are offset by half a module width (hexagonal layout).
			ix := left + min(
				(x*width+width/2+(y&0x01)*width/2)/matrixWidth,
				width-1)
			if image.Get(ix, iy) {
				bits.Set(x, y)
			}
		}
	}
	return bits, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
