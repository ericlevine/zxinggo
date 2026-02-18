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

// detector holds the image and rectangle detector for detecting Data Matrix codes.
type detector struct {
	image             *bitutil.BitMatrix
	rectangleDetector *whiteRectangleDetector
}

// Detect locates a Data Matrix barcode in the given binary image and returns
// the sampled bit matrix along with the four corner points.
func Detect(image *bitutil.BitMatrix) (*DetectorResult, error) {
	wrd, err := newWhiteRectangleDetector(image)
	if err != nil {
		return nil, err
	}
	d := &detector{
		image:             image,
		rectangleDetector: wrd,
	}
	return d.detect()
}

func (d *detector) detect() (*DetectorResult, error) {
	cornerPoints, err := d.rectangleDetector.detect()
	if err != nil {
		return nil, err
	}

	points := d.detectSolid1(cornerPoints)
	points = d.detectSolid2(points)
	points[3] = d.correctTopRight(points)
	if points[3] == (zxinggo.ResultPoint{}) {
		return nil, zxinggo.ErrNotFound
	}
	points = d.shiftToModuleCenter(points)

	topLeft := points[0]
	bottomLeft := points[1]
	bottomRight := points[2]
	topRight := points[3]

	dimensionTop := d.transitionsBetween(topLeft, topRight) + 1
	dimensionRight := d.transitionsBetween(bottomRight, topRight) + 1
	if (dimensionTop & 0x01) == 1 {
		dimensionTop++
	}
	if (dimensionRight & 0x01) == 1 {
		dimensionRight++
	}

	if 4*dimensionTop < 6*dimensionRight && 4*dimensionRight < 6*dimensionTop {
		// The matrix is square
		if dimensionTop > dimensionRight {
			dimensionRight = dimensionTop
		} else {
			dimensionTop = dimensionRight
		}
	}

	bits, err := sampleGrid(d.image,
		topLeft, bottomLeft, bottomRight, topRight,
		dimensionTop, dimensionRight)
	if err != nil {
		return nil, err
	}

	return &DetectorResult{
		Bits:   bits,
		Points: []zxinggo.ResultPoint{topLeft, bottomLeft, bottomRight, topRight},
	}, nil
}

// shiftPoint shifts a point toward another point by 1/(div+1) of the distance.
func shiftPoint(point, to zxinggo.ResultPoint, div int) zxinggo.ResultPoint {
	x := (to.X - point.X) / float64(div+1)
	y := (to.Y - point.Y) / float64(div+1)
	return zxinggo.ResultPoint{X: point.X + x, Y: point.Y + y}
}

// moveAway moves a point away from a center point by 1 pixel in each axis.
func moveAway(point zxinggo.ResultPoint, fromX, fromY float64) zxinggo.ResultPoint {
	x := point.X
	y := point.Y

	if x < fromX {
		x -= 1
	} else {
		x += 1
	}

	if y < fromY {
		y -= 1
	} else {
		y += 1
	}

	return zxinggo.ResultPoint{X: x, Y: y}
}

// detectSolid1 detects the solid side which has the minimum number of transitions.
func (d *detector) detectSolid1(cornerPoints []zxinggo.ResultPoint) []zxinggo.ResultPoint {
	// 0  2
	// 1  3
	pointA := cornerPoints[0]
	pointB := cornerPoints[1]
	pointC := cornerPoints[3]
	pointD := cornerPoints[2]

	trAB := d.transitionsBetween(pointA, pointB)
	trBC := d.transitionsBetween(pointB, pointC)
	trCD := d.transitionsBetween(pointC, pointD)
	trDA := d.transitionsBetween(pointD, pointA)

	// 0..3
	// :  :
	// 1--2
	min := trAB
	points := []zxinggo.ResultPoint{pointD, pointA, pointB, pointC}
	if min > trBC {
		min = trBC
		points[0] = pointA
		points[1] = pointB
		points[2] = pointC
		points[3] = pointD
	}
	if min > trCD {
		min = trCD
		points[0] = pointB
		points[1] = pointC
		points[2] = pointD
		points[3] = pointA
	}
	if min > trDA {
		points[0] = pointC
		points[1] = pointD
		points[2] = pointA
		points[3] = pointB
	}

	return points
}

