package decoder

import "fmt"

const maxNearbyDistance = 5

// DetectionResultColumn represents a column of codewords in a PDF417 detection result.
type DetectionResultColumn struct {
	boundingBox *BoundingBox
	codewords   []*Codeword
}

// NewDetectionResultColumn creates a new DetectionResultColumn.
func NewDetectionResultColumn(boundingBox *BoundingBox) *DetectionResultColumn {
	return &DetectionResultColumn{
		boundingBox: CopyBoundingBox(boundingBox),
		codewords:   make([]*Codeword, boundingBox.MaxY()-boundingBox.MinY()+1),
	}
}

// CodewordNearby returns the codeword at the given image row, or the nearest
// codeword within maxNearbyDistance rows.
func (col *DetectionResultColumn) CodewordNearby(imageRow int) *Codeword {
	codeword := col.Codeword(imageRow)
	if codeword != nil {
		return codeword
	}
	for i := 1; i < maxNearbyDistance; i++ {
		nearImageRow := col.ImageRowToCodewordIndex(imageRow) - i
		if nearImageRow >= 0 {
			codeword = col.codewords[nearImageRow]
			if codeword != nil {
				return codeword
			}
		}
		nearImageRow = col.ImageRowToCodewordIndex(imageRow) + i
		if nearImageRow < len(col.codewords) {
			codeword = col.codewords[nearImageRow]
			if codeword != nil {
				return codeword
			}
		}
	}
	return nil
}

// ImageRowToCodewordIndex converts an image row to a codeword index in this column.
func (col *DetectionResultColumn) ImageRowToCodewordIndex(imageRow int) int {
	return imageRow - col.boundingBox.MinY()
}

// SetCodeword sets the codeword at the given image row.
func (col *DetectionResultColumn) SetCodeword(imageRow int, codeword *Codeword) {
	col.codewords[col.ImageRowToCodewordIndex(imageRow)] = codeword
}

// Codeword returns the codeword at the given image row.
func (col *DetectionResultColumn) Codeword(imageRow int) *Codeword {
	return col.codewords[col.ImageRowToCodewordIndex(imageRow)]
}

// GetBoundingBox returns the bounding box of this column.
func (col *DetectionResultColumn) GetBoundingBox() *BoundingBox {
	return col.boundingBox
}

// Codewords returns the codeword array for this column.
func (col *DetectionResultColumn) Codewords() []*Codeword {
	return col.codewords
}

// String returns a string representation of the column.
func (col *DetectionResultColumn) String() string {
	result := ""
	row := 0
	for _, codeword := range col.codewords {
		if codeword == nil {
			result += fmt.Sprintf("%3d:    |   \n", row)
		} else {
			result += fmt.Sprintf("%3d: %3d|%3d\n", row, codeword.RowNumber(), codeword.Value())
		}
		row++
	}
	return result
}
