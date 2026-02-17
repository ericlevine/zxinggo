package bitutil

import "testing"

func TestBitArrayGetSet(t *testing.T) {
	ba := NewBitArray(33)
	for i := 0; i < 33; i++ {
		if ba.Get(i) {
			t.Errorf("bit %d should not be set", i)
		}
	}
	ba.Set(0)
	ba.Set(31)
	ba.Set(32)
	if !ba.Get(0) || !ba.Get(31) || !ba.Get(32) {
		t.Error("bits should be set")
	}
	if ba.Get(1) || ba.Get(30) {
		t.Error("bits should not be set")
	}
}

func TestBitArrayFlip(t *testing.T) {
	ba := NewBitArray(8)
	ba.Flip(3)
	if !ba.Get(3) {
		t.Error("bit 3 should be set after flip")
	}
	ba.Flip(3)
	if ba.Get(3) {
		t.Error("bit 3 should be unset after double flip")
	}
}

func TestBitArrayGetNextSet(t *testing.T) {
	ba := NewBitArray(64)
	ba.Set(10)
	ba.Set(40)
	if got := ba.GetNextSet(0); got != 10 {
		t.Errorf("GetNextSet(0) = %d, want 10", got)
	}
	if got := ba.GetNextSet(10); got != 10 {
		t.Errorf("GetNextSet(10) = %d, want 10", got)
	}
	if got := ba.GetNextSet(11); got != 40 {
		t.Errorf("GetNextSet(11) = %d, want 40", got)
	}
	if got := ba.GetNextSet(41); got != 64 {
		t.Errorf("GetNextSet(41) = %d, want 64", got)
	}
}

func TestBitArrayGetNextUnset(t *testing.T) {
	ba := NewBitArray(8)
	ba.SetRange(0, 8)
	ba.Flip(3) // unset bit 3
	if got := ba.GetNextUnset(0); got != 3 {
		t.Errorf("GetNextUnset(0) = %d, want 3", got)
	}
}

func TestBitArrayAppendBit(t *testing.T) {
	ba := &BitArray{}
	ba.AppendBit(true)
	ba.AppendBit(false)
	ba.AppendBit(true)
	if ba.Size() != 3 {
		t.Errorf("size = %d, want 3", ba.Size())
	}
	if !ba.Get(0) || ba.Get(1) || !ba.Get(2) {
		t.Error("incorrect bits after append")
	}
}

func TestBitArrayAppendBits(t *testing.T) {
	ba := &BitArray{}
	ba.AppendBits(0x1E, 6) // 011110
	if ba.Size() != 6 {
		t.Fatalf("size = %d, want 6", ba.Size())
	}
	expected := []bool{false, true, true, true, true, false}
	for i, exp := range expected {
		if ba.Get(i) != exp {
			t.Errorf("bit %d = %v, want %v", i, ba.Get(i), exp)
		}
	}
}

func TestBitArrayXor(t *testing.T) {
	a := NewBitArray(8)
	b := NewBitArray(8)
	a.Set(0)
	a.Set(2)
	b.Set(1)
	b.Set(2)
	a.Xor(b)
	if !a.Get(0) || !a.Get(1) || a.Get(2) {
		t.Error("XOR result incorrect")
	}
}

func TestBitArrayReverse(t *testing.T) {
	ba := NewBitArray(8)
	ba.Set(0) // bit 0
	ba.Set(2) // bit 2
	ba.Reverse()
	if !ba.Get(5) || !ba.Get(7) {
		t.Error("reversed bits incorrect")
	}
	if ba.Get(0) || ba.Get(2) {
		t.Error("original positions should be unset")
	}
}

func TestBitArrayClone(t *testing.T) {
	ba := NewBitArray(16)
	ba.Set(5)
	clone := ba.Clone()
	clone.Set(10)
	if ba.Get(10) {
		t.Error("modifying clone should not affect original")
	}
	if !clone.Get(5) || !clone.Get(10) {
		t.Error("clone should have both bits set")
	}
}

func TestBitArrayIsRange(t *testing.T) {
	ba := NewBitArray(16)
	ba.SetRange(4, 12)
	if !ba.IsRange(4, 12, true) {
		t.Error("range [4,12) should be all set")
	}
	if !ba.IsRange(0, 4, false) {
		t.Error("range [0,4) should be all unset")
	}
	if ba.IsRange(0, 8, true) {
		t.Error("range [0,8) should not be all set")
	}
}
