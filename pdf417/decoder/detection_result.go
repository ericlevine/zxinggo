package decoder

import "fmt"

const adjustRowNumberSkip = 2

// DetectionResult holds the complete detection result of a PDF417 barcode,
// including all column detection results and barcode metadata.
type DetectionResult struct {
	barcodeMetadata        *BarcodeMetadata
	detectionResultColumns []DetectionResultColumnI
	boundingBox            *BoundingBox
	barcodeColumnCount     int
}

// DetectionResultColumnI is an interface for detection result columns, allowing
// both regular columns and row indicator columns to be stored together.
type DetectionResultColumnI interface {
	CodewordNearby(imageRow int) *Codeword
	ImageRowToCodewordIndex(imageRow int) int
	SetCodeword(imageRow int, codeword *Codeword)
	Codeword(imageRow int) *Codeword
	GetBoundingBox() *BoundingBox
	Codewords() []*Codeword
	String() string
}

// NewDetectionResult creates a new DetectionResult.
func NewDetectionResult(barcodeMetadata *BarcodeMetadata, boundingBox *BoundingBox) *DetectionResult {
	return &DetectionResult{
		barcodeMetadata:        barcodeMetadata,
		barcodeColumnCount:     barcodeMetadata.ColumnCount(),
		boundingBox:            boundingBox,
		detectionResultColumns: make([]DetectionResultColumnI, barcodeMetadata.ColumnCount()+2),
	}
}

// GetDetectionResultColumns adjusts row numbers and returns the detection
// result columns.
func (dr *DetectionResult) GetDetectionResultColumns() []DetectionResultColumnI {
	dr.adjustIndicatorColumnRowNumbers(dr.detectionResultColumns[0])
	dr.adjustIndicatorColumnRowNumbers(dr.detectionResultColumns[dr.barcodeColumnCount+1])
	unadjustedCodewordCount := maxCodewordsInBarcode
	var previousUnadjustedCount int
	for {
		previousUnadjustedCount = unadjustedCodewordCount
		unadjustedCodewordCount = dr.adjustRowNumbers()
		if unadjustedCodewordCount <= 0 || unadjustedCodewordCount >= previousUnadjustedCount {
			break
		}
	}
	return dr.detectionResultColumns
}

func (dr *DetectionResult) adjustIndicatorColumnRowNumbers(col DetectionResultColumnI) {
	if col != nil {
		if ric, ok := col.(*DetectionResultRowIndicatorColumn); ok && ric != nil {
			ric.AdjustCompleteIndicatorColumnRowNumbers(dr.barcodeMetadata)
		}
	}
}

func (dr *DetectionResult) adjustRowNumbers() int {
	unadjustedCount := dr.adjustRowNumbersByRow()
	if unadjustedCount == 0 {
		return 0
	}
	for barcodeColumn := 1; barcodeColumn < dr.barcodeColumnCount+1; barcodeColumn++ {
		codewords := dr.detectionResultColumns[barcodeColumn].Codewords()
		for codewordsRow := 0; codewordsRow < len(codewords); codewordsRow++ {
			if codewords[codewordsRow] == nil {
				continue
			}
			if !codewords[codewordsRow].HasValidRowNumber() {
				dr.adjustRowNumbersSingle(barcodeColumn, codewordsRow, codewords)
			}
		}
	}
	return unadjustedCount
}

func (dr *DetectionResult) adjustRowNumbersByRow() int {
	dr.adjustRowNumbersFromBothRI()
	unadjustedCount := dr.adjustRowNumbersFromLRI()
	return unadjustedCount + dr.adjustRowNumbersFromRRI()
}