// detectSolid2 detects a second solid side next to the first solid side.
func (d *detector) detectSolid2(points []zxinggo.ResultPoint) []zxinggo.ResultPoint {
	// A..D
	// :  :
	// B--C
	pointA := points[0]
	pointB := points[1]
	pointC := points[2]
	pointD := points[3]

	// Transition detection on the edge is not stable.
	// To safely detect, shift the points to the module center.
	tr := d.transitionsBetween(pointA, pointD)
	pointBs := shiftPoint(pointB, pointC, (tr+1)*4)
	pointCs := shiftPoint(pointC, pointB, (tr+1)*4)
	trBA := d.transitionsBetween(pointBs, pointA)
	trCD := d.transitionsBetween(pointCs, pointD)

	// 0..3
	// |  :
	// 1--2
	if trBA < trCD {
		// solid sides: A-B-C
		points[0] = pointA
		points[1] = pointB
		points[2] = pointC
		points[3] = pointD
	} else {
		// solid sides: B-C-D
		points[0] = pointB
		points[1] = pointC
		points[2] = pointD
		points[3] = pointA
	}

	return points
}

// correctTopRight calculates the corner position of the white top right module.
func (d *detector) correctTopRight(points []zxinggo.ResultPoint) zxinggo.ResultPoint {
	// A..D
	// |  :
	// B--C
	pointA := points[0]
	pointB := points[1]
	pointC := points[2]
	pointD := points[3]

	// shift points for safe transition detection.
	trTop := d.transitionsBetween(pointA, pointD)
	trRight := d.transitionsBetween(pointB, pointD)
	pointAs := shiftPoint(pointA, pointB, (trRight+1)*4)
	pointCs := shiftPoint(pointC, pointB, (trTop+1)*4)

	trTop = d.transitionsBetween(pointAs, pointD)
	trRight = d.transitionsBetween(pointCs, pointD)

	candidate1 := zxinggo.ResultPoint{
		X: pointD.X + (pointC.X-pointB.X)/float64(trTop+1),
		Y: pointD.Y + (pointC.Y-pointB.Y)/float64(trTop+1),
	}
	candidate2 := zxinggo.ResultPoint{
		X: pointD.X + (pointA.X-pointB.X)/float64(trRight+1),
		Y: pointD.Y + (pointA.Y-pointB.Y)/float64(trRight+1),
	}

	if !d.isValid(candidate1) {
		if d.isValid(candidate2) {
			return candidate2
		}
		return zxinggo.ResultPoint{}
	}
	if !d.isValid(candidate2) {
		return candidate1
	}

	sumc1 := d.transitionsBetween(pointAs, candidate1) + d.transitionsBetween(pointCs, candidate1)
	sumc2 := d.transitionsBetween(pointAs, candidate2) + d.transitionsBetween(pointCs, candidate2)

	if sumc1 > sumc2 {
		return candidate1
	}
	return candidate2
}

