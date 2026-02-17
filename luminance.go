package zxinggo

import "github.com/ericlevine/zxinggo/bitutil"

// LuminanceSource provides access to greyscale luminance values for an image.
type LuminanceSource interface {
	// Row returns a row of luminance data. If row is non-nil and large enough,
	// it should be reused.
	Row(y int, row []byte) []byte

	// Matrix returns the entire luminance matrix.
	Matrix() []byte

	// Width returns the width of the image.
	Width() int

	// Height returns the height of the image.
	Height() int
}

// Binarizer converts luminance data to 1-bit black/white data.
type Binarizer interface {
	// BlackRow returns a row of black/white values.
	BlackRow(y int, row *bitutil.BitArray) (*bitutil.BitArray, error)

	// BlackMatrix returns the 2D matrix of black/white values.
	BlackMatrix() (*bitutil.BitMatrix, error)

	// LuminanceSource returns the underlying LuminanceSource.
	LuminanceSource() LuminanceSource

	// Width returns the width of the image.
	Width() int

	// Height returns the height of the image.
	Height() int
}
