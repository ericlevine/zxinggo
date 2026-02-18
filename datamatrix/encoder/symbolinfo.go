// Copyright 2006 Jeremias Maerki in part, and ZXing Authors in part.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Ported from Java ZXing library.

package encoder

import (
	"errors"
	"fmt"
)

// SymbolShapeHint controls whether the encoder prefers square or rectangular symbols.
type SymbolShapeHint int

const (
	// ShapeHintForceNone allows either square or rectangular symbols.
	ShapeHintForceNone SymbolShapeHint = iota
	// ShapeHintForceSquare forces the encoder to choose a square symbol.
	ShapeHintForceSquare
	// ShapeHintForceRectangle forces the encoder to choose a rectangular symbol.
	ShapeHintForceRectangle
)

// SymbolInfo describes a single Data Matrix ECC-200 symbol size.
type SymbolInfo struct {
	Rectangular           bool
	DataCapacity          int // number of data codewords (sum across all interleaved blocks)
	ErrorCodewords        int // total number of EC codewords
	MatrixWidth           int // symbol width in modules (including finder patterns)
	MatrixHeight          int // symbol height in modules (including finder patterns)
	DataRegionSizeRows    int // number of data rows per data region
	DataRegionSizeColumns int // number of data columns per data region
	RSBlockData           int // data codewords per RS block (first block if two sizes)
	RSBlockError          int // EC codewords per RS block
	// For symbols with two differently-sized RS blocks (version 24: 144x144)
	RSBlockData2 int // data codewords per second-type block (0 if uniform)
	NumRSBlocks2 int // number of second-type blocks (0 if uniform)
}

// InterleavedBlockCount returns the total number of interleaved RS blocks.
func (si *SymbolInfo) InterleavedBlockCount() int {
	n := si.dataCodewordsPerBlock1Count()
	if si.RSBlockData2 > 0 {
		n += si.NumRSBlocks2
	}
	return n
}

// dataCodewordsPerBlock1Count returns the count of first-type RS blocks.
func (si *SymbolInfo) dataCodewordsPerBlock1Count() int {
	// For uniform blocks, the count is DataCapacity / RSBlockData
	if si.RSBlockData2 == 0 {
		return si.DataCapacity / si.RSBlockData
	}
	// For two-size blocks: total = count1*RSBlockData + NumRSBlocks2*RSBlockData2
	return (si.DataCapacity - si.NumRSBlocks2*si.RSBlockData2) / si.RSBlockData
}

// SymbolDataCapacity returns the data capacity after accounting for EC codewords.
func (si *SymbolInfo) SymbolDataCapacity() int {
	return si.DataCapacity
}

// TotalCodewords returns data + error correction codewords.
func (si *SymbolInfo) TotalCodewords() int {
	return si.DataCapacity + si.ErrorCodewords
}

// MappingMatrixRows returns the number of rows in the mapping matrix
// (symbol rows minus finder pattern rows: each data region has 2 extra rows).
func (si *SymbolInfo) MappingMatrixRows() int {
	return si.MatrixHeight - (si.MatrixHeight / (si.DataRegionSizeRows + 2)) * 2
}

// MappingMatrixColumns returns the number of columns in the mapping matrix.
func (si *SymbolInfo) MappingMatrixColumns() int {
	return si.MatrixWidth - (si.MatrixWidth / (si.DataRegionSizeColumns + 2)) * 2
}

// symbols is the full list of ECC-200 symbol sizes ordered by data capacity.
// Derived from ISO/IEC 16022 Table 7.
var symbols = []SymbolInfo{
	// Square symbols
	// {Rectangular, DataCapacity, ErrorCodewords, MatrixWidth, MatrixHeight,
	//  DataRegionSizeRows, DataRegionSizeColumns, RSBlockData, RSBlockError, RSBlockData2, NumRSBlocks2}
	{false, 3, 5, 10, 10, 8, 8, 3, 5, 0, 0},
	{false, 5, 7, 12, 12, 10, 10, 5, 7, 0, 0},
	{false, 8, 10, 14, 14, 12, 12, 8, 10, 0, 0},
	{false, 12, 12, 16, 16, 14, 14, 12, 12, 0, 0},
	{false, 18, 14, 18, 18, 16, 16, 18, 14, 0, 0},
	{false, 22, 18, 20, 20, 18, 18, 22, 18, 0, 0},
	{false, 30, 20, 22, 22, 20, 20, 30, 20, 0, 0},
	{false, 36, 24, 24, 24, 22, 22, 36, 24, 0, 0},
	{false, 44, 28, 26, 26, 24, 24, 44, 28, 0, 0},
	{false, 62, 36, 32, 32, 14, 14, 62, 36, 0, 0},
	{false, 86, 42, 36, 36, 16, 16, 86, 42, 0, 0},
	{false, 114, 48, 40, 40, 18, 18, 114, 48, 0, 0},
	{false, 144, 56, 44, 44, 20, 20, 144, 56, 0, 0},
	{false, 174, 68, 48, 48, 22, 22, 174, 68, 0, 0},
	{false, 204, 84, 52, 52, 24, 24, 102, 42, 0, 0},
	{false, 280, 112, 64, 64, 14, 14, 140, 56, 0, 0},
	{false, 368, 144, 72, 72, 16, 16, 92, 36, 0, 0},
	{false, 456, 192, 80, 80, 18, 18, 114, 48, 0, 0},
	{false, 576, 224, 88, 88, 20, 20, 144, 56, 0, 0},
	{false, 696, 272, 96, 96, 22, 22, 174, 68, 0, 0},
	{false, 816, 336, 104, 104, 24, 24, 136, 56, 0, 0},
	{false, 1050, 408, 120, 120, 18, 18, 175, 68, 0, 0},
	{false, 1304, 496, 132, 132, 20, 20, 163, 62, 0, 0},
	{false, 1558, 620, 144, 144, 22, 22, 156, 62, 155, 2},

	// Rectangular symbols
	{true, 5, 7, 18, 8, 6, 16, 5, 7, 0, 0},
	{true, 10, 11, 32, 8, 6, 14, 10, 11, 0, 0},
	{true, 16, 14, 26, 12, 10, 24, 16, 14, 0, 0},
	{true, 22, 18, 36, 12, 10, 16, 22, 18, 0, 0},
	{true, 32, 24, 36, 16, 14, 16, 32, 24, 0, 0},
	{true, 49, 28, 48, 16, 14, 22, 49, 28, 0, 0},
}

// Lookup finds the smallest symbol that can hold the given number of data codewords.
// shapeHint can be used to restrict the search to square or rectangular symbols.
func Lookup(dataCodewords int, shapeHint SymbolShapeHint) (*SymbolInfo, error) {
	for i := range symbols {
		si := &symbols[i]
		if shapeHint == ShapeHintForceSquare && si.Rectangular {
			continue
		}
		if shapeHint == ShapeHintForceRectangle && !si.Rectangular {
			continue
		}
		if si.DataCapacity >= dataCodewords {
			return si, nil
		}
	}
	return nil, fmt.Errorf("datamatrix/encoder: no symbol found for %d data codewords", dataCodewords)
}

// LookupBySize returns the SymbolInfo for a specific symbol matrix size.
func LookupBySize(matrixWidth, matrixHeight int) (*SymbolInfo, error) {
	for i := range symbols {
		si := &symbols[i]
		if si.MatrixWidth == matrixWidth && si.MatrixHeight == matrixHeight {
			return si, nil
		}
	}
	return nil, errors.New("datamatrix/encoder: no symbol found for the given size")
}
