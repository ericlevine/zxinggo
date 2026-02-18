// Package detector implements Data Matrix barcode detection in binary images.
// This is a Go port of the ZXing Java Data Matrix detector.
//
// Data Matrix barcodes have an L-shaped finder pattern consisting of two solid
// edges (the "L") along the left and bottom, and two alternating black/white
// clock-track edges along the top and right. The detector locates these edges,
// determines the four corner points, counts modules along the clock tracks,
// and samples the grid to produce the bit matrix.
package detector

import (
	"math"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
	"github.com/ericlevine/zxinggo/transform"
)

// DetectorResult holds the result of detecting a Data Matrix barcode: the
// sampled bit matrix and the four corner points.
type DetectorResult struct {
	Bits   *bitutil.BitMatrix
	Points []zxinggo.ResultPoint
}

// initSize is the default initial search size for WhiteRectangleDetector.
const initSize = 10

// Detect locates a Data Matrix barcode in the given binary image and returns
// the sampled bit matrix along with the four corner points.
func Detect(image *bitutil.BitMatrix) (*DetectorResult, error) {
	// Step 1: use WhiteRectangleDetector to find a bounding white rectangle.
	wrd, err := newWhiteRectangleDetector(image)
	if err != nil {
		return nil, err
	}
	cornerPoints, err := wrd.detect()
	if err != nil {
		return nil, err
	}

	// cornerPoints are four points on the edges of the Data Matrix.
	pointA := cornerPoints[0]
	pointB := cornerPoints[1]
	pointC := cornerPoints[2]
	pointD := cornerPoints[3]

	// Step 2: Count transitions between each pair of adjacent corners.
	// The two edges with the fewest transitions are the solid L-shape edges.
	transitions := make([]resultPointsAndTransitions, 0, 4)
	transitions = append(transitions, transitionsBetween(image, pointA, pointB))
	transitions = append(transitions, transitionsBetween(image, pointA, pointC))
	transitions = append(transitions, transitionsBetween(image, pointB, pointD))
	transitions = append(transitions, transitionsBetween(image, pointC, pointD))
	sortByTransitions(transitions)

	lSideOne := transitions[0]
	lSideTwo := transitions[1]

	// Determine which point is shared by both L-shape edges (the corner of the L).
	pointCount := make(map[zxinggo.ResultPoint]int)
	increment(pointCount, lSideOne.from)
	increment(pointCount, lSideOne.to)
	increment(pointCount, lSideTwo.from)
	increment(pointCount, lSideTwo.to)

	var maybeTopLeft, bottomLeft, maybeBottomRight zxinggo.ResultPoint
	for point, count := range pointCount {
		if count == 2 {
			bottomLeft = point
		} else {
			if maybeTopLeft == (zxinggo.ResultPoint{}) {
				maybeTopLeft = point
			} else {
				maybeBottomRight = point
			}
		}
	}

	if bottomLeft == (zxinggo.ResultPoint{}) ||
		maybeTopLeft == (zxinggo.ResultPoint{}) ||
		maybeBottomRight == (zxinggo.ResultPoint{}) {
		return nil, zxinggo.ErrNotFound
	}

	// Order the three L-corner points using cross product to get consistent
	// orientation: bottomLeft is the corner of the L, topLeft and bottomRight
	// are the endpoints of the two solid edges.
	candidates := [3]zxinggo.ResultPoint{maybeTopLeft, bottomLeft, maybeBottomRight}
	candidates = zxinggo.OrderBestPatterns(candidates)
	// OrderBestPatterns returns [pointA, pointB, pointC] where pointA is
	// opposite the longest side (the vertex of the right angle), and pointB
	// and pointC are the ends of the longest side.
	bottomRight := candidates[0]
	bottomLeft = candidates[1]
	topLeft := candidates[2]

	// The fourth corner (topRight) is whichever white-rectangle corner point
	// was not used in the L-shape.
	topRight := selectFourthPoint(pointA, pointB, pointC, pointD,
		bottomLeft, topLeft, bottomRight)

	// Step 3: Count modules along the two clock-track edges.
	dimensionTop := transitionsBetween(image, topLeft, topRight).transitions + 2
	dimensionRight := transitionsBetween(image, bottomRight, topRight).transitions + 2

	// Data Matrix dimensions are always even.
	if dimensionTop%2 != 0 {
		dimensionTop++
	}
	if dimensionRight%2 != 0 {
		dimensionRight++
	}

	// Step 4: Build a perspective transform and sample the grid.
	xform, err := createDataMatrixTransform(
		topLeft, topRight, bottomRight, bottomLeft,
		dimensionTop, dimensionRight)
	if err != nil {
		return nil, err
	}

	sampler := &transform.DefaultGridSampler{}
	bits, err := sampler.SampleGridTransform(image, dimensionTop, dimensionRight, xform)
	if err != nil {
		return nil, err
	}

	return &DetectorResult{
		Bits:   bits,
		Points: []zxinggo.ResultPoint{topLeft, bottomLeft, bottomRight, topRight},
	}, nil
}

