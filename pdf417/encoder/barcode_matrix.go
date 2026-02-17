// Copyright 2011 ZXing authors. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Ported from Java ZXing library.

package encoder

// BarcodeRow represents a single row in a barcode matrix.
type BarcodeRow struct {
	row             []byte
	currentLocation int
}

// NewBarcodeRow creates a BarcodeRow of the given width.
func NewBarcodeRow(width int) *BarcodeRow {
	return &BarcodeRow{
		row:             make([]byte, width),
		currentLocation: 0,
	}
}

// Set sets a specific location in the bar.
func (br *BarcodeRow) Set(x int, value byte) {
	br.row[x] = value
}

// AddBar adds a bar (black or white) of the given width at the current location.
func (br *BarcodeRow) AddBar(black bool, width int) {
	var value byte
	if black {
		value = 1
	}
	for i := 0; i < width; i++ {
		br.row[br.currentLocation] = value
		br.currentLocation++
	}
}

// GetScaledRow returns a scaled version of the row.
func (br *BarcodeRow) GetScaledRow(scale int) []byte {
	output := make([]byte, len(br.row)*scale)
	for i := range output {
		output[i] = br.row[i/scale]
	}
	return output
}

// BarcodeMatrix holds all of the information for a barcode in a format where
// it can be easily accessible.
type BarcodeMatrix struct {
	matrix     []*BarcodeRow
	currentRow int
	height     int
	width      int
}

// NewBarcodeMatrix creates a new BarcodeMatrix with the given height (rows)
// and width (cols).
func NewBarcodeMatrix(height, width int) *BarcodeMatrix {
	m := &BarcodeMatrix{
		matrix:     make([]*BarcodeRow, height),
		currentRow: -1,
		height:     height,
		width:      width * 17,
	}
	for i := range m.matrix {
		m.matrix[i] = NewBarcodeRow((width + 4) * 17 + 1)
	}
	return m
}

// Set sets a specific location in the matrix.
func (bm *BarcodeMatrix) Set(x, y int, value byte) {
	bm.matrix[y].Set(x, value)
}

// StartRow increments the current row counter.
func (bm *BarcodeMatrix) StartRow() {
	bm.currentRow++
}

// CurrentRow returns the current BarcodeRow.
func (bm *BarcodeMatrix) CurrentRow() *BarcodeRow {
	return bm.matrix[bm.currentRow]
}

// Matrix returns the unscaled matrix as a 2D byte slice.
func (bm *BarcodeMatrix) Matrix() [][]byte {
	return bm.ScaledMatrix(1, 1)
}

// ScaledMatrix returns the matrix scaled by the given x and y scale factors.
func (bm *BarcodeMatrix) ScaledMatrix(xScale, yScale int) [][]byte {
	matrixOut := make([][]byte, bm.height*yScale)
	yMax := bm.height * yScale
	for i := 0; i < yMax; i++ {
		matrixOut[yMax-i-1] = bm.matrix[i/yScale].GetScaledRow(xScale)
	}
	return matrixOut
}
