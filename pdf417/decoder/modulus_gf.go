// Package decoder implements the PDF417 barcode decoder.
package decoder

// ModulusGF represents a field based on powers of a generator integer, modulo
// some modulus. It is used for PDF417 error correction.
type ModulusGF struct {
	expTable []int
	logTable []int
	zero     *ModulusPoly
	one      *ModulusPoly
	modulus  int
}

// PDF417GF is the pre-built Galois Field for PDF417 (modulus 929, generator 3).
// This must be a var initialization (not init()) so that other package-level
// vars like scanErrorCorrection can depend on it via Go's dependency ordering.
var PDF417GF = NewModulusGF(929, 3)

// NewModulusGF creates a new ModulusGF with the given modulus and generator.
// It builds the exponential and logarithm lookup tables.
func NewModulusGF(modulus, generator int) *ModulusGF {
	gf := &ModulusGF{
		modulus:  modulus,
		expTable: make([]int, modulus),
		logTable: make([]int, modulus),
	}

	x := 1
	for i := 0; i < modulus; i++ {
		gf.expTable[i] = x
		x = (x * generator) % modulus
	}
	for i := 0; i < modulus-1; i++ {
		gf.logTable[gf.expTable[i]] = i
	}
	// logTable[0] == 0 but this should never be used

	gf.zero = NewModulusPoly(gf, []int{0})
	gf.one = NewModulusPoly(gf, []int{1})

	return gf
}

// Zero returns the zero polynomial for this field.
func (gf *ModulusGF) Zero() *ModulusPoly {
	return gf.zero
}

// One returns the one polynomial for this field.
func (gf *ModulusGF) One() *ModulusPoly {
	return gf.one
}

// BuildMonomial returns coefficient * x^degree in this field.
func (gf *ModulusGF) BuildMonomial(degree, coefficient int) *ModulusPoly {
	if degree < 0 {
		panic("decoder: negative degree")
	}
	if coefficient == 0 {
		return gf.zero
	}
	coefficients := make([]int, degree+1)
	coefficients[0] = coefficient
	return NewModulusPoly(gf, coefficients)
}

// Add returns (a + b) mod modulus.
func (gf *ModulusGF) Add(a, b int) int {
	return (a + b) % gf.modulus
}

// Subtract returns (a - b) mod modulus.
func (gf *ModulusGF) Subtract(a, b int) int {
	return (gf.modulus + a - b) % gf.modulus
}

// Exp returns the exponential table value at index a.
func (gf *ModulusGF) Exp(a int) int {
	return gf.expTable[a]
}

// Log returns the logarithm of a in this field. Panics if a is 0.
func (gf *ModulusGF) Log(a int) int {
	if a == 0 {
		panic("decoder: log(0)")
	}
	return gf.logTable[a]
}

// Inverse returns the multiplicative inverse of a. Panics if a is 0.
func (gf *ModulusGF) Inverse(a int) int {
	if a == 0 {
		panic("decoder: inverse(0)")
	}
	return gf.expTable[gf.modulus-gf.logTable[a]-1]
}

// Multiply returns a * b in this field.
func (gf *ModulusGF) Multiply(a, b int) int {
	if a == 0 || b == 0 {
		return 0
	}
	return gf.expTable[(gf.logTable[a]+gf.logTable[b])%(gf.modulus-1)]
}

// Size returns the modulus (size) of this field.
func (gf *ModulusGF) Size() int {
	return gf.modulus
}
