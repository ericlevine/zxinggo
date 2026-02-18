// Package detector implements Aztec barcode detection in binary images.
// This is a faithful Go port of the ZXing Java Aztec detector
// (com.google.zxing.aztec.detector.Detector).
package detector

import (
	"fmt"
	"math"
	"math/bits"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
	"github.com/ericlevine/zxinggo/reedsolomon"
	"github.com/ericlevine/zxinggo/transform"
)

// DetectorResult encapsulates the result of detecting an Aztec barcode.
type DetectorResult struct {
	Bits            *bitutil.BitMatrix
	Points          []zxinggo.ResultPoint
	Compact         bool
	NbDataBlocks    int
	NbLayers        int
	ErrorsCorrected int
}

// EXPECTED_CORNER_BITS for rotation detection.
// Each entry is a 12-bit pattern formed by concatenating the 3-bit orientation
// marks from each of the 4 sides.
var expectedCornerBits = [4]int{
	0xee0, // 07340  XXX .XX X.. ...
	0x1dc, // 00734  ... XXX .XX X..
	0x83b, // 04073  X.. ... XXX .XX
	0x707, // 03407  .XX X.. ... XXX
}

// point is an integer coordinate point (matching Java Detector.Point).
type point struct {
	x, y int
}

func (p point) toResultPoint() zxinggo.ResultPoint {
	return zxinggo.ResultPoint{X: float64(p.x), Y: float64(p.y)}
}

// correctedParameter holds the result of RS correction on parameter data.
type correctedParameter struct {
	data            int
	errorsCorrected int
}

// Detect locates an Aztec barcode in the given binary image and returns the
// detection result.
func Detect(image *bitutil.BitMatrix, isMirror bool) (*DetectorResult, error) {
	// 1. Get the center of the aztec matrix
	pCenter := getMatrixCenter(image)

	// 2. Get the center points of the four diagonal points just outside the bull's eye
	//  [topRight, bottomRight, bottomLeft, topLeft]
	bullsEyeCorners, compact, nbCenterLayers, err := getBullsEyeCorners(image, pCenter)
	if err != nil {
		return nil, err
	}

	if isMirror {
		bullsEyeCorners[0], bullsEyeCorners[2] = bullsEyeCorners[2], bullsEyeCorners[0]
	}

	// 3. Get the size of the matrix and other parameters from the bull's eye
	nbDataBlocks, nbLayers, shift, errorsCorrected, err := extractParameters(image, bullsEyeCorners, compact, nbCenterLayers)
	if err != nil {
		return nil, err
	}

	// 4. Sample the grid
	sampled, err := sampleGrid(image,
		bullsEyeCorners[shift%4],
		bullsEyeCorners[(shift+1)%4],
		bullsEyeCorners[(shift+2)%4],
		bullsEyeCorners[(shift+3)%4],
		compact, nbLayers, nbCenterLayers)
	if err != nil {
		return nil, err
	}

	// 5. Get the corners of the matrix.
	corners := getMatrixCornerPoints(bullsEyeCorners, nbCenterLayers, compact, nbLayers)

	return &DetectorResult{
		Bits:            sampled,
		Points:          corners,
		Compact:         compact,
		NbDataBlocks:    nbDataBlocks,
		NbLayers:        nbLayers,
		ErrorsCorrected: errorsCorrected,
	}, nil
}