// shiftToModuleCenter shifts the edge points to the module center.
func (d *detector) shiftToModuleCenter(points []zxinggo.ResultPoint) []zxinggo.ResultPoint {
	// A..D
	// |  :
	// B--C
	pointA := points[0]
	pointB := points[1]
	pointC := points[2]
	pointD := points[3]

	// calculate pseudo dimensions
	dimH := d.transitionsBetween(pointA, pointD) + 1
	dimV := d.transitionsBetween(pointC, pointD) + 1

	// shift points for safe dimension detection
	pointAs := shiftPoint(pointA, pointB, dimV*4)
	pointCs := shiftPoint(pointC, pointB, dimH*4)

	// calculate more precise dimensions
	dimH = d.transitionsBetween(pointAs, pointD) + 1
	dimV = d.transitionsBetween(pointCs, pointD) + 1
	if (dimH & 0x01) == 1 {
		dimH++
	}
	if (dimV & 0x01) == 1 {
		dimV++
	}

	// WhiteRectangleDetector returns points inside of the rectangle.
	// We want points on the edges.
	centerX := (pointA.X + pointB.X + pointC.X + pointD.X) / 4
	centerY := (pointA.Y + pointB.Y + pointC.Y + pointD.Y) / 4
	pointA = moveAway(pointA, centerX, centerY)
	pointB = moveAway(pointB, centerX, centerY)
	pointC = moveAway(pointC, centerX, centerY)
	pointD = moveAway(pointD, centerX, centerY)

	// shift points to the center of each module
	pointAs = shiftPoint(pointA, pointB, dimV*4)
	pointAs = shiftPoint(pointAs, pointD, dimH*4)
	pointBs := shiftPoint(pointB, pointA, dimV*4)
	pointBs = shiftPoint(pointBs, pointC, dimH*4)
	pointCs = shiftPoint(pointC, pointD, dimV*4)
	pointCs = shiftPoint(pointCs, pointB, dimH*4)
	pointDs := shiftPoint(pointD, pointC, dimV*4)
	pointDs = shiftPoint(pointDs, pointA, dimH*4)

	return []zxinggo.ResultPoint{pointAs, pointBs, pointCs, pointDs}
}

// isValid checks whether a point is within the image bounds.
func (d *detector) isValid(p zxinggo.ResultPoint) bool {
	return p.X >= 0 && p.X <= float64(d.image.Width()-1) && p.Y > 0 && p.Y <= float64(d.image.Height()-1)
}

// sampleGrid samples the image grid to produce the bit matrix.
func sampleGrid(image *bitutil.BitMatrix,
	topLeft, bottomLeft, bottomRight, topRight zxinggo.ResultPoint,
	dimensionX, dimensionY int) (*bitutil.BitMatrix, error) {

	sampler := &transform.DefaultGridSampler{}

	return sampler.SampleGrid(image,
		dimensionX,
		dimensionY,
		0.5,
		0.5,
		float64(dimensionX)-0.5,
		0.5,
		float64(dimensionX)-0.5,
		float64(dimensionY)-0.5,
		0.5,
		float64(dimensionY)-0.5,
		topLeft.X,
		topLeft.Y,
		topRight.X,
		topRight.Y,
		bottomRight.X,
		bottomRight.Y,
		bottomLeft.X,
		bottomLeft.Y,
	)
}

// transitionsBetween counts the number of black/white transitions between two
// points, using Bresenham's algorithm. This is a faithful port of the Java
// ZXing Detector.transitionsBetween method.
func (d *detector) transitionsBetween(from, to zxinggo.ResultPoint) int {
	fromX := int(from.X)
	fromY := int(from.Y)
	toX := int(to.X)
	toY := int(to.Y)
	if toY > d.image.Height()-1 {
		toY = d.image.Height() - 1
	}

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
	inBlack := d.image.Get(boolSelect(steep, fromY, fromX), boolSelect(steep, fromX, fromY))

	y := fromY
	for x := fromX; x != toX; x += xstep {
		isBlack := d.image.Get(boolSelect(steep, y, x), boolSelect(steep, x, y))
		if isBlack != inBlack {
			transitions++
			inBlack = isBlack
		}
		err += dy
		if err > 0 {
			if y == toY {
				break
			}
			y += ystep
			err -= dx
		}
	}
	return transitions
}

