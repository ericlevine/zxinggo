package reedsolomon

import "testing"

func TestEncodeDecodeQR(t *testing.T) {
	// Test with QR code field
	field := QRCodeField256

	// Create test data (10 data codewords + 7 EC codewords)
	dataSize := 10
	ecSize := 7
	toEncode := make([]int, dataSize+ecSize)
	for i := 0; i < dataSize; i++ {
		toEncode[i] = i + 1
	}

	// Encode
	enc := NewEncoder(field)
	enc.Encode(toEncode, ecSize)

	// Verify data is still intact
	for i := 0; i < dataSize; i++ {
		if toEncode[i] != i+1 {
			t.Errorf("data[%d] = %d, want %d", i, toEncode[i], i+1)
		}
	}

	// Introduce errors
	received := make([]int, len(toEncode))
	copy(received, toEncode)
	received[0] = 0    // corrupt first byte
	received[3] = 200  // corrupt another byte
	received[6] = 100  // corrupt another byte

	// Decode (should correct up to ecSize/2 = 3 errors)
	dec := NewDecoder(field)
	corrected, err := dec.Decode(received, ecSize)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if corrected != 3 {
		t.Errorf("corrected = %d, want 3", corrected)
	}

	// Verify correction
	for i := 0; i < dataSize; i++ {
		if received[i] != toEncode[i] {
			t.Errorf("after correction, data[%d] = %d, want %d", i, received[i], toEncode[i])
		}
	}
}

func TestDecodeNoErrors(t *testing.T) {
	field := QRCodeField256
	dataSize := 5
	ecSize := 4
	toEncode := make([]int, dataSize+ecSize)
	for i := 0; i < dataSize; i++ {
		toEncode[i] = (i + 1) * 10
	}

	enc := NewEncoder(field)
	enc.Encode(toEncode, ecSize)

	dec := NewDecoder(field)
	corrected, err := dec.Decode(toEncode, ecSize)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if corrected != 0 {
		t.Errorf("corrected = %d, want 0 (no errors)", corrected)
	}
}

func TestDecodeTooManyErrors(t *testing.T) {
	field := QRCodeField256
	dataSize := 5
	ecSize := 4
	toEncode := make([]int, dataSize+ecSize)
	for i := 0; i < dataSize; i++ {
		toEncode[i] = (i + 1) * 10
	}

	enc := NewEncoder(field)
	enc.Encode(toEncode, ecSize)

	// Introduce more errors than can be corrected
	received := make([]int, len(toEncode))
	copy(received, toEncode)
	received[0] = 0
	received[1] = 0
	received[2] = 0 // 3 errors, ecSize/2 = 2

	dec := NewDecoder(field)
	_, err := dec.Decode(received, ecSize)
	if err == nil {
		t.Error("expected error for too many errors")
	}
}

func TestGaloisFieldBasics(t *testing.T) {
	field := QRCodeField256
	if field.Size() != 256 {
		t.Errorf("size = %d, want 256", field.Size())
	}
	if field.GeneratorBase() != 0 {
		t.Errorf("generatorBase = %d, want 0", field.GeneratorBase())
	}

	// a * inverse(a) should be 1
	for a := 1; a < 256; a++ {
		inv := field.Inverse(a)
		product := field.Multiply(a, inv)
		if product != 1 {
			t.Errorf("a=%d: a*inv(a) = %d, want 1", a, product)
		}
	}

	// a XOR a should be 0
	if AddOrSubtract(42, 42) != 0 {
		t.Error("a XOR a should be 0")
	}

	// multiply by 0
	if field.Multiply(0, 100) != 0 || field.Multiply(100, 0) != 0 {
		t.Error("multiply by 0 should be 0")
	}
}

func TestGenericGFPoly(t *testing.T) {
	field := QRCodeField256

	// Test zero polynomial
	zero := field.Zero()
	if !zero.IsZero() {
		t.Error("zero should be zero")
	}

	// Test one polynomial
	one := field.One()
	if one.IsZero() {
		t.Error("one should not be zero")
	}
	if one.Degree() != 0 {
		t.Errorf("one degree = %d, want 0", one.Degree())
	}

	// Test evaluation
	// p(x) = 2x + 3
	p := newGenericGFPoly(field, []int{2, 3})
	// p(0) = 3
	if p.EvaluateAt(0) != 3 {
		t.Errorf("p(0) = %d, want 3", p.EvaluateAt(0))
	}

	// Test multiply by scalar
	doubled := p.MultiplyScalar(1) // multiply by 1 should return same
	if doubled != p {
		t.Error("multiply by 1 should return same polynomial")
	}
}