func (dr *DetectionResult) adjustRowNumbersFromBothRI() {
	if dr.detectionResultColumns[0] == nil || dr.detectionResultColumns[dr.barcodeColumnCount+1] == nil {
		return
	}
	lriCodewords := dr.detectionResultColumns[0].Codewords()
	rriCodewords := dr.detectionResultColumns[dr.barcodeColumnCount+1].Codewords()
	for codewordsRow := 0; codewordsRow < len(lriCodewords); codewordsRow++ {
		if lriCodewords[codewordsRow] != nil &&
			rriCodewords[codewordsRow] != nil &&
			lriCodewords[codewordsRow].RowNumber() == rriCodewords[codewordsRow].RowNumber() {
			for barcodeColumn := 1; barcodeColumn <= dr.barcodeColumnCount; barcodeColumn++ {
				codeword := dr.detectionResultColumns[barcodeColumn].Codewords()[codewordsRow]
				if codeword == nil {
					continue
				}
				codeword.SetRowNumber(lriCodewords[codewordsRow].RowNumber())
				if !codeword.HasValidRowNumber() {
					dr.detectionResultColumns[barcodeColumn].Codewords()[codewordsRow] = nil
				}
			}
		}
	}
}

func (dr *DetectionResult) adjustRowNumbersFromRRI() int {
	if dr.detectionResultColumns[dr.barcodeColumnCount+1] == nil {
		return 0
	}
	unadjustedCount := 0
	codewords := dr.detectionResultColumns[dr.barcodeColumnCount+1].Codewords()
	for codewordsRow := 0; codewordsRow < len(codewords); codewordsRow++ {
		if codewords[codewordsRow] == nil {
			continue
		}
		rowIndicatorRowNumber := codewords[codewordsRow].RowNumber()
		invalidRowCounts := 0
		for barcodeColumn := dr.barcodeColumnCount + 1; barcodeColumn > 0 && invalidRowCounts < adjustRowNumberSkip; barcodeColumn-- {
			codeword := dr.detectionResultColumns[barcodeColumn].Codewords()[codewordsRow]
			if codeword != nil {
				invalidRowCounts = adjustRowNumberIfValid(rowIndicatorRowNumber, invalidRowCounts, codeword)
				if !codeword.HasValidRowNumber() {
					unadjustedCount++
				}
			}
		}
	}
	return unadjustedCount
}

func (dr *DetectionResult) adjustRowNumbersFromLRI() int {
	if dr.detectionResultColumns[0] == nil {
		return 0
	}
	unadjustedCount := 0
	codewords := dr.detectionResultColumns[0].Codewords()
	for codewordsRow := 0; codewordsRow < len(codewords); codewordsRow++ {
		if codewords[codewordsRow] == nil {
			continue
		}
		rowIndicatorRowNumber := codewords[codewordsRow].RowNumber()
		invalidRowCounts := 0
		for barcodeColumn := 1; barcodeColumn < dr.barcodeColumnCount+1 && invalidRowCounts < adjustRowNumberSkip; barcodeColumn++ {
			codeword := dr.detectionResultColumns[barcodeColumn].Codewords()[codewordsRow]
			if codeword != nil {
				invalidRowCounts = adjustRowNumberIfValid(rowIndicatorRowNumber, invalidRowCounts, codeword)
				if !codeword.HasValidRowNumber() {
					unadjustedCount++
				}
			}
		}
	}
	return unadjustedCount
}

func adjustRowNumberIfValid(rowIndicatorRowNumber, invalidRowCounts int, codeword *Codeword) int {
	if codeword == nil {
		return invalidRowCounts
	}
	if !codeword.HasValidRowNumber() {
		if codeword.IsValidRowNumber(rowIndicatorRowNumber) {
			codeword.SetRowNumber(rowIndicatorRowNumber)
			invalidRowCounts = 0
		} else {
			invalidRowCounts++
		}
	}
	return invalidRowCounts
}

