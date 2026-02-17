package pdf417

import (
	"fmt"
	"math"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/pdf417/decoder"
	"github.com/ericlevine/zxinggo/pdf417/detector"
)

// PDF417Reader decodes PDF417 barcodes from binary images.
type PDF417Reader struct{}

// NewPDF417Reader creates a new PDF417 reader.
func NewPDF417Reader() *PDF417Reader {
	return &PDF417Reader{}
}

// Decode locates and decodes a PDF417 barcode in the given image.
func (r *PDF417Reader) Decode(image *zxinggo.BinaryBitmap, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	results, err := r.decode(image, opts, false)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, zxinggo.ErrNotFound
	}
	return results[0], nil
}

// DecodeMultiple locates and decodes all PDF417 barcodes in the given image.
func (r *PDF417Reader) DecodeMultiple(image *zxinggo.BinaryBitmap, opts *zxinggo.DecodeOptions) ([]*zxinggo.Result, error) {
	return r.decode(image, opts, true)
}

func (r *PDF417Reader) decode(image *zxinggo.BinaryBitmap, opts *zxinggo.DecodeOptions, multiple bool) ([]*zxinggo.Result, error) {
	matrix, err := image.BlackMatrix()
	if err != nil {
		return nil, err
	}

	detResult, err := detector.Detect(matrix, multiple)
	if err != nil {
		return nil, err
	}

	var results []*zxinggo.Result
	for _, points := range detResult.Points {
		if len(points) < 8 {
			continue
		}
		dr, err := decoder.Decode(
			detResult.Bits,
			points[4], // imageTopLeft
			points[5], // imageBottomLeft
			points[6], // imageTopRight
			points[7], // imageBottomRight
			getMinCodewordWidth(points),
			getMaxCodewordWidth(points),
		)
		if err != nil {
			continue
		}

		result := zxinggo.NewResult(
			dr.Text,
			dr.RawBytes,
			[]zxinggo.ResultPoint{},
			zxinggo.FormatPDF417,
		)

		result.PutMetadata(zxinggo.MetadataErrorCorrectionLevel, dr.ECLevel)
		result.PutMetadata(zxinggo.MetadataErrorsCorrected, dr.ErrorsCorrected)
		result.PutMetadata(zxinggo.MetadataErasuresCorrected, dr.Erasures)
		if dr.Other != nil {
			result.PutMetadata(zxinggo.MetadataPDF417ExtraMetadata, dr.Other)
		}
		result.PutMetadata(zxinggo.MetadataSymbologyIdentifier, fmt.Sprintf("]L%d", dr.SymbologyModifier))

		results = append(results, result)
	}

	if len(results) == 0 {
		return nil, zxinggo.ErrNotFound
	}
	return results, nil
}

// Reset resets internal state.
func (r *PDF417Reader) Reset() {}

func getMinWidth(p1, p2 *zxinggo.ResultPoint) int {
	if p1 == nil || p2 == nil {
		return 0
	}
	return int(math.Abs(p1.X - p2.X))
}

func getMaxWidth(p1, p2 *zxinggo.ResultPoint) int {
	if p1 == nil || p2 == nil {
		return 0
	}
	return int(math.Abs(p1.X-p2.X)) | 1 // ensure odd
}

func getMinCodewordWidth(points []*zxinggo.ResultPoint) int {
	return min(
		getMinWidth(points[0], points[4]),
		getMinWidth(points[6], points[2]),
		getMinWidth(points[1], points[5]),
		getMinWidth(points[7], points[3]),
	)
}

func getMaxCodewordWidth(points []*zxinggo.ResultPoint) int {
	return max(
		getMaxWidth(points[0], points[4]),
		getMaxWidth(points[6], points[2]),
		getMaxWidth(points[1], points[5]),
		getMaxWidth(points[7], points[3]),
	)
}
