package reedsolomon

// GenericGFPoly represents a polynomial whose coefficients are elements of a GF.
// Instances are immutable.
type GenericGFPoly struct {
	field        *GenericGF
	coefficients []int
}

// newGenericGFPoly creates a new polynomial. Coefficients are ordered from
// highest-degree to lowest-degree.
func newGenericGFPoly(field *GenericGF, coefficients []int) *GenericGFPoly {
	if len(coefficients) == 0 {
		panic("reedsolomon: empty coefficients")
	}
	if len(coefficients) > 1 && coefficients[0] == 0 {
		firstNonZero := 1
		for firstNonZero < len(coefficients) && coefficients[firstNonZero] == 0 {
			firstNonZero++
		}
		if firstNonZero == len(coefficients) {
			coefficients = []int{0}
		} else {
			newCoeff := make([]int, len(coefficients)-firstNonZero)
			copy(newCoeff, coefficients[firstNonZero:])
			coefficients = newCoeff
		}
	}
	return &GenericGFPoly{field: field, coefficients: coefficients}
}

// Coefficients returns the polynomial coefficients.
func (p *GenericGFPoly) Coefficients() []int {
	return p.coefficients
}

// Degree returns the degree of this polynomial.
func (p *GenericGFPoly) Degree() int {
	return len(p.coefficients) - 1
}

// IsZero returns true if this is the zero polynomial.
func (p *GenericGFPoly) IsZero() bool {
	return p.coefficients[0] == 0
}

// GetCoefficient returns the coefficient of x^degree.
func (p *GenericGFPoly) GetCoefficient(degree int) int {
	return p.coefficients[len(p.coefficients)-1-degree]
}

// EvaluateAt evaluates this polynomial at a.
func (p *GenericGFPoly) EvaluateAt(a int) int {
	if a == 0 {
		return p.GetCoefficient(0)
	}
	if a == 1 {
		result := 0
		for _, c := range p.coefficients {
			result = AddOrSubtract(result, c)
		}
		return result
	}
	result := p.coefficients[0]
	for i := 1; i < len(p.coefficients); i++ {
		result = AddOrSubtract(p.field.Multiply(a, result), p.coefficients[i])
	}
	return result
}

// AddOrSubtractPoly adds (or subtracts) another polynomial.
func (p *GenericGFPoly) AddOrSubtractPoly(other *GenericGFPoly) *GenericGFPoly {
	if p.IsZero() {
		return other
	}
	if other.IsZero() {
		return p
	}

	smallerCoeff := p.coefficients
	largerCoeff := other.coefficients
	if len(smallerCoeff) > len(largerCoeff) {
		smallerCoeff, largerCoeff = largerCoeff, smallerCoeff
	}

	sumDiff := make([]int, len(largerCoeff))
	lengthDiff := len(largerCoeff) - len(smallerCoeff)
	copy(sumDiff, largerCoeff[:lengthDiff])

	for i := lengthDiff; i < len(largerCoeff); i++ {
		sumDiff[i] = AddOrSubtract(smallerCoeff[i-lengthDiff], largerCoeff[i])
	}

	return newGenericGFPoly(p.field, sumDiff)
}

// MultiplyPoly multiplies by another polynomial.
func (p *GenericGFPoly) MultiplyPoly(other *GenericGFPoly) *GenericGFPoly {
	if p.IsZero() || other.IsZero() {
		return p.field.Zero()
	}
	aCoeff := p.coefficients
	bCoeff := other.coefficients
	product := make([]int, len(aCoeff)+len(bCoeff)-1)
	for i, ac := range aCoeff {
		for j, bc := range bCoeff {
			product[i+j] = AddOrSubtract(product[i+j], p.field.Multiply(ac, bc))
		}
	}
	return newGenericGFPoly(p.field, product)
}

// MultiplyScalar multiplies by a scalar.
func (p *GenericGFPoly) MultiplyScalar(scalar int) *GenericGFPoly {
	if scalar == 0 {
		return p.field.Zero()
	}
	if scalar == 1 {
		return p
	}
	product := make([]int, len(p.coefficients))
	for i, c := range p.coefficients {
		product[i] = p.field.Multiply(c, scalar)
	}
	return newGenericGFPoly(p.field, product)
}

// MultiplyByMonomial multiplies by coefficient * x^degree.
func (p *GenericGFPoly) MultiplyByMonomial(degree, coefficient int) *GenericGFPoly {
	if degree < 0 {
		panic("reedsolomon: negative degree")
	}
	if coefficient == 0 {
		return p.field.Zero()
	}
	product := make([]int, len(p.coefficients)+degree)
	for i, c := range p.coefficients {
		product[i] = p.field.Multiply(c, coefficient)
	}
	return newGenericGFPoly(p.field, product)
}

// Divide divides by another polynomial, returning [quotient, remainder].
func (p *GenericGFPoly) Divide(other *GenericGFPoly) [2]*GenericGFPoly {
	if other.IsZero() {
		panic("reedsolomon: divide by zero")
	}

	quotient := p.field.Zero()
	remainder := p

	denominatorLeadingTerm := other.GetCoefficient(other.Degree())
	inverseDLT := p.field.Inverse(denominatorLeadingTerm)

	for remainder.Degree() >= other.Degree() && !remainder.IsZero() {
		degreeDiff := remainder.Degree() - other.Degree()
		scale := p.field.Multiply(remainder.GetCoefficient(remainder.Degree()), inverseDLT)
		term := other.MultiplyByMonomial(degreeDiff, scale)
		iterQuot := p.field.BuildMonomial(degreeDiff, scale)
		quotient = quotient.AddOrSubtractPoly(iterQuot)
		remainder = remainder.AddOrSubtractPoly(term)
	}

	return [2]*GenericGFPoly{quotient, remainder}
}
