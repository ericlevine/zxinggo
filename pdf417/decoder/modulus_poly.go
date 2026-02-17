package decoder

import "fmt"

// ModulusPoly represents a polynomial over a ModulusGF.
type ModulusPoly struct {
	field        *ModulusGF
	coefficients []int
}

// NewModulusPoly creates a new ModulusPoly in the given field with the given
// coefficients. Leading zeros are stripped so that the degree is correct.
func NewModulusPoly(field *ModulusGF, coefficients []int) *ModulusPoly {
	if len(coefficients) == 0 {
		panic("decoder: empty coefficients")
	}
	coefficientsLength := len(coefficients)
	if coefficientsLength > 1 && coefficients[0] == 0 {
		// Leading term must be non-zero for anything except the constant polynomial "0"
		firstNonZero := 1
		for firstNonZero < coefficientsLength && coefficients[firstNonZero] == 0 {
			firstNonZero++
		}
		if firstNonZero == coefficientsLength {
			coefficients = []int{0}
		} else {
			c := make([]int, coefficientsLength-firstNonZero)
			copy(c, coefficients[firstNonZero:])
			coefficients = c
		}
	}
	return &ModulusPoly{
		field:        field,
		coefficients: coefficients,
	}
}

// Coefficients returns the coefficient slice of this polynomial.
func (p *ModulusPoly) Coefficients() []int {
	return p.coefficients
}

// Degree returns the degree of this polynomial.
func (p *ModulusPoly) Degree() int {
	return len(p.coefficients) - 1
}

// IsZero returns true if this polynomial is the zero polynomial.
func (p *ModulusPoly) IsZero() bool {
	return p.coefficients[0] == 0
}

// GetCoefficient returns the coefficient of the x^degree term.
func (p *ModulusPoly) GetCoefficient(degree int) int {
	return p.coefficients[len(p.coefficients)-1-degree]
}

// EvaluateAt evaluates this polynomial at a given point.
func (p *ModulusPoly) EvaluateAt(a int) int {
	if a == 0 {
		// Just return the x^0 coefficient
		return p.GetCoefficient(0)
	}
	if a == 1 {
		// Sum of the coefficients
		result := 0
		for _, coefficient := range p.coefficients {
			result = p.field.Add(result, coefficient)
		}
		return result
	}
	result := p.coefficients[0]
	size := len(p.coefficients)
	for i := 1; i < size; i++ {
		result = p.field.Add(p.field.Multiply(a, result), p.coefficients[i])
	}
	return result
}

// Add returns the sum of this polynomial and other.
func (p *ModulusPoly) Add(other *ModulusPoly) *ModulusPoly {
	if p.field != other.field {
		panic("decoder: ModulusPolys do not have same ModulusGF field")
	}
	if p.IsZero() {
		return other
	}
	if other.IsZero() {
		return p
	}

	smallerCoefficients := p.coefficients
	largerCoefficients := other.coefficients
	if len(smallerCoefficients) > len(largerCoefficients) {
		smallerCoefficients, largerCoefficients = largerCoefficients, smallerCoefficients
	}
	sumDiff := make([]int, len(largerCoefficients))
	lengthDiff := len(largerCoefficients) - len(smallerCoefficients)
	// Copy high-order terms only found in higher-degree polynomial's coefficients
	copy(sumDiff, largerCoefficients[:lengthDiff])

	for i := lengthDiff; i < len(largerCoefficients); i++ {
		sumDiff[i] = p.field.Add(smallerCoefficients[i-lengthDiff], largerCoefficients[i])
	}

	return NewModulusPoly(p.field, sumDiff)
}

// Subtract returns the difference of this polynomial and other.
func (p *ModulusPoly) Subtract(other *ModulusPoly) *ModulusPoly {
	if p.field != other.field {
		panic("decoder: ModulusPolys do not have same ModulusGF field")
	}
	if other.IsZero() {
		return p
	}
	return p.Add(other.Negative())
}

// Multiply returns the product of this polynomial and other.
func (p *ModulusPoly) Multiply(other *ModulusPoly) *ModulusPoly {
	if p.field != other.field {
		panic("decoder: ModulusPolys do not have same ModulusGF field")
	}
	if p.IsZero() || other.IsZero() {
		return p.field.Zero()
	}
	aCoefficients := p.coefficients
	aLength := len(aCoefficients)
	bCoefficients := other.coefficients
	bLength := len(bCoefficients)
	product := make([]int, aLength+bLength-1)
	for i := 0; i < aLength; i++ {
		aCoeff := aCoefficients[i]
		for j := 0; j < bLength; j++ {
			product[i+j] = p.field.Add(product[i+j], p.field.Multiply(aCoeff, bCoefficients[j]))
		}
	}
	return NewModulusPoly(p.field, product)
}

// Negative returns the negation of this polynomial.
func (p *ModulusPoly) Negative() *ModulusPoly {
	size := len(p.coefficients)
	negativeCoefficients := make([]int, size)
	for i := 0; i < size; i++ {
		negativeCoefficients[i] = p.field.Subtract(0, p.coefficients[i])
	}
	return NewModulusPoly(p.field, negativeCoefficients)
}

// MultiplyScalar returns this polynomial multiplied by a scalar.
func (p *ModulusPoly) MultiplyScalar(scalar int) *ModulusPoly {
	if scalar == 0 {
		return p.field.Zero()
	}
	if scalar == 1 {
		return p
	}
	size := len(p.coefficients)
	product := make([]int, size)
	for i := 0; i < size; i++ {
		product[i] = p.field.Multiply(p.coefficients[i], scalar)
	}
	return NewModulusPoly(p.field, product)
}

// MultiplyByMonomial returns this polynomial multiplied by coefficient * x^degree.
func (p *ModulusPoly) MultiplyByMonomial(degree, coefficient int) *ModulusPoly {
	if degree < 0 {
		panic("decoder: negative degree")
	}
	if coefficient == 0 {
		return p.field.Zero()
	}
	size := len(p.coefficients)
	product := make([]int, size+degree)
	for i := 0; i < size; i++ {
		product[i] = p.field.Multiply(p.coefficients[i], coefficient)
	}
	return NewModulusPoly(p.field, product)
}

// String returns a string representation of this polynomial.
func (p *ModulusPoly) String() string {
	result := ""
	for degree := p.Degree(); degree >= 0; degree-- {
		coefficient := p.GetCoefficient(degree)
		if coefficient != 0 {
			if coefficient < 0 {
				result += " - "
				coefficient = -coefficient
			} else {
				if len(result) > 0 {
					result += " + "
				}
			}
			if degree == 0 || coefficient != 1 {
				result += fmt.Sprintf("%d", coefficient)
			}
			if degree != 0 {
				if degree == 1 {
					result += "x"
				} else {
					result += fmt.Sprintf("x^%d", degree)
				}
			}
		}
	}
	return result
}
