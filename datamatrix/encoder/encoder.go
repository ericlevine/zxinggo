// Copyright 2006 Jeremias Maerki in part, and ZXing Authors in part.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Ported from Java ZXing library.

// Package encoder implements Data Matrix (ECC-200) barcode encoding.
package encoder

import (
	"fmt"

	"github.com/ericlevine/zxinggo/bitutil"
)

// Encode encodes the contents string into a Data Matrix ECC-200 symbol,
// returning the resulting BitMatrix. The symbol shape can be constrained
// using EncodeWithShape.
func Encode(contents string) (*bitutil.BitMatrix, error) {
	return EncodeWithShape(contents, ShapeHintForceNone)
}

// EncodeWithShape encodes the contents string into a Data Matrix ECC-200
// symbol with the given shape constraint.
func EncodeWithShape(contents string, shape SymbolShapeHint) (*bitutil.BitMatrix, error) {
	if len(contents) == 0 {
		return nil, fmt.Errorf("datamatrix/encoder: empty contents")
	}

	// Step 1: High-level encode the contents into codewords.
	encoded, err := EncodeHighLevel(contents)
	if err != nil {
		return nil, fmt.Errorf("datamatrix/encoder: high-level encoding failed: %w", err)
	}

	// Step 2: Look up the appropriate symbol size.
	symbolInfo, err := Lookup(len(encoded), shape)
	if err != nil {
		return nil, fmt.Errorf("datamatrix/encoder: symbol lookup failed: %w", err)
	}

	// Step 3: Pad codewords to fill the data capacity.
	codewords := PadCodewords(encoded, symbolInfo.DataCapacity)

	// Step 4: Generate error correction codewords.
	fullCodewords, err := EncodeECC200(codewords, symbolInfo)
	if err != nil {
		return nil, fmt.Errorf("datamatrix/encoder: ECC encoding failed: %w", err)
	}

	// Step 5: Place codewords into the mapping matrix using the placement algorithm.
	mappingRows := symbolInfo.MappingMatrixRows()
	mappingCols := symbolInfo.MappingMatrixColumns()

	placement := NewDefaultPlacement(fullCodewords, mappingCols, mappingRows)
	placement.Place()

	// Step 6: Build the final symbol matrix with finder patterns and timing patterns.
	return encodeLowLevel(placement, symbolInfo), nil
}

// encodeLowLevel builds the final BitMatrix from the placement result.
// It adds the finder pattern (solid L on bottom-left) and the clock track
// pattern (alternating on top-right), and maps data region modules into
// their correct positions accounting for separator rows/columns between
// data regions.
func encodeLowLevel(placement *DefaultPlacement, symbolInfo *SymbolInfo) *bitutil.BitMatrix {
	symbolWidth := symbolInfo.MatrixWidth
	symbolHeight := symbolInfo.MatrixHeight

	matrix := bitutil.NewBitMatrixWithSize(symbolWidth, symbolHeight)

	// Data region dimensions (usable modules, no finder/timing).
	drRows := symbolInfo.DataRegionSizeRows
	drCols := symbolInfo.DataRegionSizeColumns

	// Number of data regions horizontally and vertically.
	numRegionsH := symbolWidth / (drCols + 2) // +2 for left L-bar + right timing column
	numRegionsV := symbolHeight / (drRows + 2)

	// Fill finder patterns and timing patterns for each data region.
	for vRegion := 0; vRegion < numRegionsV; vRegion++ {
		for hRegion := 0; hRegion < numRegionsH; hRegion++ {
			// Origin of this data region in the symbol matrix.
			regionOriginX := hRegion * (drCols + 2)
			regionOriginY := vRegion * (drRows + 2)

			// Draw the solid L-shape: bottom row and left column of each region.
			// Left column (solid bar).
			for y := 0; y < drRows+2; y++ {
				matrix.Set(regionOriginX, regionOriginY+y)
			}
			// Bottom row (solid bar).
			for x := 0; x < drCols+2; x++ {
				matrix.Set(regionOriginX+x, regionOriginY+drRows+1)
			}

			// Draw the clock track: top row and right column of each region.
			// Top row (alternating, starting with unset at origin+1).
			for x := 0; x < drCols+2; x++ {
				if x%2 == 0 {
					matrix.Set(regionOriginX+x, regionOriginY)
				}
			}
			// Right column (alternating, starting with set at the top).
			for y := 0; y < drRows+2; y++ {
				if y%2 == 0 {
					matrix.Set(regionOriginX+drCols+1, regionOriginY+y)
				}
			}
		}
	}

	// Now place the data modules from the mapping matrix into the symbol matrix.
	for vRegion := 0; vRegion < numRegionsV; vRegion++ {
		for hRegion := 0; hRegion < numRegionsH; hRegion++ {
			for r := 0; r < drRows; r++ {
				for c := 0; c < drCols; c++ {
					// Position in the mapping matrix.
					mappingRow := vRegion*drRows + r
					mappingCol := hRegion*drCols + c

					if placement.GetBit(mappingCol, mappingRow) {
						// Position in the symbol matrix.
						// +1 to skip the left L-bar column.
						symbolX := hRegion*(drCols+2) + c + 1
						// +1 to skip the top timing row.
						symbolY := vRegion*(drRows+2) + r + 1
						matrix.Set(symbolX, symbolY)
					}
				}
			}
		}
	}

	return matrix
}
