// Package reedsolomon implements Reed-Solomon error correction coding.
package reedsolomon

import "fmt"

// GenericGF represents a Galois Field for Reed-Solomon coding.
type GenericGF struct {
	expTable      []int
	logTable      []int
	zero          *GenericGFPoly
	one           *GenericGFPoly
	size          int
	primitive     int
	generatorBase int
}

// Pre-defined Galois Fields.
var (
	QRCodeField256    = NewGenericGF(0x011D, 256, 0) // x^8 + x^4 + x^3 + x^2 + 1
	DataMatrixField256 = NewGenericGF(0x012D, 256, 1) // x^8 + x^5 + x^3 + x^2 + 1
	AztecData12       = NewGenericGF(0x1069, 4096, 1)
	AztecData10       = NewGenericGF(0x0409, 1024, 1)
	AztecData8        = DataMatrixField256
	AztecData6        = NewGenericGF(0x0043, 64, 1)
	AztecParam        = NewGenericGF(0x0013, 16, 1)
	MaxiCodeField64   = AztecData6
)

// NewGenericGF creates a GF(size) using the given primitive polynomial.
func NewGenericGF(primitive, size, generatorBase int) *GenericGF {
	gf := &GenericGF{
		primitive:     primitive,
		size:          size,
		generatorBase: generatorBase,
		expTable:      make([]int, size),
		logTable:      make([]int, size),
	}

	x := 1
	for i := 0; i < size; i++ {
		gf.expTable[i] = x
		x *= 2
		if x >= size {
			x ^= primitive
			x &= size - 1
		}
	}
	for i := 0; i < size-1; i++ {
		gf.logTable[gf.expTable[i]] = i
	}

	gf.zero = newGenericGFPoly(gf, []int{0})
	gf.one = newGenericGFPoly(gf, []int{1})

	return gf
}

// Zero returns the zero polynomial.
func (gf *GenericGF) Zero() *GenericGFPoly { return gf.zero }

// One returns the one polynomial.
func (gf *GenericGF) One() *GenericGFPoly { return gf.one }

// BuildMonomial returns coefficient * x^degree.
func (gf *GenericGF) BuildMonomial(degree, coefficient int) *GenericGFPoly {
	if degree < 0 {
		panic("reedsolomon: negative degree")
	}
	if coefficient == 0 {
		return gf.zero
	}
	coefficients := make([]int, degree+1)
	coefficients[0] = coefficient
	return newGenericGFPoly(gf, coefficients)
}

// AddOrSubtract computes a XOR b (addition and subtraction are the same in GF(2^n)).
func AddOrSubtract(a, b int) int {
	return a ^ b
}

// Exp returns 2^a in this field.
func (gf *GenericGF) Exp(a int) int {
	return gf.expTable[a]
}

// Log returns log2(a) in this field.
func (gf *GenericGF) Log(a int) int {
	if a == 0 {
		panic("reedsolomon: log(0)")
	}
	return gf.logTable[a]
}

// Inverse returns the multiplicative inverse of a.
func (gf *GenericGF) Inverse(a int) int {
	if a == 0 {
		panic("reedsolomon: inverse(0)")
	}
	return gf.expTable[gf.size-gf.logTable[a]-1]
}

// Multiply returns a * b in this field.
func (gf *GenericGF) Multiply(a, b int) int {
	if a == 0 || b == 0 {
		return 0
	}
	return gf.expTable[(gf.logTable[a]+gf.logTable[b])%(gf.size-1)]
}

// Size returns the size of the field.
func (gf *GenericGF) Size() int { return gf.size }

// GeneratorBase returns the generator base.
func (gf *GenericGF) GeneratorBase() int { return gf.generatorBase }

// String returns a string representation.
func (gf *GenericGF) String() string {
	return fmt.Sprintf("GF(0x%x,%d)", gf.primitive, gf.size)
}