// extractParameters reads the mode message from the ring around the bull's eye.
func extractParameters(image *bitutil.BitMatrix, bullsEyeCorners [4]zxinggo.ResultPoint, compact bool, nbCenterLayers int) (nbDataBlocks, nbLayers, shift, errorsCorrected int, err error) {
	if !isValidRP(image, bullsEyeCorners[0]) || !isValidRP(image, bullsEyeCorners[1]) ||
		!isValidRP(image, bullsEyeCorners[2]) || !isValidRP(image, bullsEyeCorners[3]) {
		return 0, 0, 0, 0, zxinggo.ErrNotFound
	}
	length := 2 * nbCenterLayers
	// Get the bits around the bull's eye
	sides := [4]int{
		sampleLine(image, bullsEyeCorners[0], bullsEyeCorners[1], length), // Right side
		sampleLine(image, bullsEyeCorners[1], bullsEyeCorners[2], length), // Bottom
		sampleLine(image, bullsEyeCorners[2], bullsEyeCorners[3], length), // Left side
		sampleLine(image, bullsEyeCorners[3], bullsEyeCorners[0], length), // Top
	}

	shift, err = getRotation(sides, length)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	// Flatten the parameter bits into a single 28- or 40-bit long
	var parameterData int64
	for i := 0; i < 4; i++ {
		side := sides[(shift+i)%4]
		if compact {
			// Each side of the form ..XXXXXXX. where Xs are parameter data
			parameterData <<= 7
			parameterData += int64((side >> 1) & 0x7F)
		} else {
			// Each side of the form ..XXXXX.XXXXX. where Xs are parameter data
			parameterData <<= 10
			parameterData += int64(((side >> 2) & (0x1f << 5)) + ((side >> 1) & 0x1F))
		}
	}

	// Corrects parameter data using RS
	corrected, err := getCorrectedParameterData(parameterData, compact)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	correctedData := corrected.data

	if compact {
		// 8 bits: 2 bits layers and 6 bits data blocks
		nbLayers = (correctedData >> 6) + 1
		nbDataBlocks = (correctedData & 0x3F) + 1
	} else {
		// 16 bits: 5 bits layers and 11 bits data blocks
		nbLayers = (correctedData >> 11) + 1
		nbDataBlocks = (correctedData & 0x7FF) + 1
	}

	return nbDataBlocks, nbLayers, shift, corrected.errorsCorrected, nil
}

// getRotation determines the rotation shift from orientation marks.
func getRotation(sides [4]int, length int) (int, error) {
	// Grab the 3 bits from each of the sides that form the locator pattern
	// and concatenate into a 12-bit integer.
	cornerBits := 0
	for _, side := range sides {
		// XX......X where X's are orientation marks
		t := ((side >> (length - 2)) << 1) + (side & 1)
		cornerBits = (cornerBits << 3) + t
	}
	// Move the bottom bit to the top, so that the three bits of the locator
	// pattern at A are together.
	cornerBits = ((cornerBits & 1) << 11) + (cornerBits >> 1)

	for shift := 0; shift < 4; shift++ {
		if bits.OnesCount(uint(cornerBits^expectedCornerBits[shift])) <= 2 {
			return shift, nil
		}
	}
	return 0, zxinggo.ErrNotFound
}

// getCorrectedParameterData corrects parameter data using Reed-Solomon.
func getCorrectedParameterData(parameterData int64, compact bool) (*correctedParameter, error) {
	var numCodewords, numDataCodewords int
	if compact {
		numCodewords = 7
		numDataCodewords = 2
	} else {
		numCodewords = 10
		numDataCodewords = 4
	}

	numECCodewords := numCodewords - numDataCodewords
	parameterWords := make([]int, numCodewords)
	for i := numCodewords - 1; i >= 0; i-- {
		parameterWords[i] = int(parameterData & 0xF)
		parameterData >>= 4
	}

	rsDecoder := reedsolomon.NewDecoder(reedsolomon.AztecParam)
	errorsCorrected, err := rsDecoder.Decode(parameterWords, numECCodewords)
	if err != nil {
		return nil, zxinggo.ErrNotFound
	}

	// Toss the error correction. Just return the data as an integer
	result := 0
	for i := 0; i < numDataCodewords; i++ {
		result = (result << 4) + parameterWords[i]
	}
	return &correctedParameter{data: result, errorsCorrected: errorsCorrected}, nil
}

// getBullsEyeCorners finds the corners of the bull-eye centered on pCenter.
// Returns [topRight, bottomRight, bottomLeft, topLeft].
func getBullsEyeCorners(image *bitutil.BitMatrix, pCenter point) ([4]zxinggo.ResultPoint, bool, int, error) {
	pina := pCenter
	pinb := pCenter
	pinc := pCenter
	pind := pCenter

	color := true

	var nbCenterLayers int
	for nbCenterLayers = 1; nbCenterLayers < 9; nbCenterLayers++ {
		pouta := getFirstDifferent(image, pina, color, 1, -1)
		poutb := getFirstDifferent(image, pinb, color, 1, 1)
		poutc := getFirstDifferent(image, pinc, color, -1, 1)
		poutd := getFirstDifferent(image, pind, color, -1, -1)

		//d      a
		//
		//c      b

		if nbCenterLayers > 2 {
			q := distanceP(poutd, pouta) * float64(nbCenterLayers) / (distanceP(pind, pina) * float64(nbCenterLayers+2))
			if q < 0.75 || q > 1.25 || !isWhiteOrBlackRectangle(image, pouta, poutb, poutc, poutd) {
				break
			}
		}

		pina = pouta
		pinb = poutb
		pinc = poutc
		pind = poutd

		color = !color
	}

	if nbCenterLayers != 5 && nbCenterLayers != 7 {
		return [4]zxinggo.ResultPoint{}, false, 0, zxinggo.ErrNotFound
	}

	compact := nbCenterLayers == 5

	// Expand the square by .5 pixel in each direction so that we're on the border
	// between the white square and the black square
	pinax := zxinggo.ResultPoint{X: float64(pina.x) + 0.5, Y: float64(pina.y) - 0.5}
	pinbx := zxinggo.ResultPoint{X: float64(pinb.x) + 0.5, Y: float64(pinb.y) + 0.5}
	pincx := zxinggo.ResultPoint{X: float64(pinc.x) - 0.5, Y: float64(pinc.y) + 0.5}
	pindx := zxinggo.ResultPoint{X: float64(pind.x) - 0.5, Y: float64(pind.y) - 0.5}

	// Expand the square so that its corners are the centers of the points
	// just outside the bull's eye.
	corners := expandSquare(
		[4]zxinggo.ResultPoint{pinax, pinbx, pincx, pindx},
		2*nbCenterLayers-3,
		2*nbCenterLayers)

	return corners, compact, nbCenterLayers, nil
}