// selectFourthPoint picks the white-rectangle corner point that is not one of
// the three L-shape corners, i.e. the one farthest from bottomLeft.
func selectFourthPoint(a, b, c, d, bl, tl, br zxinggo.ResultPoint) zxinggo.ResultPoint {
	pts := []zxinggo.ResultPoint{a, b, c, d}
	bestScore := -1.0
	best := a
	for _, p := range pts {
		dBL := pointDistance(p, bl)
		dTL := pointDistance(p, tl)
		dBR := pointDistance(p, br)
		// The fourth corner maximises the minimum distance to any of the
		// three known corners (it should not be close to any of them).
		minD := math.Min(dBL, math.Min(dTL, dBR))
		if minD > bestScore {
			bestScore = minD
			best = p
		}
	}
	return best
}

// createDataMatrixTransform builds a PerspectiveTransform that maps logical
// module coordinates to image coordinates for grid sampling.
func createDataMatrixTransform(
	topLeft, topRight, bottomRight, bottomLeft zxinggo.ResultPoint,
	dimensionTop, dimensionRight int,
) (*transform.PerspectiveTransform, error) {
	if dimensionTop <= 0 || dimensionRight <= 0 {
		return nil, zxinggo.ErrNotFound
	}
	return transform.QuadrilateralToQuadrilateral(
		0.5, 0.5,
		float64(dimensionTop)-0.5, 0.5,
		float64(dimensionTop)-0.5, float64(dimensionRight)-0.5,
		0.5, float64(dimensionRight)-0.5,
		topLeft.X, topLeft.Y,
		topRight.X, topRight.Y,
		bottomRight.X, bottomRight.Y,
		bottomLeft.X, bottomLeft.Y,
	), nil
}

// ---------------------------------------------------------------------------
// Transition counting helpers
// ---------------------------------------------------------------------------

// resultPointsAndTransitions records the number of black/white transitions
// between two points in the image.
type resultPointsAndTransitions struct {
	from        zxinggo.ResultPoint
	to          zxinggo.ResultPoint
	transitions int
}

// transitionsBetween counts the number of black-to-white and white-to-black
// transitions along a line from 'from' to 'to' using Bresenham's algorithm.
func transitionsBetween(image *bitutil.BitMatrix, from, to zxinggo.ResultPoint) resultPointsAndTransitions {
	fromX := int(from.X)
	fromY := int(from.Y)
	toX := int(to.X)
	toY := int(to.Y)

	steep := iabs(toY-fromY) > iabs(toX-fromX)
	if steep {
		fromX, fromY = fromY, fromX
		toX, toY = toY, toX
	}

	dx := iabs(toX - fromX)
	dy := iabs(toY - fromY)
	err := -dx / 2
	ystep := 1
	if fromY > toY {
		ystep = -1
	}
	xstep := 1
	if fromX > toX {
		xstep = -1
	}

	transitions := 0
	inBlack := imageGet(image, fromX, fromY, steep)

	y := fromY
	for x := fromX; x != toX+xstep; x += xstep {
		isBlack := imageGet(image, x, y, steep)
		if isBlack != inBlack {
			transitions++
			inBlack = isBlack
		}
		err += dy
		if err > 0 {
			if y != toY {
				y += ystep
			}
			err -= dx
		}
	}
	return resultPointsAndTransitions{from: from, to: to, transitions: transitions}
}

// imageGet reads a pixel, swapping x/y when the line is steep.
func imageGet(image *bitutil.BitMatrix, x, y int, steep bool) bool {
	if steep {
		return image.Get(y, x)
	}
	return image.Get(x, y)
}

// sortByTransitions sorts in ascending order of transition count (insertion sort).
func sortByTransitions(t []resultPointsAndTransitions) {
	for i := 1; i < len(t); i++ {
		key := t[i]
		j := i - 1
		for j >= 0 && t[j].transitions > key.transitions {
			t[j+1] = t[j]
			j--
		}
		t[j+1] = key
	}
}

// increment adds one to the count for a point in a frequency map.
func increment(m map[zxinggo.ResultPoint]int, p zxinggo.ResultPoint) {
	m[p]++
}

// pointDistance returns the Euclidean distance between two ResultPoints.
func pointDistance(a, b zxinggo.ResultPoint) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return math.Sqrt(dx*dx + dy*dy)
}

