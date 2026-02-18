// Package qrcode provides multi-QR code detection and structured append support.
package qrcode

import (
	"fmt"
	"sort"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/qrcode/decoder"
	"github.com/ericlevine/zxinggo/qrcode/detector"
)

// QRCodeMultiReader can detect and decode multiple QR codes in an image,
// and also combines structured append results.
type QRCodeMultiReader struct {
	dec *decoder.Decoder
}

// NewQRCodeMultiReader creates a new QRCodeMultiReader.
func NewQRCodeMultiReader() *QRCodeMultiReader {
	return &QRCodeMultiReader{dec: decoder.NewDecoder()}
}

// DecodeMultiple detects and decodes all QR codes in the image.
func (r *QRCodeMultiReader) DecodeMultiple(image *zxinggo.BinaryBitmap, opts *zxinggo.DecodeOptions) ([]*zxinggo.Result, error) {
	if opts == nil {
		opts = &zxinggo.DecodeOptions{}
	}

	matrix, err := image.BlackMatrix()
	if err != nil {
		return nil, err
	}

	detectorResults, err := detector.DetectMulti(matrix, opts.TryHarder)
	if err != nil {
		return nil, err
	}

	var results []*zxinggo.Result
	for _, detResult := range detectorResults {
		dr, err := r.dec.Decode(detResult.Bits, opts.CharacterSet)
		if err != nil {
			continue
		}

		points := make([]zxinggo.ResultPoint, len(detResult.Points))
		for i, p := range detResult.Points {
			points[i] = zxinggo.ResultPoint{X: p.X, Y: p.Y}
		}

		result := zxinggo.NewResult(dr.Text, dr.RawBytes, points, zxinggo.FormatQRCode)
		if dr.ByteSegments != nil {
			result.PutMetadata(zxinggo.MetadataByteSegments, dr.ByteSegments)
		}
		if dr.ECLevel != "" {
			result.PutMetadata(zxinggo.MetadataErrorCorrectionLevel, dr.ECLevel)
		}
		if dr.HasStructuredAppend() {
			result.PutMetadata(zxinggo.MetadataStructuredAppendSequence, dr.StructuredAppendSequenceNumber)
			result.PutMetadata(zxinggo.MetadataStructuredAppendParity, dr.StructuredAppendParity)
		}
		result.PutMetadata(zxinggo.MetadataErrorsCorrected, dr.ErrorsCorrected)
		result.PutMetadata(zxinggo.MetadataSymbologyIdentifier, fmt.Sprintf("]Q%d", dr.SymbologyModifier))

		results = append(results, result)
	}

	if len(results) == 0 {
		return nil, zxinggo.ErrNotFound
	}

	results = processStructuredAppend(results)
	return results, nil
}

// Decode decodes a single QR code (delegate to standard reader behavior).
func (r *QRCodeMultiReader) Decode(image *zxinggo.BinaryBitmap, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	results, err := r.DecodeMultiple(image, opts)
	if err != nil {
		return nil, err
	}
	return results[0], nil
}

// Reset is a no-op.
func (r *QRCodeMultiReader) Reset() {}

func processStructuredAppend(results []*zxinggo.Result) []*zxinggo.Result {
	var newResults []*zxinggo.Result
	var saResults []*zxinggo.Result

	for _, result := range results {
		if _, ok := result.Metadata[zxinggo.MetadataStructuredAppendSequence]; ok {
			saResults = append(saResults, result)
		} else {
			newResults = append(newResults, result)
		}
	}

	if len(saResults) == 0 {
		return results
	}

	// Sort by sequence number
	sort.Slice(saResults, func(i, j int) bool {
		seqI, _ := saResults[i].Metadata[zxinggo.MetadataStructuredAppendSequence].(int)
		seqJ, _ := saResults[j].Metadata[zxinggo.MetadataStructuredAppendSequence].(int)
		return seqI < seqJ
	})

	// Concatenate text and raw bytes
	var combinedText string
	var combinedRawBytes []byte
	var combinedByteSegment []byte
	for _, sa := range saResults {
		combinedText += sa.Text
		if sa.RawBytes != nil {
			combinedRawBytes = append(combinedRawBytes, sa.RawBytes...)
		}
		if segs, ok := sa.Metadata[zxinggo.MetadataByteSegments].([][]byte); ok {
			for _, seg := range segs {
				combinedByteSegment = append(combinedByteSegment, seg...)
			}
		}
	}

	combined := zxinggo.NewResult(combinedText, combinedRawBytes, nil, zxinggo.FormatQRCode)
	if len(combinedByteSegment) > 0 {
		combined.PutMetadata(zxinggo.MetadataByteSegments, [][]byte{combinedByteSegment})
	}
	newResults = append(newResults, combined)
	return newResults
}

// DecodeMultipleFromResults is a convenience for combining results that may
// have been decoded separately but share structured append metadata.
func DecodeMultipleFromResults(results []*zxinggo.Result) []*zxinggo.Result {
	return processStructuredAppend(results)
}

// ensure interface compliance
var _ zxinggo.MultipleBarcodeReader = (*QRCodeMultiReader)(nil)
var _ zxinggo.Reader = (*QRCodeMultiReader)(nil)
