package decoder

// detection_result_row_indicator.go

// DetectionResultRowIndicatorColumn is a specialized DetectionResultColumn
// for left or right row indicator columns.
type DetectionResultRowIndicatorColumn struct {
	*DetectionResultColumn
	isLeft bool
}

// NewDetectionResultRowIndicatorColumn creates a new row indicator column.
func NewDetectionResultRowIndicatorColumn(boundingBox *BoundingBox, isLeft bool) *DetectionResultRowIndicatorColumn {
	return &DetectionResultRowIndicatorColumn{
		DetectionResultColumn: NewDetectionResultColumn(boundingBox),
		isLeft:                isLeft,
	}
}

func (col *DetectionResultRowIndicatorColumn) setRowNumbers() {
	for _, codeword := range col.Codewords() {
		if codeword != nil {
			codeword.SetRowNumberAsRowIndicatorColumn()
		}
	}
}

// AdjustCompleteIndicatorColumnRowNumbers adjusts the row numbers of all
// codewords in this indicator column using the barcode metadata.
func (col *DetectionResultRowIndicatorColumn) AdjustCompleteIndicatorColumnRowNumbers(barcodeMetadata *BarcodeMetadata) {
	codewords := col.Codewords()
	col.setRowNumbers()
	col.removeIncorrectCodewords(codewords, barcodeMetadata)
	boundingBox := col.GetBoundingBox()
	var topY, bottomY float64
	if col.isLeft {
		topY = boundingBox.TopLeft().Y
		bottomY = boundingBox.BottomLeft().Y
	} else {
		topY = boundingBox.TopRight().Y
		bottomY = boundingBox.BottomRight().Y
	}
	firstRow := col.ImageRowToCodewordIndex(int(topY))
	lastRow := col.ImageRowToCodewordIndex(int(bottomY))
	barcodeRow := -1
	maxRowHeight := 1
	currentRowHeight := 0
	for codewordsRow := firstRow; codewordsRow < lastRow; codewordsRow++ {
		if codewords[codewordsRow] == nil {
			continue
		}
		codeword := codewords[codewordsRow]
		rowDifference := codeword.RowNumber() - barcodeRow

		if rowDifference == 0 {
			currentRowHeight++
		} else if rowDifference == 1 {
			if currentRowHeight > maxRowHeight {
				maxRowHeight = currentRowHeight
			}
			currentRowHeight = 1
			barcodeRow = codeword.RowNumber()
		} else if rowDifference < 0 ||
			codeword.RowNumber() >= barcodeMetadata.RowCount() ||
			rowDifference > codewordsRow {
			codewords[codewordsRow] = nil
		} else {
			var checkedRows int
			if maxRowHeight > 2 {
				checkedRows = (maxRowHeight - 2) * rowDifference
			} else {
				checkedRows = rowDifference
			}
			closePreviousCodewordFound := checkedRows >= codewordsRow
			for i := 1; i <= checkedRows && !closePreviousCodewordFound; i++ {
				closePreviousCodewordFound = codewords[codewordsRow-i] != nil
			}
			if closePreviousCodewordFound {
				codewords[codewordsRow] = nil
			} else {
				barcodeRow = codeword.RowNumber()
				currentRowHeight = 1
			}
		}
	}
}

// RowHeights returns the height (in image rows) of each barcode row.
// Returns nil if barcode metadata cannot be determined.
func (col *DetectionResultRowIndicatorColumn) RowHeights() []int {
	barcodeMetadata := col.GetBarcodeMetadata()
	if barcodeMetadata == nil {
		return nil
	}
	col.adjustIncompleteIndicatorColumnRowNumbers(barcodeMetadata)
	result := make([]int, barcodeMetadata.RowCount())
	for _, codeword := range col.Codewords() {
		if codeword != nil {
			rowNumber := codeword.RowNumber()
			if rowNumber >= len(result) {
				continue
			}
			result[rowNumber]++
		}
	}
	return result
}

