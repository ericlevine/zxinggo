package decoder

import (
	"math"
	"strconv"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
	"github.com/ericlevine/zxinggo/internal"
)

const (
	codewordSkewSize = 2
	maxErrors        = 3
	maxECCodewords   = 512
)

var scanErrorCorrection = NewErrorCorrection()

// Decode decodes a PDF417 barcode from the given image and corner points.
// minCodewordWidth and maxCodewordWidth provide bounds on codeword widths.
func Decode(image *bitutil.BitMatrix,
	imageTopLeft, imageBottomLeft, imageTopRight, imageBottomRight *zxinggo.ResultPoint,
	minCodewordWidth, maxCodewordWidth int) (*internal.DecoderResult, error) {

	boundingBox, err := NewBoundingBox(image, imageTopLeft, imageBottomLeft, imageTopRight, imageBottomRight)
	if err != nil {
		return nil, err
	}

	var leftRowIndicatorColumn *DetectionResultRowIndicatorColumn
	var rightRowIndicatorColumn *DetectionResultRowIndicatorColumn
	var detectionResult *DetectionResult

	for firstPass := true; ; firstPass = false {
		if imageTopLeft != nil {
			leftRowIndicatorColumn = getRowIndicatorColumn(image, boundingBox, *imageTopLeft, true, minCodewordWidth, maxCodewordWidth)
		}
		if imageTopRight != nil {
			rightRowIndicatorColumn = getRowIndicatorColumn(image, boundingBox, *imageTopRight, false, minCodewordWidth, maxCodewordWidth)
		}
		detectionResult, err = merge(leftRowIndicatorColumn, rightRowIndicatorColumn)
		if err != nil {
			return nil, err
		}
		if detectionResult == nil {
			return nil, zxinggo.ErrNotFound
		}
		resultBox := detectionResult.GetBoundingBox()
		if firstPass && resultBox != nil &&
			(resultBox.MinY() < boundingBox.MinY() || resultBox.MaxY() > boundingBox.MaxY()) {
			boundingBox = resultBox
		} else {
			break
		}
	}

	detectionResult.SetBoundingBox(boundingBox)
	maxBarcodeColumn := detectionResult.BarcodeColumnCount() + 1
	if leftRowIndicatorColumn != nil {
		detectionResult.SetDetectionResultColumn(0, leftRowIndicatorColumn)
	}
	if rightRowIndicatorColumn != nil {
		detectionResult.SetDetectionResultColumn(maxBarcodeColumn, rightRowIndicatorColumn)
	}

	leftToRight := leftRowIndicatorColumn != nil
	for barcodeColumnCount := 1; barcodeColumnCount <= maxBarcodeColumn; barcodeColumnCount++ {
		barcodeColumn := barcodeColumnCount
		if !leftToRight {
			barcodeColumn = maxBarcodeColumn - barcodeColumnCount
		}
		if detectionResult.GetDetectionResultColumn(barcodeColumn) != nil {
			continue
		}
		var detectionResultColumn DetectionResultColumnI
		if barcodeColumn == 0 || barcodeColumn == maxBarcodeColumn {
			detectionResultColumn = NewDetectionResultRowIndicatorColumn(boundingBox, barcodeColumn == 0)
		} else {
			detectionResultColumn = NewDetectionResultColumn(boundingBox)
		}
		detectionResult.SetDetectionResultColumn(barcodeColumn, detectionResultColumn)
		startColumn := -1
		previousStartColumn := startColumn
		for imageRow := boundingBox.MinY(); imageRow <= boundingBox.MaxY(); imageRow++ {
			startColumn = getStartColumn(detectionResult, barcodeColumn, imageRow, leftToRight)
			if startColumn < 0 || startColumn > boundingBox.MaxX() {
				if previousStartColumn == -1 {
					continue
				}
				startColumn = previousStartColumn
			}
			codeword := detectCodeword(image, boundingBox.MinX(), boundingBox.MaxX(), leftToRight,
				startColumn, imageRow, minCodewordWidth, maxCodewordWidth)
			if codeword != nil {
				detectionResultColumn.SetCodeword(imageRow, codeword)
				previousStartColumn = startColumn
				if codeword.Width() < minCodewordWidth {
					minCodewordWidth = codeword.Width()
				}
				if codeword.Width() > maxCodewordWidth {
					maxCodewordWidth = codeword.Width()
				}
			}
		}
	}
	return createDecoderResult(detectionResult)
}

