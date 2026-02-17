package bitutil

import "testing"

func TestBitMatrixGetSet(t *testing.T) {
	bm := NewBitMatrixWithSize(10, 10)
	bm.Set(3, 5)
	if !bm.Get(3, 5) {
		t.Error("bit (3,5) should be set")
	}
	if bm.Get(5, 3) {
		t.Error("bit (5,3) should not be set")
	}
}

func TestBitMatrixFlip(t *testing.T) {
	bm := NewBitMatrixWithSize(4, 4)
	bm.Flip(1, 2)
	if !bm.Get(1, 2) {
		t.Error("bit should be set after flip")
	}
	bm.Flip(1, 2)
	if bm.Get(1, 2) {
		t.Error("bit should be unset after double flip")
	}
}

func TestBitMatrixUnset(t *testing.T) {
	bm := NewBitMatrixWithSize(4, 4)
	bm.Set(2, 3)
	bm.Unset(2, 3)
	if bm.Get(2, 3) {
		t.Error("bit should be unset")
	}
}

func TestBitMatrixSetRegion(t *testing.T) {
	bm := NewBitMatrixWithSize(8, 8)
	bm.SetRegion(2, 2, 4, 4)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			expected := x >= 2 && x < 6 && y >= 2 && y < 6
			if bm.Get(x, y) != expected {
				t.Errorf("(%d,%d) = %v, want %v", x, y, bm.Get(x, y), expected)
			}
		}
	}
}

func TestBitMatrixRow(t *testing.T) {
	bm := NewBitMatrixWithSize(8, 4)
	bm.Set(3, 2)
	bm.Set(5, 2)
	row := bm.Row(2, nil)
	if !row.Get(3) || !row.Get(5) {
		t.Error("row should have bits 3 and 5 set")
	}
	if row.Get(4) {
		t.Error("row bit 4 should not be set")
	}
}

func TestBitMatrixRotate180(t *testing.T) {
	bm := NewBitMatrixWithSize(4, 4)
	bm.Set(0, 0)
	bm.Rotate180()
	if !bm.Get(3, 3) {
		t.Error("(3,3) should be set after 180 rotation")
	}
	if bm.Get(0, 0) {
		t.Error("(0,0) should be unset after 180 rotation")
	}
}

func TestBitMatrixRotate90(t *testing.T) {
	bm := NewBitMatrixWithSize(4, 3)
	bm.Set(3, 0) // top-right
	bm.Rotate90()
	// After 90 CCW: (3,0) -> (0,0) for a 3x4 matrix
	if bm.Width() != 3 || bm.Height() != 4 {
		t.Errorf("dimensions after 90 rotation: %dx%d, want 3x4", bm.Width(), bm.Height())
	}
	if !bm.Get(0, 0) {
		t.Error("(0,0) should be set after 90 rotation")
	}
}

func TestBitMatrixEnclosingRectangle(t *testing.T) {
	bm := NewBitMatrixWithSize(10, 10)
	bm.Set(3, 2)
	bm.Set(7, 8)
	rect := bm.EnclosingRectangle()
	if rect == nil {
		t.Fatal("rect should not be nil")
	}
	if rect[0] != 3 || rect[1] != 2 || rect[2] != 5 || rect[3] != 7 {
		t.Errorf("rect = %v, want [3 2 5 7]", rect)
	}
}

func TestBitMatrixTopLeftOnBit(t *testing.T) {
	bm := NewBitMatrixWithSize(10, 10)
	bm.Set(5, 3)
	pt := bm.TopLeftOnBit()
	if pt == nil || pt[0] != 5 || pt[1] != 3 {
		t.Errorf("TopLeftOnBit = %v, want [5 3]", pt)
	}
}

func TestBitMatrixClone(t *testing.T) {
	bm := NewBitMatrixWithSize(8, 8)
	bm.Set(1, 1)
	clone := bm.Clone()
	clone.Set(2, 2)
	if bm.Get(2, 2) {
		t.Error("modifying clone should not affect original")
	}
}

func TestBitMatrixEquals(t *testing.T) {
	a := NewBitMatrixWithSize(4, 4)
	b := NewBitMatrixWithSize(4, 4)
	a.Set(1, 2)
	b.Set(1, 2)
	if !a.Equals(b) {
		t.Error("equal matrices should be equal")
	}
	b.Set(3, 3)
	if a.Equals(b) {
		t.Error("different matrices should not be equal")
	}
}
