// Package datamatrix provides Data Matrix (ECC-200) reading and writing.
package datamatrix

import (
	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
	"github.com/ericlevine/zxinggo/datamatrix/decoder"
	"github.com/ericlevine/zxinggo/datamatrix/detector"
)

// Reader decodes Data Matrix barcodes from binary images.
type Reader struct {
	dec *decoder.Decoder
}

// NewReader creates a new Data Matrix Reader.
func NewReader() *Reader {
	return &Reader{
		dec: decoder.NewDecoder(),
	}
}

// Decode locates and decodes a Data Matrix barcode in the given image.
func (r *Reader) Decode(image *zxinggo.BinaryBitmap, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	if opts == nil {
		opts = &zxinggo.DecodeOptions{}
	}

	matrix, err := image.BlackMatrix()
	if err != nil {
		return nil, err
	}

	if opts.PureBarcode {
		bits, err := extractPureBits(matrix)
		if err != nil {
			return nil, err
		}
		dr, err := r.dec.Decode(bits)
		if err != nil {
			return nil, err
		}
		result := zxinggo.NewResult(dr.Text, dr.RawBytes, nil, zxinggo.FormatDataMatrix)
		result.PutMetadata(zxinggo.MetadataSymbologyIdentifier, "]d1")
		return result, nil
	}

	detResult, err := detector.Detect(matrix)
	if err != nil {
		return nil, err
	}

	dr, err := r.dec.Decode(detResult.Bits)
	if err != nil {
		return nil, err
	}

	result := zxinggo.NewResult(dr.Text, dr.RawBytes, detResult.Points, zxinggo.FormatDataMatrix)
	result.PutMetadata(zxinggo.MetadataSymbologyIdentifier, "]d1")
	return result, nil
}

// Reset resets internal state.
func (r *Reader) Reset() {}

// extractPureBits extracts a Data Matrix from a "pure" image — one that
// contains only the unrotated, unskewed barcode with some white border.
func extractPureBits(image *bitutil.BitMatrix) (*bitutil.BitMatrix, error) {
	leftTopBlack := image.TopLeftOnBit()
	rightBottomBlack := image.BottomRightOnBit()
	if leftTopBlack == nil || rightBottomBlack == nil {
		return nil, zxinggo.ErrNotFound
	}

	moduleSize, err := moduleSizePure(leftTopBlack, image)
	if err != nil {
		return nil, err
	}

	top := leftTopBlack[1]
	bottom := rightBottomBlack[1]
	left := leftTopBlack[0]
	right := rightBottomBlack[0]

	matrixWidth := (right - left + 1) / moduleSize
	matrixHeight := (bottom - top + 1) / moduleSize
	if matrixWidth <= 0 || matrixHeight <= 0 {
		return nil, zxinggo.ErrNotFound
	}

	// Nudge to the center of each module
	nudge := moduleSize / 2

	bits := bitutil.NewBitMatrixWithSize(matrixWidth, matrixHeight)
	for y := 0; y < matrixHeight; y++ {
		iOffset := top + y*moduleSize + nudge
		for x := 0; x < matrixWidth; x++ {
			if image.Get(left+x*moduleSize+nudge, iOffset) {
				bits.Set(x, y)
			}
		}
	}
	return bits, nil
}

func moduleSizePure(leftTopBlack []int, image *bitutil.BitMatrix) (int, error) {
	width := image.Width()
	x := leftTopBlack[0]
	y := leftTopBlack[1]

	// Walk right along the top edge to find the module size
	for x < width && image.Get(x, y) {
		x++
	}
	if x == width {
		return 0, zxinggo.ErrNotFound
	}

	moduleSize := x - leftTopBlack[0]
	if moduleSize == 0 {
		return 0, zxinggo.ErrNotFound
	}
	return moduleSize, nil
}

// Compile-time check.
var _ zxinggo.Reader = (*Reader)(nil)