func merge(leftRowIndicatorColumn, rightRowIndicatorColumn *DetectionResultRowIndicatorColumn) (*DetectionResult, error) {
	if leftRowIndicatorColumn == nil && rightRowIndicatorColumn == nil {
		return nil, nil
	}
	barcodeMetadata := getBarcodeMetadata(leftRowIndicatorColumn, rightRowIndicatorColumn)
	if barcodeMetadata == nil {
		return nil, nil
	}
	leftBox, err := adjustBoundingBox(leftRowIndicatorColumn)
	if err != nil {
		return nil, err
	}
	rightBox, err := adjustBoundingBox(rightRowIndicatorColumn)
	if err != nil {
		return nil, err
	}
	boundingBox, err := MergeBoundingBoxes(leftBox, rightBox)
	if err != nil {
		return nil, err
	}
	return NewDetectionResult(barcodeMetadata, boundingBox), nil
}

func adjustBoundingBox(rowIndicatorColumn *DetectionResultRowIndicatorColumn) (*BoundingBox, error) {
	if rowIndicatorColumn == nil {
		return nil, nil
	}
	rowHeights := rowIndicatorColumn.RowHeights()
	if rowHeights == nil {
		return nil, nil
	}
	maxRowHeight := getMaxInt(rowHeights)
	missingStartRows := 0
	for _, rowHeight := range rowHeights {
		missingStartRows += maxRowHeight - rowHeight
		if rowHeight > 0 {
			break
		}
	}
	codewords := rowIndicatorColumn.Codewords()
	for row := 0; missingStartRows > 0 && codewords[row] == nil; row++ {
		missingStartRows--
	}
	missingEndRows := 0
	for row := len(rowHeights) - 1; row >= 0; row-- {
		missingEndRows += maxRowHeight - rowHeights[row]
		if rowHeights[row] > 0 {
			break
		}
	}
	for row := len(codewords) - 1; missingEndRows > 0 && codewords[row] == nil; row-- {
		missingEndRows--
	}
	return rowIndicatorColumn.GetBoundingBox().AddMissingRows(missingStartRows, missingEndRows, rowIndicatorColumn.IsLeft())
}

func getMaxInt(values []int) int {
	maxValue := -1
	for _, value := range values {
		if value > maxValue {
			maxValue = value
		}
	}
	return maxValue
}

func getBarcodeMetadata(leftRowIndicatorColumn, rightRowIndicatorColumn *DetectionResultRowIndicatorColumn) *BarcodeMetadata {
	var leftBarcodeMetadata *BarcodeMetadata
	if leftRowIndicatorColumn == nil {
		if rightRowIndicatorColumn == nil {
			return nil
		}
		return rightRowIndicatorColumn.GetBarcodeMetadata()
	}
	leftBarcodeMetadata = leftRowIndicatorColumn.GetBarcodeMetadata()
	if leftBarcodeMetadata == nil {
		if rightRowIndicatorColumn == nil {
			return nil
		}
		return rightRowIndicatorColumn.GetBarcodeMetadata()
	}

	var rightBarcodeMetadata *BarcodeMetadata
	if rightRowIndicatorColumn == nil {
		return leftBarcodeMetadata
	}
	rightBarcodeMetadata = rightRowIndicatorColumn.GetBarcodeMetadata()
	if rightBarcodeMetadata == nil {
		return leftBarcodeMetadata
	}

	if leftBarcodeMetadata.ColumnCount() != rightBarcodeMetadata.ColumnCount() &&
		leftBarcodeMetadata.ErrorCorrectionLevel() != rightBarcodeMetadata.ErrorCorrectionLevel() &&
		leftBarcodeMetadata.RowCount() != rightBarcodeMetadata.RowCount() {
		return nil
	}
	return leftBarcodeMetadata
}

func getRowIndicatorColumn(image *bitutil.BitMatrix,
	boundingBox *BoundingBox,
	startPoint zxinggo.ResultPoint,
	leftToRight bool,
	minCodewordWidth, maxCodewordWidth int) *DetectionResultRowIndicatorColumn {

	rowIndicatorColumn := NewDetectionResultRowIndicatorColumn(boundingBox, leftToRight)
	for i := 0; i < 2; i++ {
		increment := 1
		if i != 0 {
			increment = -1
		}
		startColumn := int(startPoint.X)
		for imageRow := int(startPoint.Y); imageRow <= boundingBox.MaxY() && imageRow >= boundingBox.MinY(); imageRow += increment {
			codeword := detectCodeword(image, 0, image.Width(), leftToRight, startColumn, imageRow,
				minCodewordWidth, maxCodewordWidth)
			if codeword != nil {
				rowIndicatorColumn.SetCodeword(imageRow, codeword)
				if leftToRight {
					startColumn = codeword.StartX()
				} else {
					startColumn = codeword.EndX()
				}
			}
		}
	}
	return rowIndicatorColumn
}

