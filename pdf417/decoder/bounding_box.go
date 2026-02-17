package decoder

import (
	"math"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// BoundingBox represents the bounding box around a PDF417 barcode in the image.
type BoundingBox struct {
	image       *bitutil.BitMatrix
	topLeft     zxinggo.ResultPoint
	bottomLeft  zxinggo.ResultPoint
	topRight    zxinggo.ResultPoint
	bottomRight zxinggo.ResultPoint
	minX        int
	maxX        int
	minY        int
	maxY        int
}

// NewBoundingBox creates a new BoundingBox. The topLeft/bottomLeft pair or the
// topRight/bottomRight pair (or both) must be non-nil. If one side is nil,
// it is inferred from the other side and the image dimensions. Nil points are
// indicated by passing a nil pointer.
func NewBoundingBox(image *bitutil.BitMatrix,
	topLeft, bottomLeft, topRight, bottomRight *zxinggo.ResultPoint) (*BoundingBox, error) {

	leftUnspecified := topLeft == nil || bottomLeft == nil
	rightUnspecified := topRight == nil || bottomRight == nil
	if leftUnspecified && rightUnspecified {
		return nil, zxinggo.ErrNotFound
	}

	var tl, bl, tr, br zxinggo.ResultPoint

	if leftUnspecified {
		tl = zxinggo.ResultPoint{X: 0, Y: topRight.Y}
		bl = zxinggo.ResultPoint{X: 0, Y: bottomRight.Y}
		tr = *topRight
		br = *bottomRight
	} else if rightUnspecified {
		tl = *topLeft
		bl = *bottomLeft
		tr = zxinggo.ResultPoint{X: float64(image.Width() - 1), Y: topLeft.Y}
		br = zxinggo.ResultPoint{X: float64(image.Width() - 1), Y: bottomLeft.Y}
	} else {
		tl = *topLeft
		bl = *bottomLeft
		tr = *topRight
		br = *bottomRight
	}

	return &BoundingBox{
		image:       image,
		topLeft:     tl,
		bottomLeft:  bl,
		topRight:    tr,
		bottomRight: br,
		minX:        int(math.Min(tl.X, bl.X)),
		maxX:        int(math.Max(tr.X, br.X)),
		minY:        int(math.Min(tl.Y, tr.Y)),
		maxY:        int(math.Max(bl.Y, br.Y)),
	}, nil
}

// CopyBoundingBox creates a copy of the given BoundingBox.
func CopyBoundingBox(bb *BoundingBox) *BoundingBox {
	return &BoundingBox{
		image:       bb.image,
		topLeft:     bb.topLeft,
		bottomLeft:  bb.bottomLeft,
		topRight:    bb.topRight,
		bottomRight: bb.bottomRight,
		minX:        bb.minX,
		maxX:        bb.maxX,
		minY:        bb.minY,
		maxY:        bb.maxY,
	}
}

// MergeBoundingBoxes merges a left and right bounding box. If one is nil, the other is returned.
func MergeBoundingBoxes(leftBox, rightBox *BoundingBox) (*BoundingBox, error) {
	if leftBox == nil {
		return rightBox, nil
	}
	if rightBox == nil {
		return leftBox, nil
	}
	tl := leftBox.topLeft
	bl := leftBox.bottomLeft
	tr := rightBox.topRight
	br := rightBox.bottomRight
	return NewBoundingBox(leftBox.image, &tl, &bl, &tr, &br)
}

// AddMissingRows extends the bounding box by the specified number of missing
// rows at the start (top) and end (bottom), on the left or right side.
func (bb *BoundingBox) AddMissingRows(missingStartRows, missingEndRows int, isLeft bool) (*BoundingBox, error) {
	newTopLeft := bb.topLeft
	newBottomLeft := bb.bottomLeft
	newTopRight := bb.topRight
	newBottomRight := bb.bottomRight

	if missingStartRows > 0 {
		top := bb.topLeft
		if !isLeft {
			top = bb.topRight
		}
		newMinY := int(top.Y) - missingStartRows
		if newMinY < 0 {
			newMinY = 0
		}
		newTop := zxinggo.ResultPoint{X: top.X, Y: float64(newMinY)}
		if isLeft {
			newTopLeft = newTop
		} else {
			newTopRight = newTop
		}
	}

	if missingEndRows > 0 {
		bottom := bb.bottomLeft
		if !isLeft {
			bottom = bb.bottomRight
		}
		newMaxY := int(bottom.Y) + missingEndRows
		if newMaxY >= bb.image.Height() {
			newMaxY = bb.image.Height() - 1
		}
		newBottom := zxinggo.ResultPoint{X: bottom.X, Y: float64(newMaxY)}
		if isLeft {
			newBottomLeft = newBottom
		} else {
			newBottomRight = newBottom
		}
	}

	return NewBoundingBox(bb.image, &newTopLeft, &newBottomLeft, &newTopRight, &newBottomRight)
}

// MinX returns the minimum x coordinate.
func (bb *BoundingBox) MinX() int { return bb.minX }

// MaxX returns the maximum x coordinate.
func (bb *BoundingBox) MaxX() int { return bb.maxX }

// MinY returns the minimum y coordinate.
func (bb *BoundingBox) MinY() int { return bb.minY }

// MaxY returns the maximum y coordinate.
func (bb *BoundingBox) MaxY() int { return bb.maxY }

// TopLeft returns the top-left point.
func (bb *BoundingBox) TopLeft() zxinggo.ResultPoint { return bb.topLeft }

// TopRight returns the top-right point.
func (bb *BoundingBox) TopRight() zxinggo.ResultPoint { return bb.topRight }

// BottomLeft returns the bottom-left point.
func (bb *BoundingBox) BottomLeft() zxinggo.ResultPoint { return bb.bottomLeft }

// BottomRight returns the bottom-right point.
func (bb *BoundingBox) BottomRight() zxinggo.ResultPoint { return bb.bottomRight }
