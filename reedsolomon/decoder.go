package reedsolomon

import "errors"

// ErrReedSolomon indicates a Reed-Solomon decoding failure.
var ErrReedSolomon = errors.New("reedsolomon: decoding error")

// Decoder performs Reed-Solomon error correction decoding.
type Decoder struct {
	field *GenericGF
}

// NewDecoder creates a new Decoder for the given field.
func NewDecoder(field *GenericGF) *Decoder {
	return &Decoder{field: field}
}

// Decode corrects errors in received in-place and returns the number of
// errors corrected. twoS is the number of error-correction codewords.
func (d *Decoder) Decode(received []int, twoS int) (int, error) {
	poly := newGenericGFPoly(d.field, received)
	syndromeCoefficients := make([]int, twoS)
	noError := true
	for i := 0; i < twoS; i++ {
		eval := poly.EvaluateAt(d.field.Exp(i + d.field.GeneratorBase()))
		syndromeCoefficients[twoS-1-i] = eval
		if eval != 0 {
			noError = false
		}
	}
	if noError {
		return 0, nil
	}

	syndrome := newGenericGFPoly(d.field, syndromeCoefficients)
	sigmaOmega, err := d.runEuclideanAlgorithm(d.field.BuildMonomial(twoS, 1), syndrome, twoS)
	if err != nil {
		return 0, err
	}
	sigma := sigmaOmega[0]
	omega := sigmaOmega[1]
	errorLocations, err := d.findErrorLocations(sigma)
	if err != nil {
		return 0, err
	}
	errorMagnitudes := d.findErrorMagnitudes(omega, errorLocations)
	for i := 0; i < len(errorLocations); i++ {
		position := len(received) - 1 - d.field.Log(errorLocations[i])
		if position < 0 {
			return 0, ErrReedSolomon
		}
		received[position] = AddOrSubtract(received[position], errorMagnitudes[i])
	}
	return len(errorLocations), nil
}

func (d *Decoder) runEuclideanAlgorithm(a, b *GenericGFPoly, R int) ([2]*GenericGFPoly, error) {
	if a.Degree() < b.Degree() {
		a, b = b, a
	}

	rLast := a
	r := b
	tLast := d.field.Zero()
	t := d.field.One()

	for 2*r.Degree() >= R {
		rLastLast := rLast
		tLastLast := tLast
		rLast = r
		tLast = t

		if rLast.IsZero() {
			return [2]*GenericGFPoly{}, ErrReedSolomon
		}
		r = rLastLast
		q := d.field.Zero()
		denominatorLeadingTerm := rLast.GetCoefficient(rLast.Degree())
		dltInverse := d.field.Inverse(denominatorLeadingTerm)
		for r.Degree() >= rLast.Degree() && !r.IsZero() {
			degreeDiff := r.Degree() - rLast.Degree()
			scale := d.field.Multiply(r.GetCoefficient(r.Degree()), dltInverse)
			q = q.AddOrSubtractPoly(d.field.BuildMonomial(degreeDiff, scale))
			r = r.AddOrSubtractPoly(rLast.MultiplyByMonomial(degreeDiff, scale))
		}

		t = q.MultiplyPoly(tLast).AddOrSubtractPoly(tLastLast)

		if r.Degree() >= rLast.Degree() {
			return [2]*GenericGFPoly{}, ErrReedSolomon
		}
	}

	sigmaTildeAtZero := t.GetCoefficient(0)
	if sigmaTildeAtZero == 0 {
		return [2]*GenericGFPoly{}, ErrReedSolomon
	}

	inverse := d.field.Inverse(sigmaTildeAtZero)
	sigma := t.MultiplyScalar(inverse)
	omega := r.MultiplyScalar(inverse)
	return [2]*GenericGFPoly{sigma, omega}, nil
}

func (d *Decoder) findErrorLocations(errorLocator *GenericGFPoly) ([]int, error) {
	numErrors := errorLocator.Degree()
	if numErrors == 1 {
		return []int{errorLocator.GetCoefficient(1)}, nil
	}
	result := make([]int, 0, numErrors)
	for i := 1; i < d.field.Size() && len(result) < numErrors; i++ {
		if errorLocator.EvaluateAt(i) == 0 {
			result = append(result, d.field.Inverse(i))
		}
	}
	if len(result) != numErrors {
		return nil, ErrReedSolomon
	}
	return result, nil
}

func (d *Decoder) findErrorMagnitudes(errorEvaluator *GenericGFPoly, errorLocations []int) []int {
	s := len(errorLocations)
	result := make([]int, s)
	for i := 0; i < s; i++ {
		xiInverse := d.field.Inverse(errorLocations[i])
		denominator := 1
		for j := 0; j < s; j++ {
			if i != j {
				term := d.field.Multiply(errorLocations[j], xiInverse)
				termPlus1 := term | 1
				if term&1 != 0 {
					termPlus1 = term &^ 1
				}
				denominator = d.field.Multiply(denominator, termPlus1)
			}
		}
		result[i] = d.field.Multiply(errorEvaluator.EvaluateAt(xiInverse), d.field.Inverse(denominator))
		if d.field.GeneratorBase() != 0 {
			result[i] = d.field.Multiply(result[i], xiInverse)
		}
	}
	return result
}
