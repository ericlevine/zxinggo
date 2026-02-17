package reedsolomon

// Encoder performs Reed-Solomon encoding.
type Encoder struct {
	field            *GenericGF
	cachedGenerators []*GenericGFPoly
}

// NewEncoder creates a new Encoder for the given field.
func NewEncoder(field *GenericGF) *Encoder {
	e := &Encoder{
		field:            field,
		cachedGenerators: make([]*GenericGFPoly, 1),
	}
	e.cachedGenerators[0] = newGenericGFPoly(field, []int{1})
	return e
}

func (e *Encoder) buildGenerator(degree int) *GenericGFPoly {
	if degree < len(e.cachedGenerators) {
		return e.cachedGenerators[degree]
	}
	lastGenerator := e.cachedGenerators[len(e.cachedGenerators)-1]
	for d := len(e.cachedGenerators); d <= degree; d++ {
		nextGenerator := lastGenerator.MultiplyPoly(
			newGenericGFPoly(e.field, []int{1, e.field.Exp(d - 1 + e.field.GeneratorBase())}))
		e.cachedGenerators = append(e.cachedGenerators, nextGenerator)
		lastGenerator = nextGenerator
	}
	return e.cachedGenerators[degree]
}

// Encode appends ecBytes error-correction codewords to the data in toEncode.
// toEncode must have space for data + ecBytes values.
func (e *Encoder) Encode(toEncode []int, ecBytes int) {
	if ecBytes == 0 {
		panic("reedsolomon: no error correction bytes")
	}
	dataBytes := len(toEncode) - ecBytes
	if dataBytes <= 0 {
		panic("reedsolomon: no data bytes provided")
	}
	generator := e.buildGenerator(ecBytes)
	infoCoefficients := make([]int, dataBytes)
	copy(infoCoefficients, toEncode[:dataBytes])
	info := newGenericGFPoly(e.field, infoCoefficients)
	info = info.MultiplyByMonomial(ecBytes, 1)
	remainder := info.Divide(generator)[1]
	coefficients := remainder.Coefficients()
	numZero := ecBytes - len(coefficients)
	for i := 0; i < numZero; i++ {
		toEncode[dataBytes+i] = 0
	}
	copy(toEncode[dataBytes+numZero:], coefficients)
}