// iabs returns the absolute value of an int.
func iabs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ---------------------------------------------------------------------------
// WhiteRectangleDetector
// ---------------------------------------------------------------------------

// whiteRectangleDetector locates a white rectangular region surrounding a
// barcode in a binary image.  Starting from the center it expands outward
// until each edge encounters black pixels, then walks the edges to find
// precise corner coordinates.
type whiteRectangleDetector struct {
	image     *bitutil.BitMatrix
	width     int
	height    int
	leftInit  int
	rightInit int
	downInit  int
	upInit    int
}

func newWhiteRectangleDetector(image *bitutil.BitMatrix) (*whiteRectangleDetector, error) {
	return newWhiteRectangleDetectorWithInit(image, initSize, image.Width()/2, image.Height()/2)
}

func newWhiteRectangleDetectorWithInit(image *bitutil.BitMatrix, halfInit, x, y int) (*whiteRectangleDetector, error) {
	w := image.Width()
	h := image.Height()

	li := x - halfInit
	ri := x + halfInit
	ui := y - halfInit
	di := y + halfInit

	if ui < 0 || li < 0 || di >= h || ri >= w {
		return nil, zxinggo.ErrNotFound
	}
	return &whiteRectangleDetector{
		image: image, width: w, height: h,
		leftInit: li, rightInit: ri, downInit: di, upInit: ui,
	}, nil
}

// detect expands the search rectangle and returns four corner points.
func (d *whiteRectangleDetector) detect() ([]zxinggo.ResultPoint, error) {
	left := d.leftInit
	right := d.rightInit
	up := d.upInit
	down := d.downInit

	sizeExceeded := false
	aBlackPointFoundOnBorder := true

	atLeastOneBlackPointFoundOnRight := false
	atLeastOneBlackPointFoundOnBottom := false
	atLeastOneBlackPointFoundOnLeft := false
	atLeastOneBlackPointFoundOnTop := false

	for aBlackPointFoundOnBorder {
		aBlackPointFoundOnBorder = false

		// Expand right edge.
		rightBorderNotWhite := true
		for (rightBorderNotWhite || !atLeastOneBlackPointFoundOnRight) && right < d.width {
			rightBorderNotWhite = d.containsBlackPoint(up, down, right, false)
			if rightBorderNotWhite {
				right++
				aBlackPointFoundOnBorder = true
				atLeastOneBlackPointFoundOnRight = true
			} else if !atLeastOneBlackPointFoundOnRight {
				right++
			}
		}
		if right >= d.width {
			sizeExceeded = true
			break
		}

		// Expand bottom edge.
		bottomBorderNotWhite := true
		for (bottomBorderNotWhite || !atLeastOneBlackPointFoundOnBottom) && down < d.height {
			bottomBorderNotWhite = d.containsBlackPoint(left, right, down, true)
			if bottomBorderNotWhite {
				down++
				aBlackPointFoundOnBorder = true
				atLeastOneBlackPointFoundOnBottom = true
			} else if !atLeastOneBlackPointFoundOnBottom {
				down++
			}
		}
		if down >= d.height {
			sizeExceeded = true
			break
		}

		// Expand left edge.
		leftBorderNotWhite := true
		for (leftBorderNotWhite || !atLeastOneBlackPointFoundOnLeft) && left >= 0 {
			leftBorderNotWhite = d.containsBlackPoint(up, down, left, false)
			if leftBorderNotWhite {
				left--
				aBlackPointFoundOnBorder = true
				atLeastOneBlackPointFoundOnLeft = true
			} else if !atLeastOneBlackPointFoundOnLeft {
				left--
			}
		}
		if left < 0 {
			sizeExceeded = true
			break
		}

		// Expand top edge.
		topBorderNotWhite := true
		for (topBorderNotWhite || !atLeastOneBlackPointFoundOnTop) && up >= 0 {
			topBorderNotWhite = d.containsBlackPoint(left, right, up, true)
			if topBorderNotWhite {
				up--
				aBlackPointFoundOnBorder = true
				atLeastOneBlackPointFoundOnTop = true
			} else if !atLeastOneBlackPointFoundOnTop {
				up--
			}
		}
		if up < 0 {
			sizeExceeded = true
			break
		}
	}

	if sizeExceeded ||
		!atLeastOneBlackPointFoundOnRight ||
		!atLeastOneBlackPointFoundOnBottom ||
		!atLeastOneBlackPointFoundOnLeft ||
		!atLeastOneBlackPointFoundOnTop {
		return nil, zxinggo.ErrNotFound
	}

	maxSize := right - left
	if down-up > maxSize {
		maxSize = down - up
	}

	// Walk each edge to find the precise corner points.
	var (
		pA, pB, pC, pD zxinggo.ResultPoint
		found          bool
	)

	// Bottom-left area: scan from left side toward bottom.
	for i := 1; !found && i < maxSize; i++ {
		pA, found = d.getBlackPointOnSegment(left, down-i, left+i, down)
	}
	if !found {
		return nil, zxinggo.ErrNotFound
	}

	// Top-left area: scan from left side toward top.
	found = false
	for i := 1; !found && i < maxSize; i++ {
		pB, found = d.getBlackPointOnSegment(left, up+i, left+i, up)
	}
	if !found {
		return nil, zxinggo.ErrNotFound
	}

	// Top-right area: scan from right side toward top.
	found = false
	for i := 1; !found && i < maxSize; i++ {
		pC, found = d.getBlackPointOnSegment(right, up+i, right-i, up)
	}
	if !found {
		return nil, zxinggo.ErrNotFound
	}

	// Bottom-right area: scan from right side toward bottom.
	found = false
	for i := 1; !found && i < maxSize; i++ {
		pD, found = d.getBlackPointOnSegment(right, down-i, right-i, down)
	}
	if !found {
		return nil, zxinggo.ErrNotFound
	}

	// Nudge corners inward slightly.
	ce := d.centerEdges(pA, pB, pC, pD)
	return []zxinggo.ResultPoint{ce[0], ce[1], ce[2], ce[3]}, nil
}

