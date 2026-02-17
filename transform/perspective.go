// Package transform provides geometric transformation utilities for barcode detection.
package transform

// PerspectiveTransform implements a perspective transform in two dimensions.
type PerspectiveTransform struct {
	a11, a12, a13 float64
	a21, a22, a23 float64
	a31, a32, a33 float64
}

// QuadrilateralToQuadrilateral computes the transform from one quadrilateral to another.
func QuadrilateralToQuadrilateral(
	x0, y0, x1, y1, x2, y2, x3, y3 float64,
	x0p, y0p, x1p, y1p, x2p, y2p, x3p, y3p float64,
) *PerspectiveTransform {
	qToS := QuadrilateralToSquare(x0, y0, x1, y1, x2, y2, x3, y3)
	sToQ := SquareToQuadrilateral(x0p, y0p, x1p, y1p, x2p, y2p, x3p, y3p)
	return sToQ.Times(qToS)
}

// TransformPoints transforms pairs of (x, y) coordinates in-place.
// points must have even length: [x0, y0, x1, y1, ...].
func (pt *PerspectiveTransform) TransformPoints(points []float64) {
	maxI := len(points) - 1
	for i := 0; i < maxI; i += 2 {
		x := points[i]
		y := points[i+1]
		denominator := pt.a13*x + pt.a23*y + pt.a33
		points[i] = (pt.a11*x + pt.a21*y + pt.a31) / denominator
		points[i+1] = (pt.a12*x + pt.a22*y + pt.a32) / denominator
	}
}

// TransformPointsSeparate transforms separate x and y coordinate arrays.
func (pt *PerspectiveTransform) TransformPointsSeparate(xValues, yValues []float64) {
	for i := range xValues {
		x := xValues[i]
		y := yValues[i]
		denominator := pt.a13*x + pt.a23*y + pt.a33
		xValues[i] = (pt.a11*x + pt.a21*y + pt.a31) / denominator
		yValues[i] = (pt.a12*x + pt.a22*y + pt.a32) / denominator
	}
}

// SquareToQuadrilateral computes the transform from the unit square to a quadrilateral.
func SquareToQuadrilateral(x0, y0, x1, y1, x2, y2, x3, y3 float64) *PerspectiveTransform {
	dx3 := x0 - x1 + x2 - x3
	dy3 := y0 - y1 + y2 - y3
	if dx3 == 0 && dy3 == 0 {
		// Affine
		return &PerspectiveTransform{
			a11: x1 - x0, a21: x2 - x1, a31: x0,
			a12: y1 - y0, a22: y2 - y1, a32: y0,
			a13: 0, a23: 0, a33: 1,
		}
	}
	dx1 := x1 - x2
	dx2 := x3 - x2
	dy1 := y1 - y2
	dy2 := y3 - y2
	denominator := dx1*dy2 - dx2*dy1
	a13 := (dx3*dy2 - dx2*dy3) / denominator
	a23 := (dx1*dy3 - dx3*dy1) / denominator
	return &PerspectiveTransform{
		a11: x1 - x0 + a13*x1, a21: x3 - x0 + a23*x3, a31: x0,
		a12: y1 - y0 + a13*y1, a22: y3 - y0 + a23*y3, a32: y0,
		a13: a13, a23: a23, a33: 1,
	}
}

// QuadrilateralToSquare computes the transform from a quadrilateral to the unit square.
func QuadrilateralToSquare(x0, y0, x1, y1, x2, y2, x3, y3 float64) *PerspectiveTransform {
	return SquareToQuadrilateral(x0, y0, x1, y1, x2, y2, x3, y3).BuildAdjoint()
}

// BuildAdjoint returns the adjoint (transpose of the cofactor matrix).
func (pt *PerspectiveTransform) BuildAdjoint() *PerspectiveTransform {
	return &PerspectiveTransform{
		a11: pt.a22*pt.a33 - pt.a23*pt.a32,
		a21: pt.a23*pt.a31 - pt.a21*pt.a33,
		a31: pt.a21*pt.a32 - pt.a22*pt.a31,
		a12: pt.a13*pt.a32 - pt.a12*pt.a33,
		a22: pt.a11*pt.a33 - pt.a13*pt.a31,
		a32: pt.a12*pt.a31 - pt.a11*pt.a32,
		a13: pt.a12*pt.a23 - pt.a13*pt.a22,
		a23: pt.a13*pt.a21 - pt.a11*pt.a23,
		a33: pt.a11*pt.a22 - pt.a12*pt.a21,
	}
}

// Times returns this * other.
func (pt *PerspectiveTransform) Times(other *PerspectiveTransform) *PerspectiveTransform {
	return &PerspectiveTransform{
		a11: pt.a11*other.a11 + pt.a21*other.a12 + pt.a31*other.a13,
		a21: pt.a11*other.a21 + pt.a21*other.a22 + pt.a31*other.a23,
		a31: pt.a11*other.a31 + pt.a21*other.a32 + pt.a31*other.a33,
		a12: pt.a12*other.a11 + pt.a22*other.a12 + pt.a32*other.a13,
		a22: pt.a12*other.a21 + pt.a22*other.a22 + pt.a32*other.a23,
		a32: pt.a12*other.a31 + pt.a22*other.a32 + pt.a32*other.a33,
		a13: pt.a13*other.a11 + pt.a23*other.a12 + pt.a33*other.a13,
		a23: pt.a13*other.a21 + pt.a23*other.a22 + pt.a33*other.a23,
		a33: pt.a13*other.a31 + pt.a23*other.a32 + pt.a33*other.a33,
	}
}