// getMatrixCenter locates the approximate center of the Aztec bullseye.
func getMatrixCenter(image *bitutil.BitMatrix) point {
	var pointA, pointB, pointC, pointD zxinggo.ResultPoint

	// Get a white rectangle that can be the border of the matrix in center bull's eye
	wrd, err := newWhiteRectangleDetector(image)
	if err == nil {
		var cornerPoints []zxinggo.ResultPoint
		cornerPoints, err = wrd.detect()
		if err == nil {
			pointA = cornerPoints[0]
			pointB = cornerPoints[1]
			pointC = cornerPoints[2]
			pointD = cornerPoints[3]
		}
	}
	if err != nil {
		// This exception can be in case the initial rectangle is white
		// In that case, surely in the bull's eye, we try to expand the rectangle.
		cx := image.Width() / 2
		cy := image.Height() / 2
		pointA = getFirstDifferent(image, point{cx + 7, cy - 7}, false, 1, -1).toResultPoint()
		pointB = getFirstDifferent(image, point{cx + 7, cy + 7}, false, 1, 1).toResultPoint()
		pointC = getFirstDifferent(image, point{cx - 7, cy + 7}, false, -1, 1).toResultPoint()
		pointD = getFirstDifferent(image, point{cx - 7, cy - 7}, false, -1, -1).toResultPoint()
	}

	// Compute the center of the rectangle
	cx := mathRound((pointA.X + pointD.X + pointB.X + pointC.X) / 4.0)
	cy := mathRound((pointA.Y + pointD.Y + pointB.Y + pointC.Y) / 4.0)

	// Redetermine the white rectangle starting from previously computed center.
	wrd2, err := newWhiteRectangleDetectorWithInit(image, 15, cx, cy)
	if err == nil {
		var cornerPoints []zxinggo.ResultPoint
		cornerPoints, err = wrd2.detect()
		if err == nil {
			pointA = cornerPoints[0]
			pointB = cornerPoints[1]
			pointC = cornerPoints[2]
			pointD = cornerPoints[3]
		}
	}
	if err != nil {
		// Fallback: try to expand the rectangle
		pointA = getFirstDifferent(image, point{cx + 7, cy - 7}, false, 1, -1).toResultPoint()
		pointB = getFirstDifferent(image, point{cx + 7, cy + 7}, false, 1, 1).toResultPoint()
		pointC = getFirstDifferent(image, point{cx - 7, cy + 7}, false, -1, 1).toResultPoint()
		pointD = getFirstDifferent(image, point{cx - 7, cy - 7}, false, -1, -1).toResultPoint()
	}

	// Recompute the center of the rectangle
	cx = mathRound((pointA.X + pointD.X + pointB.X + pointC.X) / 4.0)
	cy = mathRound((pointA.Y + pointD.Y + pointB.Y + pointC.Y) / 4.0)

	return point{cx, cy}
}

// getMatrixCornerPoints gets the Aztec code corners from the bull's eye corners.
func getMatrixCornerPoints(bullsEyeCorners [4]zxinggo.ResultPoint, nbCenterLayers int, compact bool, nbLayers int) []zxinggo.ResultPoint {
	expanded := expandSquare(bullsEyeCorners, 2*nbCenterLayers, getDimension(compact, nbLayers))
	return expanded[:]
}