// centerEdges nudges the four corners slightly inward so that the sample
// points lie inside the barcode rather than on the quiet zone border.
func (d *whiteRectangleDetector) centerEdges(y, z, x, t zxinggo.ResultPoint) [4]zxinggo.ResultPoint {
	//    t --- z
	//    |     |
	//    y --- x

	yi, yj := y.X, y.Y
	zi, zj := z.X, z.Y
	xi, xj := x.X, x.Y
	ti, tj := t.X, t.Y

	if pointDistance(y, t) < float64(d.width)/7.0 {
		return [4]zxinggo.ResultPoint{
			{X: (yi + ti) / 2.0, Y: (yj + tj) / 2.0},
			{X: (zi + xi) / 2.0, Y: (zj + xj) / 2.0},
			{X: (yi + xi) / 2.0, Y: (yj + xj) / 2.0},
			{X: (ti + zi) / 2.0, Y: (tj + zj) / 2.0},
		}
	}

	const corr = 1.0
	return [4]zxinggo.ResultPoint{
		{X: yi + corr, Y: yj + corr},
		{X: zi + corr, Y: zj - corr},
		{X: xi - corr, Y: xj + corr},
		{X: ti - corr, Y: tj - corr},
	}
}

// getBlackPointOnSegment walks from (aX,aY) toward (bX,bY) and returns the
// first black pixel found, or false if none is found.
func (d *whiteRectangleDetector) getBlackPointOnSegment(aX, aY, bX, bY int) (zxinggo.ResultPoint, bool) {
	dist := distanceInt(aX, aY, bX, bY)
	if dist < 1 {
		return zxinggo.ResultPoint{}, false
	}
	xStep := float64(bX-aX) / dist
	yStep := float64(bY-aY) / dist

	for i := 0.0; i < dist; i++ {
		x := int(float64(aX) + i*xStep)
		y := int(float64(aY) + i*yStep)
		if x >= 0 && x < d.width && y >= 0 && y < d.height && d.image.Get(x, y) {
			return zxinggo.ResultPoint{X: float64(x), Y: float64(y)}, true
		}
	}
	return zxinggo.ResultPoint{}, false
}

// containsBlackPoint checks whether a line segment contains a black pixel.
// When horizontal is true, fixed is the y coordinate and a..b are x values.
// When horizontal is false, fixed is the x coordinate and a..b are y values.
func (d *whiteRectangleDetector) containsBlackPoint(a, b, fixed int, horizontal bool) bool {
	if horizontal {
		for x := a; x <= b; x++ {
			if x >= 0 && x < d.width && fixed >= 0 && fixed < d.height && d.image.Get(x, fixed) {
				return true
			}
		}
	} else {
		for y := a; y <= b; y++ {
			if fixed >= 0 && fixed < d.width && y >= 0 && y < d.height && d.image.Get(fixed, y) {
				return true
			}
		}
	}
	return false
}

// distanceInt returns the Euclidean distance between two integer-coordinate points.
func distanceInt(aX, aY, bX, bY int) float64 {
	dx := float64(aX - bX)
	dy := float64(aY - bY)
	return math.Sqrt(dx*dx + dy*dy)
}
