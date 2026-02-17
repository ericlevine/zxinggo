package internal

import "github.com/ericlevine/zxinggo/bitutil"

// DetectorResult encapsulates the result of detecting a barcode in an image.
type DetectorResult struct {
	Bits   *bitutil.BitMatrix
	Points []ResultPoint
}

// ResultPoint represents a point of interest found by a detector.
type ResultPoint struct {
	X, Y float64
}

// NewDetectorResult creates a new DetectorResult.
func NewDetectorResult(bits *bitutil.BitMatrix, points []ResultPoint) *DetectorResult {
	return &DetectorResult{Bits: bits, Points: points}
}
