// Package decoder implements Data Matrix (ECC-200) barcode decoding.
package decoder

import "fmt"

// ECB represents a single error-correction block specification within a version.
type ECB struct {
	Count         int
	DataCodewords int
}

// ECBlocks describes the error-correction block layout for a Data Matrix symbol.
type ECBlocks struct {
	ECCodewords int // total EC codewords across all blocks
	Blocks      []ECB
}

// Version describes a Data Matrix ECC-200 symbol size and its EC block layout.
type Version struct {
	versionNumber        int
	symbolSizeRows       int
	symbolSizeColumns    int
	dataRegionSizeRows   int
	dataRegionSizeColumns int
	ecBlocks             ECBlocks
	totalCodewords       int
}

// VersionNumber returns the version number (1-based index).
func (v *Version) VersionNumber() int { return v.versionNumber }

// SymbolSizeRows returns the total number of rows in the symbol (including finder).
func (v *Version) SymbolSizeRows() int { return v.symbolSizeRows }

// SymbolSizeColumns returns the total number of columns in the symbol (including finder).
func (v *Version) SymbolSizeColumns() int { return v.symbolSizeColumns }

// DataRegionSizeRows returns the number of data rows in each data region.
func (v *Version) DataRegionSizeRows() int { return v.dataRegionSizeRows }

// DataRegionSizeColumns returns the number of data columns in each data region.
func (v *Version) DataRegionSizeColumns() int { return v.dataRegionSizeColumns }

// TotalCodewords returns the total number of data + EC codewords.
func (v *Version) TotalCodewords() int { return v.totalCodewords }

// GetECBlocks returns the error-correction block layout.
func (v *Version) GetECBlocks() ECBlocks { return v.ecBlocks }

func newVersion(versionNumber, symbolSizeRows, symbolSizeColumns, dataRegionSizeRows, dataRegionSizeColumns, totalECCodewords int, blocks ...ECB) Version {
	total := 0
	for _, block := range blocks {
		total += block.Count * (block.DataCodewords + totalECCodewords/countBlocks(blocks))
	}
	return Version{
		versionNumber:         versionNumber,
		symbolSizeRows:        symbolSizeRows,
		symbolSizeColumns:     symbolSizeColumns,
		dataRegionSizeRows:    dataRegionSizeRows,
		dataRegionSizeColumns: dataRegionSizeColumns,
		ecBlocks: ECBlocks{
			ECCodewords: totalECCodewords,
			Blocks:      blocks,
		},
		totalCodewords: total,
	}
}

func countBlocks(blocks []ECB) int {
	total := 0
	for _, b := range blocks {
		total += b.Count
	}
	if total == 0 {
		return 1
	}
	return total
}

// GetVersionForDimensions returns the Version for a Data Matrix symbol of the
// given row and column count.
func GetVersionForDimensions(numRows, numColumns int) (*Version, error) {
	for i := range versions {
		if versions[i].symbolSizeRows == numRows && versions[i].symbolSizeColumns == numColumns {
			return &versions[i], nil
		}
	}
	return nil, fmt.Errorf("datamatrix/decoder: no version for dimensions %dx%d", numRows, numColumns)
}

// Complete ECC-200 symbol table — 24 square sizes + 6 rectangular sizes = 30 entries.
// Data from ISO/IEC 16022 Table 7 (ECC 200 symbol attributes).
//
// Fields per entry: versionNumber, symbolSizeRows, symbolSizeColumns,
//   dataRegionSizeRows, dataRegionSizeColumns, totalECCodewords, ECB{count, dataCodewords}...
var versions = [30]Version{
	// Square symbols
	newVersion(1, 10, 10, 8, 8, 5, ECB{1, 3}),
	newVersion(2, 12, 12, 10, 10, 7, ECB{1, 5}),
	newVersion(3, 14, 14, 12, 12, 10, ECB{1, 8}),
	newVersion(4, 16, 16, 14, 14, 12, ECB{1, 12}),
	newVersion(5, 18, 18, 16, 16, 14, ECB{1, 18}),
	newVersion(6, 20, 20, 18, 18, 18, ECB{1, 22}),
	newVersion(7, 22, 22, 20, 20, 20, ECB{1, 30}),
	newVersion(8, 24, 24, 22, 22, 24, ECB{1, 36}),
	newVersion(9, 26, 26, 24, 24, 28, ECB{1, 44}),
	newVersion(10, 32, 32, 14, 14, 36, ECB{1, 62}),
	newVersion(11, 36, 36, 16, 16, 42, ECB{1, 86}),
	newVersion(12, 40, 40, 18, 18, 48, ECB{1, 114}),
	newVersion(13, 44, 44, 20, 20, 56, ECB{1, 144}),
	newVersion(14, 48, 48, 22, 22, 68, ECB{1, 174}),
	newVersion(15, 52, 52, 24, 24, 42, ECB{2, 102}),
	newVersion(16, 64, 64, 14, 14, 56, ECB{2, 140}),
	newVersion(17, 72, 72, 16, 16, 36, ECB{4, 92}),
	newVersion(18, 80, 80, 18, 18, 48, ECB{4, 114}),
	newVersion(19, 88, 88, 20, 20, 56, ECB{4, 144}),
	newVersion(20, 96, 96, 22, 22, 68, ECB{4, 174}),
	newVersion(21, 104, 104, 24, 24, 56, ECB{6, 136}),
	newVersion(22, 120, 120, 18, 18, 68, ECB{6, 175}),
	newVersion(23, 132, 132, 20, 20, 62, ECB{8, 163}),
	newVersion(24, 144, 144, 22, 22, 62, ECB{8, 156}, ECB{2, 155}),

	// Rectangular symbols
	newVersion(25, 8, 18, 6, 16, 7, ECB{1, 5}),
	newVersion(26, 8, 32, 6, 14, 11, ECB{1, 10}),
	newVersion(27, 12, 26, 10, 24, 14, ECB{1, 16}),
	newVersion(28, 12, 36, 10, 16, 18, ECB{1, 22}),
	newVersion(29, 16, 36, 14, 16, 24, ECB{1, 32}),
	newVersion(30, 16, 48, 14, 22, 28, ECB{1, 49}),
}
