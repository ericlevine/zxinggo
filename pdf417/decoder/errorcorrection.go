package decoder

import (
	zxinggo "github.com/ericlevine/zxinggo"
)

// ErrorCorrection implements PDF417 error correction using a modular
// arithmetic variant of Reed-Solomon coding.
type ErrorCorrection struct {
	field *ModulusGF
}

// NewErrorCorrection creates a new ErrorCorrection using the standard
// PDF417 Galois Field.
func NewErrorCorrection() *ErrorCorrection {
	return &ErrorCorrection{
		field: PDF417GF,
	}
}

// Decode corrects errors in the received codewords. numECCodewords is the
// number of codewords used for error correction, and erasures gives the
// known positions of errors (may be nil). It returns the number of errors
// corrected and modifies received in place. Returns an error if correction
// is not possible.
func (ec *ErrorCorrection) Decode(received []int, numECCodewords int, erasures []int) (int, error) {
	poly := NewModulusPoly(ec.field, received)
	S := make([]int, numECCodewords)
	hasError := false
	for i := numECCodewords; i > 0; i-- {
		eval := poly.EvaluateAt(ec.field.Exp(i))
		S[numECCodewords-i] = eval
		if eval != 0 {
			hasError = true
		}
	}

	if !hasError {
		return 0, nil
	}

	knownErrors := ec.field.One()
	if erasures != nil {
		for _, erasure := range erasures {
			b := ec.field.Exp(len(received) - 1 - erasure)
			// Add (1 - bx) term:
			term := NewModulusPoly(ec.field, []int{ec.field.Subtract(0, b), 1})
			knownErrors = knownErrors.Multiply(term)
		}
	}

	syndrome := NewModulusPoly(ec.field, S)

	sigmaOmega, err := ec.runEuclideanAlgorithm(
		ec.field.BuildMonomial(numECCodewords, 1), syndrome, numECCodewords)
	if err != nil {
		return 0, err
	}
	sigma := sigmaOmega[0]
	omega := sigmaOmega[1]

	errorLocations, err := ec.findErrorLocations(sigma)
	if err != nil {
		return 0, err
	}
	errorMagnitudes := ec.findErrorMagnitudes(omega, sigma, errorLocations)

	for i := 0; i < len(errorLocations); i++ {
		position := len(received) - 1 - ec.field.Log(errorLocations[i])
		if position < 0 {
			return 0, zxinggo.ErrChecksum
		}
		received[position] = ec.field.Subtract(received[position], errorMagnitudes[i])
	}
	return len(errorLocations), nil
}

// runEuclideanAlgorithm runs the extended Euclidean algorithm to find the
// error locator and error evaluator polynomials.
func (ec *ErrorCorrection) runEuclideanAlgorithm(a, b *ModulusPoly, R int) ([2]*ModulusPoly, error) {
	// Assume a's degree is >= b's
	if a.Degree() < b.Degree() {
		a, b = b, a
	}

	rLast := a
	r := b
	tLast := ec.field.Zero()
	t := ec.field.One()

	// Run Euclidean algorithm until r's degree is less than R/2
	for r.Degree() >= R/2 {
		rLastLast := rLast
		tLastLast := tLast
		rLast = r
		tLast = t

		// Divide rLastLast by rLast, with quotient in q and remainder in r
		if rLast.IsZero() {
			// Euclidean algorithm already terminated
			return [2]*ModulusPoly{}, zxinggo.ErrChecksum
		}
		r = rLastLast
		q := ec.field.Zero()
		denominatorLeadingTerm := rLast.GetCoefficient(rLast.Degree())
		dltInverse := ec.field.Inverse(denominatorLeadingTerm)
		for r.Degree() >= rLast.Degree() && !r.IsZero() {
			degreeDiff := r.Degree() - rLast.Degree()
			scale := ec.field.Multiply(r.GetCoefficient(r.Degree()), dltInverse)
			q = q.Add(ec.field.BuildMonomial(degreeDiff, scale))
			r = r.Subtract(rLast.MultiplyByMonomial(degreeDiff, scale))
		}

		t = q.Multiply(tLast).Subtract(tLastLast).Negative()
	}

	sigmaTildeAtZero := t.GetCoefficient(0)
	if sigmaTildeAtZero == 0 {
		return [2]*ModulusPoly{}, zxinggo.ErrChecksum
	}

	inverse := ec.field.Inverse(sigmaTildeAtZero)
	sigma := t.MultiplyScalar(inverse)
	omega := r.MultiplyScalar(inverse)
	return [2]*ModulusPoly{sigma, omega}, nil
}

// findErrorLocations uses Chien search to find the error locations from the
// error locator polynomial.
func (ec *ErrorCorrection) findErrorLocations(errorLocator *ModulusPoly) ([]int, error) {
	numErrors := errorLocator.Degree()
	result := make([]int, numErrors)
	e := 0
	for i := 1; i < ec.field.Size() && e < numErrors; i++ {
		if errorLocator.EvaluateAt(i) == 0 {
			result[e] = ec.field.Inverse(i)
			e++
		}
	}
	if e != numErrors {
		return nil, zxinggo.ErrChecksum
	}
	return result, nil
}

// findErrorMagnitudes uses Forney's formula to compute the error magnitudes
// given the error evaluator, error locator, and error locations.
func (ec *ErrorCorrection) findErrorMagnitudes(errorEvaluator, errorLocator *ModulusPoly, errorLocations []int) []int {
	errorLocatorDegree := errorLocator.Degree()
	if errorLocatorDegree < 1 {
		return []int{}
	}
	formalDerivativeCoefficients := make([]int, errorLocatorDegree)
	for i := 1; i <= errorLocatorDegree; i++ {
		formalDerivativeCoefficients[errorLocatorDegree-i] =
			ec.field.Multiply(i, errorLocator.GetCoefficient(i))
	}
	formalDerivative := NewModulusPoly(ec.field, formalDerivativeCoefficients)

	// Directly applying Forney's Formula
	s := len(errorLocations)
	result := make([]int, s)
	for i := 0; i < s; i++ {
		xiInverse := ec.field.Inverse(errorLocations[i])
		numerator := ec.field.Subtract(0, errorEvaluator.EvaluateAt(xiInverse))
		denominator := ec.field.Inverse(formalDerivative.EvaluateAt(xiInverse))
		result[i] = ec.field.Multiply(numerator, denominator)
	}
	return result
}