func (col *DetectionResultRowIndicatorColumn) adjustIncompleteIndicatorColumnRowNumbers(barcodeMetadata *BarcodeMetadata) {
	boundingBox := col.GetBoundingBox()
	var topY, bottomY float64
	if col.isLeft {
		topY = boundingBox.TopLeft().Y
		bottomY = boundingBox.BottomLeft().Y
	} else {
		topY = boundingBox.TopRight().Y
		bottomY = boundingBox.BottomRight().Y
	}
	firstRow := col.ImageRowToCodewordIndex(int(topY))
	lastRow := col.ImageRowToCodewordIndex(int(bottomY))
	codewords := col.Codewords()
	barcodeRow := -1
	maxRowHeight := 1
	currentRowHeight := 0
	for codewordsRow := firstRow; codewordsRow < lastRow; codewordsRow++ {
		if codewords[codewordsRow] == nil {
			continue
		}
		codeword := codewords[codewordsRow]
		codeword.SetRowNumberAsRowIndicatorColumn()
		rowDifference := codeword.RowNumber() - barcodeRow

		if rowDifference == 0 {
			currentRowHeight++
		} else if rowDifference == 1 {
			if currentRowHeight > maxRowHeight {
				maxRowHeight = currentRowHeight
			}
			currentRowHeight = 1
			barcodeRow = codeword.RowNumber()
		} else if codeword.RowNumber() >= barcodeMetadata.RowCount() {
			codewords[codewordsRow] = nil
		} else {
			barcodeRow = codeword.RowNumber()
			currentRowHeight = 1
		}
	}
}

// GetBarcodeMetadata extracts barcode metadata from this row indicator column.
// Returns nil if the metadata cannot be determined.
func (col *DetectionResultRowIndicatorColumn) GetBarcodeMetadata() *BarcodeMetadata {
	codewords := col.Codewords()
	barcodeColumnCount := NewBarcodeValue()
	barcodeRowCountUpperPart := NewBarcodeValue()
	barcodeRowCountLowerPart := NewBarcodeValue()
	barcodeECLevel := NewBarcodeValue()
	for _, codeword := range codewords {
		if codeword == nil {
			continue
		}
		codeword.SetRowNumberAsRowIndicatorColumn()
		rowIndicatorValue := codeword.Value() % 30
		codewordRowNumber := codeword.RowNumber()
		if !col.isLeft {
			codewordRowNumber += 2
		}
		switch codewordRowNumber % 3 {
		case 0:
			barcodeRowCountUpperPart.SetValue(rowIndicatorValue*3 + 1)
		case 1:
			barcodeECLevel.SetValue(rowIndicatorValue / 3)
			barcodeRowCountLowerPart.SetValue(rowIndicatorValue % 3)
		case 2:
			barcodeColumnCount.SetValue(rowIndicatorValue + 1)
		}
	}
	columnCountValues := barcodeColumnCount.Value()
	upperPartValues := barcodeRowCountUpperPart.Value()
	lowerPartValues := barcodeRowCountLowerPart.Value()
	ecLevelValues := barcodeECLevel.Value()
	if len(columnCountValues) == 0 ||
		len(upperPartValues) == 0 ||
		len(lowerPartValues) == 0 ||
		len(ecLevelValues) == 0 ||
		columnCountValues[0] < 1 ||
		upperPartValues[0]+lowerPartValues[0] < minRowsInBarcode ||
		upperPartValues[0]+lowerPartValues[0] > maxRowsInBarcode {
		return nil
	}
	barcodeMetadata := NewBarcodeMetadata(
		columnCountValues[0],
		upperPartValues[0],
		lowerPartValues[0],
		ecLevelValues[0],
	)
	col.removeIncorrectCodewords(codewords, barcodeMetadata)
	return barcodeMetadata
}

func (col *DetectionResultRowIndicatorColumn) removeIncorrectCodewords(codewords []*Codeword, barcodeMetadata *BarcodeMetadata) {
	for codewordRow := 0; codewordRow < len(codewords); codewordRow++ {
		codeword := codewords[codewordRow]
		if codeword == nil {
			continue
		}
		rowIndicatorValue := codeword.Value() % 30
		codewordRowNumber := codeword.RowNumber()
		if codewordRowNumber > barcodeMetadata.RowCount() {
			codewords[codewordRow] = nil
			continue
		}
		if !col.isLeft {
			codewordRowNumber += 2
		}
		switch codewordRowNumber % 3 {
		case 0:
			if rowIndicatorValue*3+1 != barcodeMetadata.RowCountUpperPart() {
				codewords[codewordRow] = nil
			}
		case 1:
			if rowIndicatorValue/3 != barcodeMetadata.ErrorCorrectionLevel() ||
				rowIndicatorValue%3 != barcodeMetadata.RowCountLowerPart() {
				codewords[codewordRow] = nil
			}
		case 2:
			if rowIndicatorValue+1 != barcodeMetadata.ColumnCount() {
				codewords[codewordRow] = nil
			}
		}
	}
}

// IsLeft returns true if this is a left row indicator column.
func (col *DetectionResultRowIndicatorColumn) IsLeft() bool {
	return col.isLeft
}

// String returns a string representation of the row indicator column.
func (col *DetectionResultRowIndicatorColumn) String() string {
	isLeftStr := "false"
	if col.isLeft {
		isLeftStr = "true"
	}
	return "IsLeft: " + isLeftStr + "\n" + col.DetectionResultColumn.String()
}
