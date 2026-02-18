// Package zxinggo is a pure Go port of the ZXing barcode library.
package zxinggo

import (
	"math"
	"time"

	"github.com/ericlevine/zxinggo/bitutil"
)

// Format represents a barcode format.
type Format int

const (
	FormatQRCode Format = iota
	FormatPDF417
	FormatCode128
	FormatCode39
	FormatEAN13
	FormatEAN8
	FormatUPCA
	FormatUPCE
	FormatITF
	FormatCodabar
	FormatDataMatrix
	FormatAztec
)

// String returns the name of the barcode format.
func (f Format) String() string {
	switch f {
	case FormatQRCode:
		return "QR_CODE"
	case FormatPDF417:
		return "PDF_417"
	case FormatCode128:
		return "CODE_128"
	case FormatCode39:
		return "CODE_39"
	case FormatEAN13:
		return "EAN_13"
	case FormatEAN8:
		return "EAN_8"
	case FormatUPCA:
		return "UPC_A"
	case FormatUPCE:
		return "UPC_E"
	case FormatITF:
		return "ITF"
	case FormatCodabar:
		return "CODABAR"
	case FormatDataMatrix:
		return "DATA_MATRIX"
	case FormatAztec:
		return "AZTEC"
	default:
		return "UNKNOWN"
	}
}

// ResultMetadataKey identifies a type of metadata about a barcode result.
type ResultMetadataKey int

const (
	MetadataOther ResultMetadataKey = iota
	MetadataOrientation
	MetadataByteSegments
	MetadataErrorCorrectionLevel
	MetadataErrorsCorrected
	MetadataErasuresCorrected
	MetadataIssueNumber
	MetadataSuggestedPrice
	MetadataPossibleCountry
	MetadataUPCEANExtension
	MetadataPDF417ExtraMetadata
	MetadataStructuredAppendSequence
	MetadataStructuredAppendParity
	MetadataSymbologyIdentifier
)

// ResultPoint represents a point of interest in an image.
type ResultPoint struct {
	X, Y float64
}

// Distance returns the distance between two points.
func Distance(a, b ResultPoint) float64 {
	return math.Sqrt((a.X-b.X)*(a.X-b.X) + (a.Y-b.Y)*(a.Y-b.Y))
}

// CrossProductZ computes the z component of the cross product between vectors
// (bX-aX, bY-aY) and (cX-aX, cY-aY).
func CrossProductZ(a, b, c ResultPoint) float64 {
	return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
}

// OrderBestPatterns orders three points in an pointA-pointB-pointC order such
// that AB is less than AC and BC is less than AC.
func OrderBestPatterns(patterns [3]ResultPoint) [3]ResultPoint {
	d01 := Distance(patterns[0], patterns[1])
	d12 := Distance(patterns[1], patterns[2])
	d02 := Distance(patterns[0], patterns[2])

	var pointA, pointB, pointC ResultPoint
	if d12 >= d01 && d12 >= d02 {
		pointA = patterns[0]
		pointB = patterns[1]
		pointC = patterns[2]
	} else if d02 >= d01 && d02 >= d12 {
		pointA = patterns[1]
		pointB = patterns[0]
		pointC = patterns[2]
	} else {
		pointA = patterns[2]
		pointB = patterns[0]
		pointC = patterns[1]
	}

	// Use cross product to determine if pointB and pointC should be swapped
	if CrossProductZ(pointA, pointB, pointC) < 0 {
		pointB, pointC = pointC, pointB
	}

	return [3]ResultPoint{pointA, pointB, pointC}
}

// Result encapsulates the result of decoding a barcode.
type Result struct {
	Text      string
	RawBytes  []byte
	NumBits   int
	Points    []ResultPoint
	Format    Format
	Metadata  map[ResultMetadataKey]interface{}
	Timestamp time.Time
}

// NewResult creates a new Result with the given text, format, and points.
func NewResult(text string, rawBytes []byte, points []ResultPoint, format Format) *Result {
	numBits := 0
	if rawBytes != nil {
		numBits = 8 * len(rawBytes)
	}
	return &Result{
		Text:      text,
		RawBytes:  rawBytes,
		NumBits:   numBits,
		Points:    points,
		Format:    format,
		Metadata:  make(map[ResultMetadataKey]interface{}),
		Timestamp: time.Now(),
	}
}

// PutMetadata adds a metadata key/value pair.
func (r *Result) PutMetadata(key ResultMetadataKey, value interface{}) {
	r.Metadata[key] = value
}

// AddResultPoints appends additional result points.
func (r *Result) AddResultPoints(points []ResultPoint) {
	r.Points = append(r.Points, points...)
}

// BinaryBitmap represents a bitmap of binary (black/white) values.
type BinaryBitmap struct {
	binarizer Binarizer
	matrix    *bitutil.BitMatrix
}

// NewBinaryBitmap creates a new BinaryBitmap from the given Binarizer.
func NewBinaryBitmap(binarizer Binarizer) *BinaryBitmap {
	return &BinaryBitmap{binarizer: binarizer}
}

// Width returns the width of the bitmap.
func (b *BinaryBitmap) Width() int {
	return b.binarizer.Width()
}

// Height returns the height of the bitmap.
func (b *BinaryBitmap) Height() int {
	return b.binarizer.Height()
}

// BlackRow returns a row of black/white values.
func (b *BinaryBitmap) BlackRow(y int, row *bitutil.BitArray) (*bitutil.BitArray, error) {
	return b.binarizer.BlackRow(y, row)
}

// BlackMatrix returns the 2D matrix of black/white values.
func (b *BinaryBitmap) BlackMatrix() (*bitutil.BitMatrix, error) {
	if b.matrix != nil {
		return b.matrix, nil
	}
	m, err := b.binarizer.BlackMatrix()
	if err != nil {
		return nil, err
	}
	b.matrix = m
	return m, nil
}
