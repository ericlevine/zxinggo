package detector

import (
	"math"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

var (
	indexesStartPattern = [4]int{0, 4, 1, 5}
	indexesStopPattern  = [4]int{6, 2, 7, 3}
)

const (
	maxAvgVariance               = 0.42
	maxIndividualVariance        = 0.8
	maxStopPatternHeightVariance = 0.5
	maxPixelDrift                = 3
	maxPatternDrift              = 5
	skippedRowCountMax           = 25
	rowStep                      = 5
	barcodeMinHeight             = 10
)

// B S B S B S B S Bar/Space pattern
// 11111111 0 1 0 1 0 1 000
var startPattern = [8]int{8, 1, 1, 1, 1, 1, 1, 3}

// 1111111 0 1 000 1 0 1 00 1
var stopPattern = [9]int{7, 1, 1, 3, 1, 1, 1, 2, 1}

var rotations = [4]int{0, 180, 270, 90}

// Detect detects a PDF417 code in an image. It checks 0, 90, 180, and 270
// degree rotations. If multiple is true, the image is searched for multiple
// codes; otherwise at most one code will be found and returned.
func Detect(matrix *bitutil.BitMatrix, multiple bool, tryHarder bool) (*PDF417DetectorResult, error) {
	for _, rotation := range rotations {
		bitMatrix := applyRotation(matrix, rotation)
		barcodeCoordinates := detect(multiple, bitMatrix, tryHarder)
		if len(barcodeCoordinates) > 0 {
			return &PDF417DetectorResult{
				Bits:     bitMatrix,
				Points:   barcodeCoordinates,
				Rotation: rotation,
			}, nil
		}
	}

	return &PDF417DetectorResult{
		Bits:     matrix,
		Points:   nil,
		Rotation: 0,
	}, nil
}

// applyRotation applies a rotation to the supplied BitMatrix.
func applyRotation(matrix *bitutil.BitMatrix, rotation int) *bitutil.BitMatrix {
	if rotation%360 == 0 {
		return matrix
	}
	newMatrix := matrix.Clone()
	newMatrix.Rotate(rotation)
	return newMatrix
}

// detect detects PDF417 codes in an image. Only checks 0 degree rotation.
func detect(multiple bool, bitMatrix *bitutil.BitMatrix, tryHarder bool) [][]*zxinggo.ResultPoint {
	var barcodeCoordinates [][]*zxinggo.ResultPoint
	row := 0
	column := 0
	foundBarcodeInRow := false

	for row < bitMatrix.Height() {
		vertices := findVertices(bitMatrix, row, column, tryHarder)

		if vertices[0] == nil && vertices[3] == nil {
			if !foundBarcodeInRow {
				if !tryHarder {
					// we didn't find any barcode so that's the end of searching
					break
				}
				row += rowStep
				continue
			}
			// we didn't find a barcode starting at the given column and row.
			// Try again from the first column and slightly below the lowest
			// barcode we found so far.
			foundBarcodeInRow = false
			column = 0
			for _, barcodeCoordinate := range barcodeCoordinates {
				if barcodeCoordinate[1] != nil {
					row = int(math.Max(float64(row), barcodeCoordinate[1].Y))
				}
				if barcodeCoordinate[3] != nil {
					row = maxInt(row, int(barcodeCoordinate[3].Y))
				}
			}
			row += rowStep
			continue
		}
		foundBarcodeInRow = true
		barcodeCoordinates = append(barcodeCoordinates, vertices)
		if !multiple && !tryHarder {
			break
		}
		// if we didn't find a right row indicator column, then continue the
		// search for the next barcode after the start pattern of the barcode
		// just found.
		if vertices[2] != nil {
			column = int(vertices[2].X)
			row = int(vertices[2].Y)
		} else {
			column = int(vertices[4].X)
			row = int(vertices[4].Y)
		}
	}

	return barcodeCoordinates
}

// findVertices locates the vertices and the codewords area of a black blob
// using the Start and Stop patterns as locators.
//
// Returns an 8-element slice:
//
//	[0] x, y top left barcode
//	[1] x, y bottom left barcode
//	[2] x, y top right barcode
//	[3] x, y bottom right barcode
//	[4] x, y top left codeword area
//	[5] x, y bottom left codeword area
//	[6] x, y top right codeword area
//	[7] x, y bottom right codeword area
func findVertices(matrix *bitutil.BitMatrix, startRow, startColumn int, tryHarder bool) []*zxinggo.ResultPoint {
	height := matrix.Height()
	width := matrix.Width()

	result := make([]*zxinggo.ResultPoint, 8)
	minHeight := barcodeMinHeight

	copyToResult(result,
		findRowsWithPattern(matrix, height, width, startRow, startColumn, minHeight, startPattern[:], tryHarder),
		indexesStartPattern[:])

	if result[4] != nil {
		startColumn = int(result[4].X)
		startRow = int(result[4].Y)
		if result[5] != nil {
			endRow := int(result[5].Y)
			startPatternHeight := endRow - startRow
			minHeight = maxInt(int(float64(startPatternHeight)*maxStopPatternHeightVariance), barcodeMinHeight)
		}
	}

	copyToResult(result,
		findRowsWithPattern(matrix, height, width, startRow, startColumn, minHeight, stopPattern[:], tryHarder),
		indexesStopPattern[:])

	return result
}

// copyToResult copies elements from tmpResult into result at the specified
// destination indexes.
func copyToResult(result, tmpResult []*zxinggo.ResultPoint, destinationIndexes []int) {
	for i, idx := range destinationIndexes {
		result[idx] = tmpResult[i]
	}
}

// findRowsWithPattern finds the top and bottom rows where a guard pattern
// occurs, returning a 4-element slice of result points.
func findRowsWithPattern(matrix *bitutil.BitMatrix,
	height, width, startRow, startColumn, minHeight int,
	pattern []int, tryHarder bool) []*zxinggo.ResultPoint {

	result := make([]*zxinggo.ResultPoint, 4)
	found := false
	counters := make([]int, len(pattern))

	for ; startRow < height; startRow += rowStep {
		loc := findGuardPattern(matrix, startColumn, startRow, width, pattern, counters)
		if loc != nil {
			for startRow > 0 {
				startRow--
				previousRowLoc := findGuardPattern(matrix, startColumn, startRow, width, pattern, counters)
				if previousRowLoc != nil {
					loc = previousRowLoc
				} else {
					startRow++
					break
				}
			}
			result[0] = &zxinggo.ResultPoint{X: float64(loc[0]), Y: float64(startRow)}
			result[1] = &zxinggo.ResultPoint{X: float64(loc[1]), Y: float64(startRow)}
			found = true
			break
		}
	}

	stopRow := startRow + 1
	// Last row of the current symbol that contains pattern
	if found {
		skippedRowCount := 0
		previousRowLoc := [2]int{int(result[0].X), int(result[1].X)}
		for ; stopRow < height; stopRow++ {
			loc := findGuardPattern(matrix, previousRowLoc[0], stopRow, width, pattern, counters)
			// a found pattern is only considered to belong to the same barcode
			// if the start and end positions don't differ too much. Pattern
			// drift should be not bigger than two for consecutive rows. With a
			// higher number of skipped rows drift could be larger. To keep it
			// simple for now, we allow a slightly larger drift and don't check
			// for skipped rows.
			if loc != nil &&
				abs(previousRowLoc[0]-loc[0]) < maxPatternDrift &&
				abs(previousRowLoc[1]-loc[1]) < maxPatternDrift {
				previousRowLoc = [2]int{loc[0], loc[1]}
				skippedRowCount = 0
			} else {
				if skippedRowCount > skippedRowCountMax {
					break
				}
				skippedRowCount++
			}
		}
		stopRow -= skippedRowCount + 1
		result[2] = &zxinggo.ResultPoint{X: float64(previousRowLoc[0]), Y: float64(stopRow)}
		result[3] = &zxinggo.ResultPoint{X: float64(previousRowLoc[1]), Y: float64(stopRow)}
	}

	if stopRow-startRow < minHeight {
		if tryHarder && found {
			// The match was too short — likely a false positive from noise.
			// Resume searching from beyond the rejected match.
			for i := range result {
				result[i] = nil
			}
			return findRowsWithPattern(matrix, height, width, stopRow+1+rowStep, startColumn, minHeight, pattern, tryHarder)
		}
		for i := range result {
			result[i] = nil
		}
	}

	return result
}

// findGuardPattern searches a row for a guard pattern and returns the
// start/end horizontal offset as a two-element slice, or nil if not found.
func findGuardPattern(matrix *bitutil.BitMatrix,
	column, row, width int,
	pattern []int,
	counters []int) []int {

	for i := range counters {
		counters[i] = 0
	}
	patternStart := column
	pixelDrift := 0

	// if there are black pixels left of the current pixel shift to the left,
	// but only for maxPixelDrift pixels
	for patternStart > 0 && pixelDrift < maxPixelDrift && matrix.Get(patternStart, row) {
		patternStart--
		pixelDrift++
	}

	x := patternStart
	counterPosition := 0
	patternLength := len(pattern)
	isWhite := false

	for ; x < width; x++ {
		pixel := matrix.Get(x, row)
		if pixel != isWhite {
			counters[counterPosition]++
		} else {
			if counterPosition == patternLength-1 {
				if patternMatchVariance(counters, pattern) < maxAvgVariance {
					return []int{patternStart, x}
				}
				patternStart += counters[0] + counters[1]
				copy(counters, counters[2:counterPosition+1])
				counters[counterPosition-1] = 0
				counters[counterPosition] = 0
				counterPosition--
			} else {
				counterPosition++
			}
			counters[counterPosition] = 1
			isWhite = !isWhite
		}
	}

	if counterPosition == patternLength-1 &&
		patternMatchVariance(counters, pattern) < maxAvgVariance {
		return []int{patternStart, x - 1}
	}

	return nil
}

// patternMatchVariance determines how closely a set of observed counts of runs
// of black/white values matches a given target pattern. This is reported as
// the ratio of the total variance from the expected pattern proportions across
// all pattern elements, to the length of the pattern.
func patternMatchVariance(counters, pattern []int) float64 {
	numCounters := len(counters)
	total := 0
	patternLength := 0
	for i := 0; i < numCounters; i++ {
		total += counters[i]
		patternLength += pattern[i]
	}
	if total < patternLength {
		// If we don't even have one pixel per unit of bar width, assume this
		// is too small to reliably match, so fail.
		return math.Inf(1)
	}

	unitBarWidth := float64(total) / float64(patternLength)
	maxIndVar := maxIndividualVariance * unitBarWidth

	totalVariance := 0.0
	for x := 0; x < numCounters; x++ {
		counter := float64(counters[x])
		scaledPattern := float64(pattern[x]) * unitBarWidth
		variance := counter - scaledPattern
		if variance < 0 {
			variance = -variance
		}
		if variance > maxIndVar {
			return math.Inf(1)
		}
		totalVariance += variance
	}

	return totalVariance / float64(total)
}

// abs returns the absolute value of an int.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// maxInt returns the larger of two ints.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
