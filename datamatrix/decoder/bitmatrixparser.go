package decoder

import (
	"fmt"

	"github.com/ericlevine/zxinggo/bitutil"
)

// ReadCodewords reads codewords from a Data Matrix bit matrix using the standard
// ECC-200 module placement algorithm.
//
// The input matrix must have alignment patterns already stripped — it should
// contain only the data region modules (no finder pattern or alignment timing).
// The matrix is re-assembled from data regions into the logical mapping matrix
// before the codeword extraction walk.
func ReadCodewords(matrix *bitutil.BitMatrix) ([]byte, *Version, error) {
	numRows := matrix.Height()
	numColumns := matrix.Width()

	version, err := GetVersionForDimensions(numRows, numColumns)
	if err != nil {
		return nil, nil, err
	}

	// Extract the mapping matrix (strip alignment patterns)
	mappingBitMatrix := extractDataRegion(matrix, version)
	mappingRows := mappingBitMatrix.Height()
	mappingCols := mappingBitMatrix.Width()

	// readMappingMatrix returns the codewords in the correct order
	codewords, err := readMappingMatrix(mappingBitMatrix, mappingRows, mappingCols, version)
	if err != nil {
		return nil, nil, err
	}
	return codewords, version, nil
}

// extractDataRegion removes alignment patterns and finder patterns, leaving
// only the data region modules. Multiple data regions are tiled together into
// the logical mapping matrix.
func extractDataRegion(bitMatrix *bitutil.BitMatrix, version *Version) *bitutil.BitMatrix {
	symbolSizeRows := version.SymbolSizeRows()
	symbolSizeColumns := version.SymbolSizeColumns()
	dataRegionSizeRows := version.DataRegionSizeRows()
	dataRegionSizeColumns := version.DataRegionSizeColumns()

	numDataRegionsRow := symbolSizeRows / (dataRegionSizeRows + 2)
	numDataRegionsColumn := symbolSizeColumns / (dataRegionSizeColumns + 2)

	// The total size of the mapping matrix
	sizeDataRegionRow := numDataRegionsRow * dataRegionSizeRows
	sizeDataRegionColumn := numDataRegionsColumn * dataRegionSizeColumns

	mappingBitMatrix := bitutil.NewBitMatrixWithSize(sizeDataRegionColumn, sizeDataRegionRow)

	for dataRegionRow := 0; dataRegionRow < numDataRegionsRow; dataRegionRow++ {
		dataRegionRowOffset := dataRegionRow * dataRegionSizeRows
		for dataRegionColumn := 0; dataRegionColumn < numDataRegionsColumn; dataRegionColumn++ {
			dataRegionColumnOffset := dataRegionColumn * dataRegionSizeColumns
			for i := 0; i < dataRegionSizeRows; i++ {
				// +1 to skip finder pattern row, +1 for each data region boundary
				readRowOffset := dataRegionRow*(dataRegionSizeRows+2) + 1 + i
				writeRowOffset := dataRegionRowOffset + i
				for j := 0; j < dataRegionSizeColumns; j++ {
					readColumnOffset := dataRegionColumn*(dataRegionSizeColumns+2) + 1 + j
					if bitMatrix.Get(readColumnOffset, readRowOffset) {
						mappingBitMatrix.Set(dataRegionColumnOffset+j, writeRowOffset)
					}
				}
			}
		}
	}

	return mappingBitMatrix
}

// readMappingMatrix walks the mapping matrix in the Data Matrix diagonal pattern
// and extracts codewords.
func readMappingMatrix(mappingBitMatrix *bitutil.BitMatrix, numRows, numColumns int, version *Version) ([]byte, error) {
	totalCodewords := version.TotalCodewords()
	result := make([]byte, totalCodewords)

	// readMapping tracks which modules have been read
	read := make([][]bool, numRows)
	for i := range read {
		read[i] = make([]bool, numColumns)
	}

	codewordIndex := 0
	row := 4
	column := 0

	for {
		// Check the four corner cases first
		if row == numRows && column == 0 {
			if codewordIndex < totalCodewords {
				result[codewordIndex] = readCorner1(mappingBitMatrix, numRows, numColumns, read)
				codewordIndex++
			}
			row -= 2
			column += 2
		}

		if row == numRows-2 && column == 0 && numColumns%4 != 0 {
			if codewordIndex < totalCodewords {
				result[codewordIndex] = readCorner2(mappingBitMatrix, numRows, numColumns, read)
				codewordIndex++
			}
			row -= 2
			column += 2
		}

		if row == numRows+4 && column == 2 && numColumns%8 == 0 {
			if codewordIndex < totalCodewords {
				result[codewordIndex] = readCorner3(mappingBitMatrix, numRows, numColumns, read)
				codewordIndex++
			}
			row -= 2
			column += 2
		}

		if row == numRows-2 && column == 0 && numColumns%8 == 4 {
			if codewordIndex < totalCodewords {
				result[codewordIndex] = readCorner4(mappingBitMatrix, numRows, numColumns, read)
				codewordIndex++
			}
			row -= 2
			column += 2
		}

		// Sweep upward-right (do-while: body runs first, bounds checked after step)
		for {
			if row >= 0 && row < numRows && column >= 0 && column < numColumns && !read[row][column] {
				if codewordIndex < totalCodewords {
					result[codewordIndex] = readUtah(mappingBitMatrix, row, column, numRows, numColumns, read)
					codewordIndex++
				}
			}
			row -= 2
			column += 2
			if !(row >= 0 && column < numColumns) {
				break
			}
		}
		row += 1
		column += 3

		// Sweep downward-left (do-while: body runs first, bounds checked after step)
		for {
			if row >= 0 && row < numRows && column >= 0 && column < numColumns && !read[row][column] {
				if codewordIndex < totalCodewords {
					result[codewordIndex] = readUtah(mappingBitMatrix, row, column, numRows, numColumns, read)
					codewordIndex++
				}
			}
			row += 2
			column -= 2
			if !(row < numRows && column >= 0) {
				break
			}
		}
		row += 3
		column += 1

		if row >= numRows && column >= numColumns {
			break
		}
	}

	if codewordIndex != totalCodewords {
		return nil, fmt.Errorf("datamatrix/decoder: expected %d codewords but got %d", totalCodewords, codewordIndex)
	}
	return result, nil
}