// boolSelect returns a if cond is true, otherwise b.
func boolSelect(cond bool, a, b int) int {
	if cond {
		return a
	}
	return b
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
// barcode in a binary image. Starting from the center it expands outward
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

func newWhiteRectangleDetectorWithInit(image *bitutil.BitMatrix, initSz, x, y int) (*whiteRectangleDetector, error) {
	w := image.Width()
	h := image.Height()

	halfsize := initSz / 2
	li := x - halfsize
	ri := x + halfsize
	ui := y - halfsize
	di := y + halfsize

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

	// Find the four corner points by walking edges.
	var z zxinggo.ResultPoint
	var found bool
	for i := 1; !found && i < maxSize; i++ {
		z, found = d.getBlackPointOnSegment(float64(left), float64(down-i), float64(left+i), float64(down))
	}
	if !found {
		return nil, zxinggo.ErrNotFound
	}

	var t zxinggo.ResultPoint
	found = false
	for i := 1; !found && i < maxSize; i++ {
		t, found = d.getBlackPointOnSegment(float64(left), float64(up+i), float64(left+i), float64(up))
	}
	if !found {
		return nil, zxinggo.ErrNotFound
	}

	var x zxinggo.ResultPoint
	found = false
	for i := 1; !found && i < maxSize; i++ {
		x, found = d.getBlackPointOnSegment(float64(right), float64(up+i), float64(right-i), float64(up))
	}
	if !found {
		return nil, zxinggo.ErrNotFound
	}

	var y zxinggo.ResultPoint
	found = false
	for i := 1; !found && i < maxSize; i++ {
		y, found = d.getBlackPointOnSegment(float64(right), float64(down-i), float64(right-i), float64(down))
	}
	if !found {
		return nil, zxinggo.ErrNotFound
	}

	return d.centerEdges(y, z, x, t), nil
}

// centerEdges recenters the points at a constant distance towards the center.
func (d *whiteRectangleDetector) centerEdges(y, z, x, t zxinggo.ResultPoint) []zxinggo.ResultPoint {
	//
	//       t            t
	//  z                      x
	//        x    OR    z
	//   y                    y
	//

	yi := y.X
	yj := y.Y
	zi := z.X
	zj := z.Y
	xi := x.X
	xj := x.Y
	ti := t.X
	tj := t.Y

	const corr = 1.0

	if yi < float64(d.width)/2.0 {
		return []zxinggo.ResultPoint{
			{X: ti - corr, Y: tj + corr},
			{X: zi + corr, Y: zj + corr},
			{X: xi - corr, Y: xj - corr},
			{X: yi + corr, Y: yj - corr},
		}
	}
	return []zxinggo.ResultPoint{
		{X: ti + corr, Y: tj + corr},
		{X: zi + corr, Y: zj - corr},
		{X: xi - corr, Y: xj + corr},
		{X: yi - corr, Y: yj - corr},
	}
}

// getBlackPointOnSegment walks from (aX,aY) toward (bX,bY) and returns the
// first black pixel found, or false if none is found.
func (d *whiteRectangleDetector) getBlackPointOnSegment(aX, aY, bX, bY float64) (zxinggo.ResultPoint, bool) {
	dist := mathRound(distanceFloat(aX, aY, bX, bY))
	if dist < 1 {
		return zxinggo.ResultPoint{}, false
	}
	xStep := (bX - aX) / float64(dist)
	yStep := (bY - aY) / float64(dist)

	for i := 0; i < dist; i++ {
		px := mathRound(aX + float64(i)*xStep)
		py := mathRound(aY + float64(i)*yStep)
		if px >= 0 && px < d.width && py >= 0 && py < d.height && d.image.Get(px, py) {
			return zxinggo.ResultPoint{X: float64(px), Y: float64(py)}, true
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
			if d.image.Get(x, fixed) {
				return true
			}
		}
	} else {
		for y := a; y <= b; y++ {
			if d.image.Get(fixed, y) {
				return true
			}
		}
	}
	return false
}

// mathRound rounds a float64 to the nearest int (matching Java's MathUtils.round).
func mathRound(d float64) int {
	if d < 0 {
		return int(d - 0.5)
	}
	return int(d + 0.5)
}

// distanceFloat returns the Euclidean distance between two points.
func distanceFloat(aX, aY, bX, bY float64) float64 {
	dx := aX - bX
	dy := aY - bY
	return math.Sqrt(dx*dx + dy*dy)
}
