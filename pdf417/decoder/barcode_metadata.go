package decoder

// BarcodeMetadata holds metadata about a PDF417 barcode extracted from
// the row indicator columns.
type BarcodeMetadata struct {
	columnCount          int
	errorCorrectionLevel int
	rowCountUpperPart    int
	rowCountLowerPart    int
	rowCount             int
}

// NewBarcodeMetadata creates a new BarcodeMetadata.
func NewBarcodeMetadata(columnCount, rowCountUpperPart, rowCountLowerPart, errorCorrectionLevel int) *BarcodeMetadata {
	return &BarcodeMetadata{
		columnCount:          columnCount,
		errorCorrectionLevel: errorCorrectionLevel,
		rowCountUpperPart:    rowCountUpperPart,
		rowCountLowerPart:    rowCountLowerPart,
		rowCount:             rowCountUpperPart + rowCountLowerPart,
	}
}

// ColumnCount returns the number of data columns in the barcode.
func (bm *BarcodeMetadata) ColumnCount() int {
	return bm.columnCount
}

// ErrorCorrectionLevel returns the error correction level.
func (bm *BarcodeMetadata) ErrorCorrectionLevel() int {
	return bm.errorCorrectionLevel
}

// RowCount returns the total number of rows.
func (bm *BarcodeMetadata) RowCount() int {
	return bm.rowCount
}

// RowCountUpperPart returns the upper part of the row count.
func (bm *BarcodeMetadata) RowCountUpperPart() int {
	return bm.rowCountUpperPart
}

// RowCountLowerPart returns the lower part of the row count.
func (bm *BarcodeMetadata) RowCountLowerPart() int {
	return bm.rowCountLowerPart
}