// readModule reads a single module from the mapping matrix, handling wrap-around
// for modules that extend past the edges.
func readModule(mappingBitMatrix *bitutil.BitMatrix, row, column, numRows, numColumns int, read [][]bool) bool {
	// Adjust for negative coordinates (wrap around)
	if row < 0 {
		row += numRows
		column += 4 - ((numRows + 4) % 8)
	}
	if column < 0 {
		column += numColumns
		row += 4 - ((numColumns + 4) % 8)
	}
	if row >= numRows {
		row -= numRows
	}
	if column >= numColumns {
		column -= numColumns
	}
	read[row][column] = true
	return mappingBitMatrix.Get(column, row)
}

// readUtah reads an 8-module "Utah" shaped codeword at the given position.
// The Utah shape is the standard Data Matrix codeword shape.
func readUtah(mappingBitMatrix *bitutil.BitMatrix, row, column, numRows, numColumns int, read [][]bool) byte {
	var currentByte byte

	if readModule(mappingBitMatrix, row-2, column-2, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, row-2, column-1, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, row-1, column-2, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, row-1, column-1, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, row-1, column, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, row, column-2, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, row, column-1, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, row, column, numRows, numColumns, read) {
		currentByte |= 1
	}

	return currentByte
}

// Corner case 1: modules in the four corners of the mapping matrix.
func readCorner1(mappingBitMatrix *bitutil.BitMatrix, numRows, numColumns int, read [][]bool) byte {
	var currentByte byte

	if readModule(mappingBitMatrix, numRows-1, 0, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, numRows-1, 1, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, numRows-1, 2, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 0, numColumns-2, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 0, numColumns-1, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 1, numColumns-1, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 2, numColumns-1, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 3, numColumns-1, numRows, numColumns, read) {
		currentByte |= 1
	}

	return currentByte
}

// Corner case 2
func readCorner2(mappingBitMatrix *bitutil.BitMatrix, numRows, numColumns int, read [][]bool) byte {
	var currentByte byte

	if readModule(mappingBitMatrix, numRows-3, 0, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, numRows-2, 0, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, numRows-1, 0, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 0, numColumns-4, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 0, numColumns-3, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 0, numColumns-2, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 0, numColumns-1, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 1, numColumns-1, numRows, numColumns, read) {
		currentByte |= 1
	}

	return currentByte
}

// Corner case 3
func readCorner3(mappingBitMatrix *bitutil.BitMatrix, numRows, numColumns int, read [][]bool) byte {
	var currentByte byte

	if readModule(mappingBitMatrix, numRows-1, 0, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, numRows-1, numColumns-1, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 0, numColumns-3, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 0, numColumns-2, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 0, numColumns-1, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 1, numColumns-3, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 1, numColumns-2, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 1, numColumns-1, numRows, numColumns, read) {
		currentByte |= 1
	}

	return currentByte
}

// Corner case 4
func readCorner4(mappingBitMatrix *bitutil.BitMatrix, numRows, numColumns int, read [][]bool) byte {
	var currentByte byte

	if readModule(mappingBitMatrix, numRows-3, 0, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, numRows-2, 0, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, numRows-1, 0, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 0, numColumns-2, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 0, numColumns-1, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 1, numColumns-1, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 2, numColumns-1, numRows, numColumns, read) {
		currentByte |= 1
	}
	currentByte <<= 1

	if readModule(mappingBitMatrix, 3, numColumns-1, numRows, numColumns, read) {
		currentByte |= 1
	}

	return currentByte
}
