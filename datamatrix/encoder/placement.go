// Copyright 2006 Jeremias Maerki in part, and ZXing Authors in part.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Ported from Java ZXing library.

package encoder

// DefaultPlacement implements the ECC-200 module placement algorithm
// as defined in ISO/IEC 16022, Annex F (and Annex M for the special
// corner cases). It assigns each codeword bit to a position in the
// mapping matrix.
//
// The mapping matrix is the symbol matrix with finder/timing patterns
// stripped away; it contains only data modules.
type DefaultPlacement struct {
	codewords []byte
	numRows   int
	numCols   int
	bits      []int8 // -1 = unvisited, 0 = off, 1 = on
}

// NewDefaultPlacement creates a placement object for the given codewords
// and mapping matrix dimensions (rows and columns of the data area only,
// excluding finder patterns).
func NewDefaultPlacement(codewords []byte, numCols, numRows int) *DefaultPlacement {
	p := &DefaultPlacement{
		codewords: codewords,
		numRows:   numRows,
		numCols:   numCols,
		bits:      make([]int8, numRows*numCols),
	}
	for i := range p.bits {
		p.bits[i] = -1 // mark all as unvisited
	}
	return p
}

// NumRows returns the number of rows.
func (p *DefaultPlacement) NumRows() int { return p.numRows }

// NumCols returns the number of columns.
func (p *DefaultPlacement) NumCols() int { return p.numCols }

// GetBit returns the bit value at (col, row). Returns false if unset.
func (p *DefaultPlacement) GetBit(col, row int) bool {
	return p.bits[row*p.numCols+col] == 1
}

// setBit sets the bit at (col, row).
func (p *DefaultPlacement) setBit(col, row int, bit bool) {
	if bit {
		p.bits[row*p.numCols+col] = 1
	} else {
		p.bits[row*p.numCols+col] = 0
	}
}

// hasBit returns true if the position has been visited.
func (p *DefaultPlacement) hasBit(col, row int) bool {
	return p.bits[row*p.numCols+col] >= 0
}

// Place runs the placement algorithm, filling the mapping matrix
// with codeword bits.
func (p *DefaultPlacement) Place() {
	pos := 0 // current codeword index
	row := 4
	col := 0

	for {
		// Check the four corner conditions.
		if row == p.numRows && col == 0 {
			p.corner1(pos)
			pos++
		}
		if row == p.numRows-2 && col == 0 && p.numCols%4 != 0 {
			p.corner2(pos)
			pos++
		}
		if row == p.numRows-2 && col == 0 && p.numCols%8 == 4 {
			p.corner3(pos)
			pos++
		}
		if row == p.numRows+4 && col == 2 && p.numCols%8 == 0 {
			p.corner4(pos)
			pos++
		}

		// Sweep upward-right diagonal.
		for {
			if row < p.numRows && col >= 0 && !p.hasBit(col, row) {
				p.utah(row, col, pos)
				pos++
			}
			row -= 2
			col += 2
			if row < 0 || col >= p.numCols {
				break
			}
		}
		row++
		col += 3

		// Sweep downward-left diagonal.
		for {
			if row >= 0 && col < p.numCols && !p.hasBit(col, row) {
				p.utah(row, col, pos)
				pos++
			}
			row += 2
			col -= 2
			if row >= p.numRows || col < 0 {
				break
			}
		}
		row += 3
		col++

		if row >= p.numRows && col >= p.numCols {
			break
		}
	}

	// Fill any remaining unvisited modules with 0 (padding).
	if !p.hasBit(p.numCols-1, p.numRows-1) {
		p.setBit(p.numCols-1, p.numRows-1, true)
		p.setBit(p.numCols-2, p.numRows-2, true)
	}
}

// module places a single module, handling wrapping for positions outside
// the matrix boundaries.
func (p *DefaultPlacement) module(row, col, pos, bit int) {
	if row < 0 {
		row += p.numRows
		col += 4 - ((p.numRows + 4) % 8)
	}
	if col < 0 {
		col += p.numCols
		row += 4 - ((p.numCols + 4) % 8)
	}

	// Ensure we are in bounds after wrapping.
	if row >= p.numRows {
		row -= p.numRows
	}
	if col >= p.numCols {
		col -= p.numCols
	}

	v := false
	if pos < len(p.codewords) {
		v = (p.codewords[pos] & (1 << uint(8-bit-1))) != 0
	}
	p.setBit(col, row, v)
}

// utah places the 8 modules of a standard Utah-shaped codeword.
// The (row, col) parameters refer to the position of the lower-right
// corner of the nominal L-shaped pattern.
func (p *DefaultPlacement) utah(row, col, pos int) {
	p.module(row-2, col-2, pos, 0)
	p.module(row-2, col-1, pos, 1)
	p.module(row-1, col-2, pos, 2)
	p.module(row-1, col-1, pos, 3)
	p.module(row-1, col, pos, 4)
	p.module(row, col-2, pos, 5)
	p.module(row, col-1, pos, 6)
	p.module(row, col, pos, 7)
}

// corner1 handles special case 1: the bottom-left corner codeword
// when row==numRows and col==0.
func (p *DefaultPlacement) corner1(pos int) {
	p.module(p.numRows-1, 0, pos, 0)
	p.module(p.numRows-1, 1, pos, 1)
	p.module(p.numRows-1, 2, pos, 2)
	p.module(0, p.numCols-2, pos, 3)
	p.module(0, p.numCols-1, pos, 4)
	p.module(1, p.numCols-1, pos, 5)
	p.module(2, p.numCols-1, pos, 6)
	p.module(3, p.numCols-1, pos, 7)
}

// corner2 handles special case 2.
func (p *DefaultPlacement) corner2(pos int) {
	p.module(p.numRows-3, 0, pos, 0)
	p.module(p.numRows-2, 0, pos, 1)
	p.module(p.numRows-1, 0, pos, 2)
	p.module(0, p.numCols-4, pos, 3)
	p.module(0, p.numCols-3, pos, 4)
	p.module(0, p.numCols-2, pos, 5)
	p.module(0, p.numCols-1, pos, 6)
	p.module(1, p.numCols-1, pos, 7)
}

// corner3 handles special case 3.
func (p *DefaultPlacement) corner3(pos int) {
	p.module(p.numRows-3, 0, pos, 0)
	p.module(p.numRows-2, 0, pos, 1)
	p.module(p.numRows-1, 0, pos, 2)
	p.module(0, p.numCols-2, pos, 3)
	p.module(0, p.numCols-1, pos, 4)
	p.module(1, p.numCols-1, pos, 5)
	p.module(2, p.numCols-1, pos, 6)
	p.module(3, p.numCols-1, pos, 7)
}

// corner4 handles special case 4.
func (p *DefaultPlacement) corner4(pos int) {
	p.module(p.numRows-1, 0, pos, 0)
	p.module(p.numRows-1, p.numCols-1, pos, 1)
	p.module(0, p.numCols-3, pos, 2)
	p.module(0, p.numCols-2, pos, 3)
	p.module(0, p.numCols-1, pos, 4)
	p.module(1, p.numCols-3, pos, 5)
	p.module(1, p.numCols-2, pos, 6)
	p.module(1, p.numCols-1, pos, 7)
}
