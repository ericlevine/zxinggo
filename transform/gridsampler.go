package transform

import (
	"errors"

	"github.com/ericlevine/zxinggo/bitutil"
)

// ErrNotFound is returned when sampling fails.
var ErrNotFound = errors.New("gridsampler: not found")

// GridSampler samples an image to reconstruct a barcode, accounting for
// perspective distortion.
type GridSampler interface {
	SampleGrid(image *bitutil.BitMatrix, dimensionX, dimensionY int,
		p1ToX, p1ToY, p2ToX, p2ToY, p3ToX, p3ToY, p4ToX, p4ToY float64,
		p1FromX, p1FromY, p2FromX, p2FromY, p3FromX, p3FromY, p4FromX, p4FromY float64,
	) (*bitutil.BitMatrix, error)

	SampleGridTransform(image *bitutil.BitMatrix, dimensionX, dimensionY int,
		transform *PerspectiveTransform,
	) (*bitutil.BitMatrix, error)
}

// DefaultGridSampler is the standard GridSampler implementation.
type DefaultGridSampler struct{}

// SampleGrid samples with explicit corner points.
func (s *DefaultGridSampler) SampleGrid(image *bitutil.BitMatrix, dimensionX, dimensionY int,
	p1ToX, p1ToY, p2ToX, p2ToY, p3ToX, p3ToY, p4ToX, p4ToY float64,
	p1FromX, p1FromY, p2FromX, p2FromY, p3FromX, p3FromY, p4FromX, p4FromY float64,
) (*bitutil.BitMatrix, error) {
	transform := QuadrilateralToQuadrilateral(
		p1ToX, p1ToY, p2ToX, p2ToY, p3ToX, p3ToY, p4ToX, p4ToY,
		p1FromX, p1FromY, p2FromX, p2FromY, p3FromX, p3FromY, p4FromX, p4FromY)
	return s.SampleGridTransform(image, dimensionX, dimensionY, transform)
}

// SampleGridTransform samples using a pre-computed transform.
func (s *DefaultGridSampler) SampleGridTransform(image *bitutil.BitMatrix, dimensionX, dimensionY int,
	transform *PerspectiveTransform,
) (*bitutil.BitMatrix, error) {
	if dimensionX <= 0 || dimensionY <= 0 {
		return nil, ErrNotFound
	}
	bits := bitutil.NewBitMatrixWithSize(dimensionX, dimensionY)
	points := make([]float64, 2*dimensionX)
	for y := 0; y < dimensionY; y++ {
		iValue := float64(y) + 0.5
		for x := 0; x < len(points); x += 2 {
			points[x] = float64(x/2) + 0.5
			points[x+1] = iValue
		}
		transform.TransformPoints(points)
		if err := CheckAndNudgePoints(image, points); err != nil {
			return nil, err
		}
		for x := 0; x < len(points); x += 2 {
			ix := int(points[x])
			iy := int(points[x+1])
			if ix >= 0 && ix < image.Width() && iy >= 0 && iy < image.Height() {
				if image.Get(ix, iy) {
					bits.Set(x/2, y)
				}
			} else {
				return nil, ErrNotFound
			}
		}
	}
	return bits, nil
}

// CheckAndNudgePoints checks that transformed points are within image bounds,
// nudging slightly if they are barely outside.
func CheckAndNudgePoints(image *bitutil.BitMatrix, points []float64) error {
	width := image.Width()
	height := image.Height()
	maxOffset := len(points) - 1

	// Check from start
	nudged := true
	for offset := 0; offset < maxOffset && nudged; offset += 2 {
		x := int(points[offset])
		y := int(points[offset+1])
		if x < -1 || x > width || y < -1 || y > height {
			return ErrNotFound
		}
		nudged = false
		if x == -1 {
			points[offset] = 0
			nudged = true
		} else if x == width {
			points[offset] = float64(width - 1)
			nudged = true
		}
		if y == -1 {
			points[offset+1] = 0
			nudged = true
		} else if y == height {
			points[offset+1] = float64(height - 1)
			nudged = true
		}
	}

	// Check from end
	nudged = true
	for offset := len(points) - 2; offset >= 0 && nudged; offset -= 2 {
		x := int(points[offset])
		y := int(points[offset+1])
		if x < -1 || x > width || y < -1 || y > height {
			return ErrNotFound
		}
		nudged = false
		if x == -1 {
			points[offset] = 0
			nudged = true
		} else if x == width {
			points[offset] = float64(width - 1)
			nudged = true
		}
		if y == -1 {
			points[offset+1] = 0
			nudged = true
		} else if y == height {
			points[offset+1] = float64(height - 1)
			nudged = true
		}
	}
	return nil
}