func (dr *DetectionResult) adjustRowNumbersSingle(barcodeColumn, codewordsRow int, codewords []*Codeword) {
	codeword := codewords[codewordsRow]
	previousColumnCodewords := dr.detectionResultColumns[barcodeColumn-1].Codewords()
	nextColumnCodewords := previousColumnCodewords
	if dr.detectionResultColumns[barcodeColumn+1] != nil {
		nextColumnCodewords = dr.detectionResultColumns[barcodeColumn+1].Codewords()
	}

	otherCodewords := make([]*Codeword, 14)

	otherCodewords[2] = previousColumnCodewords[codewordsRow]
	otherCodewords[3] = nextColumnCodewords[codewordsRow]

	if codewordsRow > 0 {
		otherCodewords[0] = codewords[codewordsRow-1]
		otherCodewords[4] = previousColumnCodewords[codewordsRow-1]
		otherCodewords[5] = nextColumnCodewords[codewordsRow-1]
	}
	if codewordsRow > 1 {
		otherCodewords[8] = codewords[codewordsRow-2]
		otherCodewords[10] = previousColumnCodewords[codewordsRow-2]
		otherCodewords[11] = nextColumnCodewords[codewordsRow-2]
	}
	if codewordsRow < len(codewords)-1 {
		otherCodewords[1] = codewords[codewordsRow+1]
		otherCodewords[6] = previousColumnCodewords[codewordsRow+1]
		otherCodewords[7] = nextColumnCodewords[codewordsRow+1]
	}
	if codewordsRow < len(codewords)-2 {
		otherCodewords[9] = codewords[codewordsRow+2]
		otherCodewords[12] = previousColumnCodewords[codewordsRow+2]
		otherCodewords[13] = nextColumnCodewords[codewordsRow+2]
	}
	for _, otherCodeword := range otherCodewords {
		if adjustRowNumber(codeword, otherCodeword) {
			return
		}
	}
}

func adjustRowNumber(codeword, otherCodeword *Codeword) bool {
	if otherCodeword == nil {
		return false
	}
	if otherCodeword.HasValidRowNumber() && otherCodeword.Bucket() == codeword.Bucket() {
		codeword.SetRowNumber(otherCodeword.RowNumber())
		return true
	}
	return false
}

// BarcodeColumnCount returns the number of data columns.
func (dr *DetectionResult) BarcodeColumnCount() int {
	return dr.barcodeColumnCount
}

// BarcodeRowCount returns the total number of rows.
func (dr *DetectionResult) BarcodeRowCount() int {
	return dr.barcodeMetadata.RowCount()
}

// BarcodeECLevel returns the error correction level.
func (dr *DetectionResult) BarcodeECLevel() int {
	return dr.barcodeMetadata.ErrorCorrectionLevel()
}

// SetBoundingBox sets the bounding box.
func (dr *DetectionResult) SetBoundingBox(boundingBox *BoundingBox) {
	dr.boundingBox = boundingBox
}

// GetBoundingBox returns the bounding box.
func (dr *DetectionResult) GetBoundingBox() *BoundingBox {
	return dr.boundingBox
}

// SetDetectionResultColumn sets the detection result column at the given index.
func (dr *DetectionResult) SetDetectionResultColumn(barcodeColumn int, col DetectionResultColumnI) {
	dr.detectionResultColumns[barcodeColumn] = col
}

// GetDetectionResultColumn returns the detection result column at the given index.
func (dr *DetectionResult) GetDetectionResultColumn(barcodeColumn int) DetectionResultColumnI {
	return dr.detectionResultColumns[barcodeColumn]
}

// String returns a string representation of the detection result.
func (dr *DetectionResult) String() string {
	var rowIndicatorColumn DetectionResultColumnI
	rowIndicatorColumn = dr.detectionResultColumns[0]
	if rowIndicatorColumn == nil {
		rowIndicatorColumn = dr.detectionResultColumns[dr.barcodeColumnCount+1]
	}
	result := ""
	for codewordsRow := 0; codewordsRow < len(rowIndicatorColumn.Codewords()); codewordsRow++ {
		result += fmt.Sprintf("CW %3d:", codewordsRow)
		for barcodeColumn := 0; barcodeColumn < dr.barcodeColumnCount+2; barcodeColumn++ {
			if dr.detectionResultColumns[barcodeColumn] == nil {
				result += "    |   "
				continue
			}
			codeword := dr.detectionResultColumns[barcodeColumn].Codewords()[codewordsRow]
			if codeword == nil {
				result += "    |   "
				continue
			}
			result += fmt.Sprintf(" %3d|%3d", codeword.RowNumber(), codeword.Value())
		}
		result += "\n"
	}
	return result
}