func adjustCodewordCount(detectionResult *DetectionResult, barcodeMatrix [][]*BarcodeValue) error {
	barcodeMatrix01 := barcodeMatrix[0][1]
	numberOfCodewords := barcodeMatrix01.Value()
	calculatedNumberOfCodewords := detectionResult.BarcodeColumnCount()*
		detectionResult.BarcodeRowCount() -
		getNumberOfECCodeWords(detectionResult.BarcodeECLevel())
	if len(numberOfCodewords) == 0 {
		if calculatedNumberOfCodewords < 1 || calculatedNumberOfCodewords > maxCodewordsInBarcode {
			return zxinggo.ErrNotFound
		}
		barcodeMatrix01.SetValue(calculatedNumberOfCodewords)
	} else if numberOfCodewords[0] != calculatedNumberOfCodewords &&
		calculatedNumberOfCodewords >= 1 &&
		calculatedNumberOfCodewords <= maxCodewordsInBarcode {
		barcodeMatrix01.SetValue(calculatedNumberOfCodewords)
	}
	return nil
}

func createDecoderResult(detectionResult *DetectionResult) (*internal.DecoderResult, error) {
	barcodeMatrix := createBarcodeMatrix(detectionResult)
	if err := adjustCodewordCount(detectionResult, barcodeMatrix); err != nil {
		return nil, err
	}
	var erasures []int
	codewords := make([]int, detectionResult.BarcodeRowCount()*detectionResult.BarcodeColumnCount())
	var ambiguousIndexValuesList [][]int
	var ambiguousIndexesList []int
	for row := 0; row < detectionResult.BarcodeRowCount(); row++ {
		for column := 0; column < detectionResult.BarcodeColumnCount(); column++ {
			values := barcodeMatrix[row][column+1].Value()
			codewordIndex := row*detectionResult.BarcodeColumnCount() + column
			if len(values) == 0 {
				erasures = append(erasures, codewordIndex)
			} else if len(values) == 1 {
				codewords[codewordIndex] = values[0]
			} else {
				ambiguousIndexesList = append(ambiguousIndexesList, codewordIndex)
				ambiguousIndexValuesList = append(ambiguousIndexValuesList, values)
			}
		}
	}
	return createDecoderResultFromAmbiguousValues(detectionResult.BarcodeECLevel(), codewords,
		erasures, ambiguousIndexesList, ambiguousIndexValuesList)
}

func createDecoderResultFromAmbiguousValues(ecLevel int,
	codewords []int,
	erasureArray []int,
	ambiguousIndexes []int,
	ambiguousIndexValues [][]int) (*internal.DecoderResult, error) {

	ambiguousIndexCount := make([]int, len(ambiguousIndexes))

	tries := 100
	for tries > 0 {
		tries--
		for i := 0; i < len(ambiguousIndexCount); i++ {
			codewords[ambiguousIndexes[i]] = ambiguousIndexValues[i][ambiguousIndexCount[i]]
		}
		result, err := decodeCodewords(codewords, ecLevel, erasureArray)
		if err == nil {
			return result, nil
		}
		if err != zxinggo.ErrChecksum {
			return nil, err
		}
		if len(ambiguousIndexCount) == 0 {
			return nil, zxinggo.ErrChecksum
		}
		for i := 0; i < len(ambiguousIndexCount); i++ {
			if ambiguousIndexCount[i] < len(ambiguousIndexValues[i])-1 {
				ambiguousIndexCount[i]++
				break
			} else {
				ambiguousIndexCount[i] = 0
				if i == len(ambiguousIndexCount)-1 {
					return nil, zxinggo.ErrChecksum
				}
			}
		}
	}
	return nil, zxinggo.ErrChecksum
}