// sampleGrid creates a BitMatrix by sampling the provided image.
func sampleGrid(image *bitutil.BitMatrix,
	topLeft, topRight, bottomRight, bottomLeft zxinggo.ResultPoint,
	compact bool, nbLayers, nbCenterLayers int) (*bitutil.BitMatrix, error) {

	sampler := &transform.DefaultGridSampler{}
	dimension := getDimension(compact, nbLayers)

	low := float64(dimension)/2.0 - float64(nbCenterLayers)
	high := float64(dimension)/2.0 + float64(nbCenterLayers)

	return sampler.SampleGrid(image,
		dimension,
		dimension,
		low, low, // topleft
		high, low, // topright
		high, high, // bottomright
		low, high, // bottomleft
		topLeft.X, topLeft.Y,
		topRight.X, topRight.Y,
		bottomRight.X, bottomRight.Y,
		bottomLeft.X, bottomLeft.Y)
}

// sampleLine samples a line between two points.
// p1 is inclusive, p2 is exclusive.
// Returns the array of bits as an int (first bit is high-order bit of result).
func sampleLine(image *bitutil.BitMatrix, p1, p2 zxinggo.ResultPoint, size int) int {
	result := 0

	d := distanceRP(p1, p2)
	moduleSize := d / float64(size)
	px := p1.X
	py := p1.Y
	dx := moduleSize * (p2.X - p1.X) / d
	dy := moduleSize * (p2.Y - p1.Y) / d
	for i := 0; i < size; i++ {
		ix := mathRound(px + float64(i)*dx)
		iy := mathRound(py + float64(i)*dy)
		if ix >= 0 && ix < image.Width() && iy >= 0 && iy < image.Height() {
			if image.Get(ix, iy) {
				result |= 1 << uint(size-i-1)
			}
		}
	}
	return result
}

// isWhiteOrBlackRectangle checks if the border of the rectangle is all white or all black.
func isWhiteOrBlackRectangle(image *bitutil.BitMatrix, p1, p2, p3, p4 point) bool {
	corr := 3

	p1 = point{
		x: max(0, p1.x-corr),
		y: min(image.Height()-1, p1.y+corr),
	}
	p2 = point{
		x: max(0, p2.x-corr),
		y: max(0, p2.y-corr),
	}
	p3 = point{
		x: min(image.Width()-1, p3.x+corr),
		y: max(0, min(image.Height()-1, p3.y-corr)),
	}
	p4 = point{
		x: min(image.Width()-1, p4.x+corr),
		y: min(image.Height()-1, p4.y+corr),
	}

	cInit := getColor(image, p4, p1)
	if cInit == 0 {
		return false
	}

	c := getColor(image, p1, p2)
	if c != cInit {
		return false
	}

	c = getColor(image, p2, p3)
	if c != cInit {
		return false
	}

	c = getColor(image, p3, p4)
	return c == cInit
}

// getColor gets the color of a segment.
// Returns 1 if segment more than 90% black, -1 if more than 90% white, 0 else.
func getColor(image *bitutil.BitMatrix, p1, p2 point) int {
	d := distanceP(p1, p2)
	if d == 0.0 {
		return 0
	}
	dx := float64(p2.x-p1.x) / d
	dy := float64(p2.y-p1.y) / d
	errorCount := 0

	px := float64(p1.x)
	py := float64(p1.y)

	colorModel := image.Get(p1.x, p1.y)

	iMax := int(math.Floor(d))
	for i := 0; i < iMax; i++ {
		ix := mathRound(px)
		iy := mathRound(py)
		if ix >= 0 && ix < image.Width() && iy >= 0 && iy < image.Height() {
			if image.Get(ix, iy) != colorModel {
				errorCount++
			}
		}
		px += dx
		py += dy
	}

	errRatio := float64(errorCount) / d

	if errRatio > 0.1 && errRatio < 0.9 {
		return 0
	}

	if (errRatio <= 0.1) == colorModel {
		return 1
	}
	return -1
}

// getFirstDifferent gets the coordinate of the first point with a different color
// in the given direction.
func getFirstDifferent(image *bitutil.BitMatrix, init point, color bool, dx, dy int) point {
	x := init.x + dx
	y := init.y + dy

	for isValid(image, x, y) && image.Get(x, y) == color {
		x += dx
		y += dy
	}

	x -= dx
	y -= dy

	for isValid(image, x, y) && image.Get(x, y) == color {
		x += dx
	}
	x -= dx

	for isValid(image, x, y) && image.Get(x, y) == color {
		y += dy
	}
	y -= dy

	return point{x, y}
}

