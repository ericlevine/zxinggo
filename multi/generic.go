// Package multi provides multiple barcode detection.
package multi

import (
	zxinggo "github.com/ericlevine/zxinggo"
)

const (
	minDimensionToRecur = 100
	maxDepth            = 4
)

// GenericMultipleBarcodeReader attempts to locate multiple barcodes in an image
// by repeatedly decoding portions of the image. After one barcode is found, the
// areas left, above, right and below the barcode's ResultPoints are scanned
// recursively.
type GenericMultipleBarcodeReader struct {
	delegate zxinggo.Reader
}

// NewGenericMultipleBarcodeReader creates a new GenericMultipleBarcodeReader
// with the given delegate reader.
func NewGenericMultipleBarcodeReader(delegate zxinggo.Reader) *GenericMultipleBarcodeReader {
	return &GenericMultipleBarcodeReader{delegate: delegate}
}

// DecodeMultiple attempts to decode all barcodes in the image.
func (r *GenericMultipleBarcodeReader) DecodeMultiple(image *zxinggo.BinaryBitmap, opts *zxinggo.DecodeOptions) ([]*zxinggo.Result, error) {
	var results []*zxinggo.Result
	r.doDecodeMultiple(image, opts, &results, 0, 0, 0)
	if len(results) == 0 {
		return nil, zxinggo.ErrNotFound
	}
	return results, nil
}

func (r *GenericMultipleBarcodeReader) doDecodeMultiple(
	image *zxinggo.BinaryBitmap,
	opts *zxinggo.DecodeOptions,
	results *[]*zxinggo.Result,
	xOffset, yOffset, currentDepth int,
) {
	if currentDepth > maxDepth {
		return
	}

	result, err := r.delegate.Decode(image, opts)
	if err != nil {
		return
	}

	// Deduplicate by text content
	alreadyFound := false
	for _, existing := range *results {
		if existing.Text == result.Text {
			alreadyFound = true
			break
		}
	}
	if !alreadyFound {
		*results = append(*results, translateResultPoints(result, xOffset, yOffset))
	}

	points := result.Points
	if len(points) == 0 {
		return
	}

	width := image.Width()
	height := image.Height()
	minX := float64(width)
	minY := float64(height)
	maxX := 0.0
	maxY := 0.0
	for _, p := range points {
		if p.X < minX {
			minX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}

	// Decode left of barcode
	if minX > float64(minDimensionToRecur) {
		cropped := image.Crop(0, 0, int(minX), height)
		if cropped != nil {
			r.doDecodeMultiple(cropped, opts, results, xOffset, yOffset, currentDepth+1)
		}
	}
	// Decode above barcode
	if minY > float64(minDimensionToRecur) {
		cropped := image.Crop(0, 0, width, int(minY))
		if cropped != nil {
			r.doDecodeMultiple(cropped, opts, results, xOffset, yOffset, currentDepth+1)
		}
	}
	// Decode right of barcode
	if maxX < float64(width-minDimensionToRecur) {
		cropped := image.Crop(int(maxX), 0, width-int(maxX), height)
		if cropped != nil {
			r.doDecodeMultiple(cropped, opts, results, xOffset+int(maxX), yOffset, currentDepth+1)
		}
	}
	// Decode below barcode
	if maxY < float64(height-minDimensionToRecur) {
		cropped := image.Crop(0, int(maxY), width, height-int(maxY))
		if cropped != nil {
			r.doDecodeMultiple(cropped, opts, results, xOffset, yOffset+int(maxY), currentDepth+1)
		}
	}
}

func translateResultPoints(result *zxinggo.Result, xOffset, yOffset int) *zxinggo.Result {
	oldPoints := result.Points
	if len(oldPoints) == 0 {
		return result
	}
	newPoints := make([]zxinggo.ResultPoint, len(oldPoints))
	for i, p := range oldPoints {
		newPoints[i] = zxinggo.ResultPoint{
			X: p.X + float64(xOffset),
			Y: p.Y + float64(yOffset),
		}
	}
	newResult := zxinggo.NewResult(result.Text, result.RawBytes, newPoints, result.Format)
	newResult.NumBits = result.NumBits
	newResult.Timestamp = result.Timestamp
	for k, v := range result.Metadata {
		newResult.PutMetadata(k, v)
	}
	return newResult
}