func createBarcodeMatrix(detectionResult *DetectionResult) [][]*BarcodeValue {
	barcodeMatrix := make([][]*BarcodeValue, detectionResult.BarcodeRowCount())
	for row := 0; row < len(barcodeMatrix); row++ {
		barcodeMatrix[row] = make([]*BarcodeValue, detectionResult.BarcodeColumnCount()+2)
		for column := 0; column < len(barcodeMatrix[row]); column++ {
			barcodeMatrix[row][column] = NewBarcodeValue()
		}
	}

	column := 0
	for _, detectionResultColumn := range detectionResult.GetDetectionResultColumns() {
		if detectionResultColumn != nil {
			for _, codeword := range detectionResultColumn.Codewords() {
				if codeword != nil {
					rowNumber := codeword.RowNumber()
					if rowNumber >= 0 {
						if rowNumber >= len(barcodeMatrix) {
							continue
						}
						barcodeMatrix[rowNumber][column].SetValue(codeword.Value())
					}
				}
			}
		}
		column++
	}
	return barcodeMatrix
}

func isValidBarcodeColumn(detectionResult *DetectionResult, barcodeColumn int) bool {
	return barcodeColumn >= 0 && barcodeColumn <= detectionResult.BarcodeColumnCount()+1
}

func getStartColumn(detectionResult *DetectionResult, barcodeColumn, imageRow int, leftToRight bool) int {
	offset := 1
	if !leftToRight {
		offset = -1
	}
	var codeword *Codeword
	if isValidBarcodeColumn(detectionResult, barcodeColumn-offset) {
		codeword = detectionResult.GetDetectionResultColumn(barcodeColumn - offset).Codeword(imageRow)
	}
	if codeword != nil {
		if leftToRight {
			return codeword.EndX()
		}
		return codeword.StartX()
	}
	codeword = detectionResult.GetDetectionResultColumn(barcodeColumn).CodewordNearby(imageRow)
	if codeword != nil {
		if leftToRight {
			return codeword.StartX()
		}
		return codeword.EndX()
	}
	if isValidBarcodeColumn(detectionResult, barcodeColumn-offset) {
		codeword = detectionResult.GetDetectionResultColumn(barcodeColumn - offset).CodewordNearby(imageRow)
	}
	if codeword != nil {
		if leftToRight {
			return codeword.EndX()
		}
		return codeword.StartX()
	}
	skippedColumns := 0
	for isValidBarcodeColumn(detectionResult, barcodeColumn-offset) {
		barcodeColumn -= offset
		for _, previousRowCodeword := range detectionResult.GetDetectionResultColumn(barcodeColumn).Codewords() {
			if previousRowCodeword != nil {
				if leftToRight {
					return previousRowCodeword.EndX() + offset*skippedColumns*(previousRowCodeword.EndX()-previousRowCodeword.StartX())
				}
				return previousRowCodeword.StartX() + offset*skippedColumns*(previousRowCodeword.EndX()-previousRowCodeword.StartX())
			}
		}
		skippedColumns++
	}
	if leftToRight {
		return detectionResult.GetBoundingBox().MinX()
	}
	return detectionResult.GetBoundingBox().MaxX()
}

func detectCodeword(image *bitutil.BitMatrix,
	minColumn, maxColumn int,
	leftToRight bool,
	startColumn, imageRow int,
	minCodewordWidth, maxCodewordWidth int) *Codeword {

	startColumn = adjustCodewordStartColumn(image, minColumn, maxColumn, leftToRight, startColumn, imageRow)
	moduleBitCount := getModuleBitCount(image, minColumn, maxColumn, leftToRight, startColumn, imageRow)
	if moduleBitCount == nil {
		return nil
	}
	var endColumn int
	codewordBitCount := sumInts(moduleBitCount)
	if leftToRight {
		endColumn = startColumn + codewordBitCount
	} else {
		for i := 0; i < len(moduleBitCount)/2; i++ {
			moduleBitCount[i], moduleBitCount[len(moduleBitCount)-1-i] = moduleBitCount[len(moduleBitCount)-1-i], moduleBitCount[i]
		}
		endColumn = startColumn
		startColumn = endColumn - codewordBitCount
	}

	if !checkCodewordSkew(codewordBitCount, minCodewordWidth, maxCodewordWidth) {
		return nil
	}

	decodedValue := GetDecodedValue(moduleBitCount)
	codeword := getCodeword(decodedValue)
	if codeword == -1 {
		return nil
	}
	return NewCodeword(startColumn, endColumn, getCodewordBucketNumber(decodedValue), codeword)
}