// expandSquare expands the square by pushing out equally in all directions.
func expandSquare(cornerPoints [4]zxinggo.ResultPoint, oldSide, newSide int) [4]zxinggo.ResultPoint {
	ratio := float64(newSide) / (2.0 * float64(oldSide))
	dx := cornerPoints[0].X - cornerPoints[2].X
	dy := cornerPoints[0].Y - cornerPoints[2].Y
	centerx := (cornerPoints[0].X + cornerPoints[2].X) / 2.0
	centery := (cornerPoints[0].Y + cornerPoints[2].Y) / 2.0

	result0 := zxinggo.ResultPoint{X: centerx + ratio*dx, Y: centery + ratio*dy}
	result2 := zxinggo.ResultPoint{X: centerx - ratio*dx, Y: centery - ratio*dy}

	dx = cornerPoints[1].X - cornerPoints[3].X
	dy = cornerPoints[1].Y - cornerPoints[3].Y
	centerx = (cornerPoints[1].X + cornerPoints[3].X) / 2.0
	centery = (cornerPoints[1].Y + cornerPoints[3].Y) / 2.0
	result1 := zxinggo.ResultPoint{X: centerx + ratio*dx, Y: centery + ratio*dy}
	result3 := zxinggo.ResultPoint{X: centerx - ratio*dx, Y: centery - ratio*dy}

	return [4]zxinggo.ResultPoint{result0, result1, result2, result3}
}

func isValid(image *bitutil.BitMatrix, x, y int) bool {
	return x >= 0 && x < image.Width() && y >= 0 && y < image.Height()
}

func isValidRP(image *bitutil.BitMatrix, p zxinggo.ResultPoint) bool {
	x := mathRound(p.X)
	y := mathRound(p.Y)
	return isValid(image, x, y)
}

func distanceP(a, b point) float64 {
	dx := float64(a.x - b.x)
	dy := float64(a.y - b.y)
	return math.Sqrt(dx*dx + dy*dy)
}

func distanceRP(a, b zxinggo.ResultPoint) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return math.Sqrt(dx*dx + dy*dy)
}

// getDimension returns the dimension of the Aztec symbol.
func getDimension(compact bool, nbLayers int) int {
	if compact {
		return 4*nbLayers + 11
	}
	return 4*nbLayers + 2*((2*nbLayers+6)/15) + 15
}

func mathRound(f float64) int {
	return int(math.Round(f))
}

// ---------------------------------------------------------------------------
// WhiteRectangleDetector (local copy for Aztec center-finding)
// ---------------------------------------------------------------------------

const wrdInitSize = 10

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
	return newWhiteRectangleDetectorWithInit(image, wrdInitSize, image.Width()/2, image.Height()/2)
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
		return nil, fmt.Errorf("aztec detector: white rectangle detector init out of bounds")
	}
	return &whiteRectangleDetector{
		image: image, width: w, height: h,
		leftInit: li, rightInit: ri, downInit: di, upInit: ui,
	}, nil
}

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

	var (
		pA, pB, pC, pD zxinggo.ResultPoint
		found           bool
	)

	// Bottom-left area
	for i := 1; !found && i < maxSize; i++ {
		pA, found = d.getBlackPointOnSegment(left, down-i, left+i, down)
	}
	if !found {
		return nil, zxinggo.ErrNotFound
	}

	// Top-left area
	found = false
	for i := 1; !found && i < maxSize; i++ {
		pB, found = d.getBlackPointOnSegment(left, up+i, left+i, up)
	}
	if !found {
		return nil, zxinggo.ErrNotFound
	}

	// Top-right area
	found = false
	for i := 1; !found && i < maxSize; i++ {
		pC, found = d.getBlackPointOnSegment(right, up+i, right-i, up)
	}
	if !found {
		return nil, zxinggo.ErrNotFound
	}

	// Bottom-right area
	found = false
	for i := 1; !found && i < maxSize; i++ {
		pD, found = d.getBlackPointOnSegment(right, down-i, right-i, down)
	}
	if !found {
		return nil, zxinggo.ErrNotFound
	}

	return []zxinggo.ResultPoint{pA, pB, pC, pD}, nil
}

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

// ---------------------------------------------------------------------------
// Utility functions
// ---------------------------------------------------------------------------

func distanceInt(aX, aY, bX, bY int) float64 {
	dx := float64(aX - bX)
	dy := float64(aY - bY)
	return math.Sqrt(dx*dx + dy*dy)
}

