package decoder

import "fmt"

const barcodeRowUnknown = -1

// Codeword represents a single codeword in a PDF417 barcode.
type Codeword struct {
	startX    int
	endX      int
	bucket    int
	value     int
	rowNumber int
}

// NewCodeword creates a new Codeword.
func NewCodeword(startX, endX, bucket, value int) *Codeword {
	return &Codeword{
		startX:    startX,
		endX:      endX,
		bucket:    bucket,
		value:     value,
		rowNumber: barcodeRowUnknown,
	}
}

// HasValidRowNumber returns true if this codeword has a valid row number assigned.
func (c *Codeword) HasValidRowNumber() bool {
	return c.IsValidRowNumber(c.rowNumber)
}

// IsValidRowNumber returns true if the given row number is valid for this codeword's bucket.
func (c *Codeword) IsValidRowNumber(rowNumber int) bool {
	return rowNumber != barcodeRowUnknown && c.bucket == (rowNumber%3)*3
}

// SetRowNumberAsRowIndicatorColumn computes and sets the row number
// based on the codeword value and bucket, as used in row indicator columns.
func (c *Codeword) SetRowNumberAsRowIndicatorColumn() {
	c.rowNumber = (c.value/30)*3 + c.bucket/3
}

// Width returns the width of the codeword in pixels.
func (c *Codeword) Width() int {
	return c.endX - c.startX
}

// StartX returns the starting x coordinate.
func (c *Codeword) StartX() int {
	return c.startX
}

// EndX returns the ending x coordinate.
func (c *Codeword) EndX() int {
	return c.endX
}

// Bucket returns the bucket (cluster) value.
func (c *Codeword) Bucket() int {
	return c.bucket
}

// Value returns the codeword value.
func (c *Codeword) Value() int {
	return c.value
}

// RowNumber returns the row number, or barcodeRowUnknown if not set.
func (c *Codeword) RowNumber() int {
	return c.rowNumber
}

// SetRowNumber sets the row number for this codeword.
func (c *Codeword) SetRowNumber(rowNumber int) {
	c.rowNumber = rowNumber
}

// String returns a string representation of the codeword.
func (c *Codeword) String() string {
	return fmt.Sprintf("%d|%d", c.rowNumber, c.value)
}