func getModuleBitCount(image *bitutil.BitMatrix,
	minColumn, maxColumn int,
	leftToRight bool,
	startColumn, imageRow int) []int {

	imageColumn := startColumn
	moduleBitCount := make([]int, 8)
	moduleNumber := 0
	increment := 1
	if !leftToRight {
		increment = -1
	}
	previousPixelValue := leftToRight
	for ((leftToRight && imageColumn < maxColumn) || (!leftToRight && imageColumn >= minColumn)) && moduleNumber < len(moduleBitCount) {
		if image.Get(imageColumn, imageRow) == previousPixelValue {
			moduleBitCount[moduleNumber]++
			imageColumn += increment
		} else {
			moduleNumber++
			previousPixelValue = !previousPixelValue
		}
	}
	if moduleNumber == len(moduleBitCount) ||
		((imageColumn == maxColumn && leftToRight || imageColumn == minColumn && !leftToRight) &&
			moduleNumber == len(moduleBitCount)-1) {
		return moduleBitCount
	}
	return nil
}

func getNumberOfECCodeWords(barcodeECLevel int) int {
	return 2 << uint(barcodeECLevel)
}

func adjustCodewordStartColumn(image *bitutil.BitMatrix,
	minColumn, maxColumn int,
	leftToRight bool,
	codewordStartColumn, imageRow int) int {

	correctedStartColumn := codewordStartColumn
	increment := -1
	if !leftToRight {
		increment = 1
	}
	for i := 0; i < 2; i++ {
		for (leftToRight && correctedStartColumn >= minColumn || !leftToRight && correctedStartColumn < maxColumn) &&
			leftToRight == image.Get(correctedStartColumn, imageRow) {
			if abs(codewordStartColumn-correctedStartColumn) > codewordSkewSize {
				return codewordStartColumn
			}
			correctedStartColumn += increment
		}
		increment = -increment
		leftToRight = !leftToRight
	}
	return correctedStartColumn
}

func checkCodewordSkew(codewordSize, minCodewordWidth, maxCodewordWidth int) bool {
	return minCodewordWidth-codewordSkewSize <= codewordSize &&
		codewordSize <= maxCodewordWidth+codewordSkewSize
}

func decodeCodewords(codewords []int, ecLevel int, erasures []int) (*internal.DecoderResult, error) {
	if len(codewords) == 0 {
		return nil, zxinggo.ErrFormat
	}

	numECCodewords := 1 << uint(ecLevel+1)
	correctedErrorsCount, err := correctErrors(codewords, erasures, numECCodewords)
	if err != nil {
		return nil, err
	}
	if err := verifyCodewordCount(codewords, numECCodewords); err != nil {
		return nil, err
	}

	decoderResult, err := decodeBitStream(codewords, strconv.Itoa(ecLevel))
	if err != nil {
		return nil, err
	}
	decoderResult.ErrorsCorrected = correctedErrorsCount
	decoderResult.Erasures = len(erasures)
	return decoderResult, nil
}

func correctErrors(codewords []int, erasures []int, numECCodewords int) (int, error) {
	if erasures != nil &&
		len(erasures) > numECCodewords/2+maxErrors ||
		numECCodewords < 0 ||
		numECCodewords > maxECCodewords {
		return 0, zxinggo.ErrChecksum
	}
	return scanErrorCorrection.Decode(codewords, numECCodewords, erasures)
}

func verifyCodewordCount(codewords []int, numECCodewords int) error {
	if len(codewords) < 4 {
		return zxinggo.ErrFormat
	}
	numberOfCodewords := codewords[0]
	if numberOfCodewords > len(codewords) {
		return zxinggo.ErrFormat
	}
	if numberOfCodewords == 0 {
		if numECCodewords < len(codewords) {
			codewords[0] = len(codewords) - numECCodewords
		} else {
			return zxinggo.ErrFormat
		}
	}
	return nil
}

func getBitCountForCodeword(codeword int) []int {
	result := make([]int, 8)
	previousValue := 0
	i := len(result) - 1
	for {
		if (codeword & 0x1) != previousValue {
			previousValue = codeword & 0x1
			i--
			if i < 0 {
				break
			}
		}
		result[i]++
		codeword >>= 1
	}
	return result
}

func getCodewordBucketNumber(codeword int) int {
	return getCodewordBucketNumberFromBitCount(getBitCountForCodeword(codeword))
}

func getCodewordBucketNumberFromBitCount(moduleBitCount []int) int {
	return (moduleBitCount[0] - moduleBitCount[2] + moduleBitCount[4] - moduleBitCount[6] + 9) % 9
}

func abs(x int) int {
	return int(math.Abs(float64(x)))
}
