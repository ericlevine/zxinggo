// Package detector implements Aztec barcode detection in binary images.
// This is a Go port of the ZXing Java Aztec detector
// (com.google.zxing.aztec.detector.Detector).
//
// An Aztec code has a central bullseye finder pattern consisting of
// concentric alternating black/white rings. The mode message is encoded
// in a ring of modules just outside the bullseye. Data layers surround
// the mode-message ring.
//
// Compact Aztec codes have a 5x5 bullseye (2 rings), 28-bit mode message.
// Full-range Aztec codes have a 9x9 bullseye (3 rings), 40-bit mode message.
package detector

import (
	"fmt"
	"math"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
	"github.com/ericlevine/zxinggo/reedsolomon"
	"github.com/ericlevine/zxinggo/transform"
)

// DetectorResult encapsulates the result of detecting an Aztec barcode: the
// sampled bit matrix, corner points, whether the code is compact, and the
// number of data blocks and layers.
type DetectorResult struct {
	Bits         *bitutil.BitMatrix
	Points       []zxinggo.ResultPoint
	Compact      bool
	NbDataBlocks int
	NbLayers     int
}

// EXPECTED_CORNER_BITS contains the expected bit patterns at the four corners
// of the bullseye used for orientation detection. Index 0 is for compact
// symbols, index 1 for full-range symbols.
//
// For compact (7x7 outer ring, 3-bit corners):
//
//	corner 0: 111 = 0x07
//	corner 1: 010 = 0x02
//	corner 2: 001 = 0x01
//	corner 3: 100 = 0x04
//
// For full-range (11x11 outer ring, 5-bit corners):
//
//	corner 0: 11101 = 0x1D
//	corner 1: 01001 = 0x09
//	corner 2: 00101 = 0x05
//	corner 3: 10011 = 0x13
var expectedCornerBits = [2][4]int{
	{0x07, 0x02, 0x01, 0x04}, // compact
	{0x1D, 0x09, 0x05, 0x13}, // full-range
}

// Detect locates an Aztec barcode in the given binary image and returns the
// detection result containing the sampled bit matrix, corner points, and
// symbol parameters (compact, nbDataBlocks, nbLayers).
//
// If isMirror is true, the detector expects a horizontally mirrored symbol.
func Detect(image *bitutil.BitMatrix, isMirror bool) (*DetectorResult, error) {
	// Step 1: Find the center of the bullseye pattern.
	pCenter, err := getMatrixCenter(image)
	if err != nil {
		return nil, err
	}

	// Step 2: Get the four corners of the bullseye and determine whether
	// the symbol is compact or full-range.
	bullseyeCorners, compact, err := getBullseyeCorners(image, pCenter)
	if err != nil {
		return nil, err
	}

	// Step 3: Read the orientation marks and extract the mode message
	// (nbDataBlocks and nbLayers).
	nbDataBlocks, nbLayers, shift, err := extractParameters(image, bullseyeCorners, compact, isMirror)
	if err != nil {
		return nil, err
	}

	// Step 4: Sample the grid to extract the full symbol.
	bits, corners, err := sampleGrid(image, bullseyeCorners[0], bullseyeCorners[1],
		bullseyeCorners[2], bullseyeCorners[3], compact, nbLayers, shift)
	if err != nil {
		return nil, err
	}

	return &DetectorResult{
		Bits:         bits,
		Points:       corners,
		Compact:      compact,
		NbDataBlocks: nbDataBlocks,
		NbLayers:     nbLayers,
	}, nil
}

// ---------------------------------------------------------------------------
// Step 1: Find the center of the bullseye
// ---------------------------------------------------------------------------

// getMatrixCenter locates the approximate center of the Aztec bullseye.
// It uses a WhiteRectangleDetector to find a black region, then refines
// the center by tracing the ring-transition pattern along the cardinal axes.
func getMatrixCenter(image *bitutil.BitMatrix) (zxinggo.ResultPoint, error) {
	// Try the WhiteRectangleDetector first.
	var cx, cy int
	wrd, err := newWhiteRectangleDetector(image)
	if err == nil {
		corners, err2 := wrd.detect()
		if err2 == nil {
			cx = iround((corners[0].X + corners[1].X + corners[2].X + corners[3].X) / 4.0)
			cy = iround((corners[0].Y + corners[1].Y + corners[2].Y + corners[3].Y) / 4.0)
		} else {
			cx = image.Width() / 2
			cy = image.Height() / 2
		}
	} else {
		cx = image.Width() / 2
		cy = image.Height() / 2
	}

	// Refine center by tracing along the 4 cardinal directions.
	// Repeat up to 3 times until convergence.
	for i := 0; i < 3; i++ {
		newCX := firstDifferentCol(image, cx, cy)
		newCY := firstDifferentRow(image, cx, cy)
		if newCX == cx && newCY == cy {
			break
		}
		cx = newCX
		cy = newCY
	}

	return zxinggo.ResultPoint{X: float64(cx), Y: float64(cy)}, nil
}

// firstDifferentCol refines the horizontal center by finding the midpoint
// of the full horizontal run passing through (cx, cy) in the bullseye.
func firstDifferentCol(image *bitutil.BitMatrix, cx, cy int) int {
	w := image.Width()
	color := image.Get(cx, cy)
	left := cx
	right := cx
	for left > 0 && image.Get(left-1, cy) == color {
		left--
	}
	for right < w-1 && image.Get(right+1, cy) == color {
		right++
	}
	return (left + right) / 2
}

// firstDifferentRow refines the vertical center by finding the midpoint
// of the full vertical run passing through (cx, cy) in the bullseye.
func firstDifferentRow(image *bitutil.BitMatrix, cx, cy int) int {
	h := image.Height()
	color := image.Get(cx, cy)
	up := cy
	down := cy
	for up > 0 && image.Get(cx, up-1) == color {
		up--
	}
	for down < h-1 && image.Get(cx, down+1) == color {
		down++
	}
	return (up + down) / 2
}

// ---------------------------------------------------------------------------
// Step 2: Get the bullseye corners
// ---------------------------------------------------------------------------

// getBullseyeCorners finds the four corners of the outermost ring of the
// bullseye. It traces outward from the center along each of the four
// cardinal directions, counting black/white transitions. The transition
// count determines compact vs full-range.
//
// Returns the four corner points (in order: NE, SE, SW, NW) and a boolean
// indicating compact mode.
func getBullseyeCorners(image *bitutil.BitMatrix, center zxinggo.ResultPoint) ([4]zxinggo.ResultPoint, bool, error) {
	cx := iround(center.X)
	cy := iround(center.Y)

	// Count transitions in each of the four cardinal directions from center.
	// We count transitions outward until we have crossed through the bullseye.
	// For compact (5x5 bullseye): from center we see B(1),W(1),B(1) outward = 2 transitions per side.
	// For full (7x7 bullseye): B(1),W(1),B(1),W(1) outward = 3 transitions per side.
	// Including the outer mode-message area: compact ~4, full ~6 transitions per side.
	//
	// We trace up to 9 transitions on each side to be safe and then use the
	// count to determine compact vs full.
	rightDist, rightTrans := traceCardinal(image, cx, cy, 1, 0)
	leftDist, leftTrans := traceCardinal(image, cx, cy, -1, 0)
	downDist, downTrans := traceCardinal(image, cx, cy, 0, 1)
	upDist, upTrans := traceCardinal(image, cx, cy, 0, -1)

	// Average the transition counts from opposite directions.
	avgH := (rightTrans + leftTrans + 1) / 2
	avgV := (downTrans + upTrans + 1) / 2
	avgTrans := (avgH + avgV + 1) / 2

	// Compact bullseye (5x5): ~2 transitions per cardinal direction from center
	// Full bullseye (7x7): ~3-4 transitions per cardinal direction from center
	compact := avgTrans <= 3

	// Determine the number of ring layers in the bullseye.
	// Compact: 2 rings (5x5), half-width = 2 modules
	// Full: 3 rings (7x7), half-width = 3 modules
	var nbRings int
	if compact {
		nbRings = 2
	} else {
		nbRings = 3
	}

	// Estimate the bullseye extent in each cardinal direction.
	// The bullseye spans nbRings modules from center in each direction.
	// We use the measured pixel distances, but cap them based on the
	// expected number of ring transitions.
	halfPixRight := limitDist(rightDist, nbRings, rightTrans)
	halfPixLeft := limitDist(leftDist, nbRings, leftTrans)
	halfPixDown := limitDist(downDist, nbRings, downTrans)
	halfPixUp := limitDist(upDist, nbRings, upTrans)

	// Compute the four corner points as intersections of the axis extents.
	corners := [4]zxinggo.ResultPoint{
		{X: float64(cx) + halfPixRight, Y: float64(cy) - halfPixUp},   // NE
		{X: float64(cx) + halfPixRight, Y: float64(cy) + halfPixDown}, // SE
		{X: float64(cx) - halfPixLeft, Y: float64(cy) + halfPixDown},  // SW
		{X: float64(cx) - halfPixLeft, Y: float64(cy) - halfPixUp},    // NW
	}

	return corners, compact, nil
}

// traceCardinal traces from (cx,cy) in direction (dx,dy) and returns the
// pixel distance reached and the number of color transitions found. Tracing
// stops at the image boundary or after reaching a sufficient extent.
func traceCardinal(image *bitutil.BitMatrix, cx, cy, dx, dy int) (distPixels, transitions int) {
	w := image.Width()
	h := image.Height()
	x := cx + dx
	y := cy + dy

	if x < 0 || x >= w || y < 0 || y >= h {
		return 0, 0
	}

	currentColor := image.Get(cx, cy)
	lastTransDist := 0

	for x >= 0 && x < w && y >= 0 && y < h {
		distPixels++
		if image.Get(x, y) != currentColor {
			transitions++
			currentColor = !currentColor
			lastTransDist = distPixels
			// Stop after enough transitions for even the largest bullseye.
			if transitions >= 9 {
				break
			}
		}
		x += dx
		y += dy
	}
	// Return the distance to the last transition rather than to the edge.
	if lastTransDist > 0 {
		distPixels = lastTransDist
	}
	return distPixels, transitions
}

// limitDist estimates the pixel half-width of the bullseye in one cardinal
// direction by scaling the measured distance based on the ratio of expected
// rings to observed transitions.
func limitDist(measuredDist, nbRings, measuredTrans int) float64 {
	if measuredTrans <= 0 {
		return float64(measuredDist)
	}
	// The bullseye has nbRings transitions from center to outer edge.
	// Scale the measured distance proportionally.
	ratio := float64(nbRings) / float64(measuredTrans)
	if ratio > 1.0 {
		ratio = 1.0
	}
	return float64(measuredDist) * ratio
}

// ---------------------------------------------------------------------------
// Step 3: Extract parameters from the mode message
// ---------------------------------------------------------------------------

// extractParameters reads the orientation marks at the four corners of the
// bullseye to determine rotation, then reads and error-corrects the mode
// message to extract nbDataBlocks and nbLayers.
func extractParameters(image *bitutil.BitMatrix, corners [4]zxinggo.ResultPoint, compact, isMirror bool) (nbDataBlocks, nbLayers, shift int, err error) {
	// Determine rotation.
	shift, err = getRotation(image, corners, compact)
	if err != nil {
		return 0, 0, 0, err
	}

	// Read the mode message bits from the ring outside the bullseye.
	modeMsgBits, err := readModeMessage(image, corners, compact, isMirror, shift)
	if err != nil {
		return 0, 0, 0, err
	}

	// Reed-Solomon error correction on the mode message using GF(16).
	var numCodewords int
	var numECCodewords int
	if compact {
		numCodewords = 7   // 28 bits / 4 bits per word
		numECCodewords = 5 // 7 total - 2 data = 5 EC
	} else {
		numCodewords = 10  // 40 bits / 4 bits per word
		numECCodewords = 6 // 10 total - 4 data = 6 EC
	}

	// Convert mode message bits to 4-bit codewords.
	words := make([]int, numCodewords)
	for i := 0; i < numCodewords; i++ {
		word := 0
		for bit := 0; bit < 4; bit++ {
			idx := i*4 + bit
			if idx < len(modeMsgBits) && modeMsgBits[idx] {
				word |= 1 << uint(3-bit)
			}
		}
		words[i] = word
	}

	rsDecoder := reedsolomon.NewDecoder(reedsolomon.AztecParam)
	_, err = rsDecoder.Decode(words, numECCodewords)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("aztec detector: mode message RS correction failed: %w", err)
	}

	// Extract parameters from corrected data words.
	if compact {
		// 2 data words = 8 data bits.
		// bits[0:2]  -> nbLayers - 1
		// bits[2:8]  -> nbDataBlocks - 1
		val := (words[0] << 4) | words[1]
		nbLayers = ((val >> 6) & 0x03) + 1
		nbDataBlocks = (val & 0x3F) + 1
	} else {
		// 4 data words = 16 data bits.
		// bits[0:5]  -> nbLayers - 1
		// bits[5:16] -> nbDataBlocks - 1
		val := (words[0] << 12) | (words[1] << 8) | (words[2] << 4) | words[3]
		nbLayers = ((val >> 11) & 0x1F) + 1
		nbDataBlocks = (val & 0x07FF) + 1
	}

	return nbDataBlocks, nbLayers, shift, nil
}

// getRotation determines the rotation of the symbol (0, 1, 2, or 3
// quarter-turns) by reading the orientation marks at the four corners of
// the bullseye and matching them against the expected patterns.
func getRotation(image *bitutil.BitMatrix, corners [4]zxinggo.ResultPoint, compact bool) (int, error) {
	// Read the orientation bit pattern at each corner.
	var cornerBitLen int
	if compact {
		cornerBitLen = 3
	} else {
		cornerBitLen = 5
	}

	cornerBits := [4]int{}
	for i := 0; i < 4; i++ {
		cornerBits[i] = readCornerBits(image, corners, i, cornerBitLen)
	}

	// Determine which expected pattern set to use.
	var expectedIdx int
	if compact {
		expectedIdx = 0
	} else {
		expectedIdx = 1
	}
	expected := expectedCornerBits[expectedIdx]

	// Try each of the 4 rotations.
	bestShift := 0
	bestScore := -1
	for shift := 0; shift < 4; shift++ {
		score := 0
		for i := 0; i < 4; i++ {
			rotIdx := (i + shift) % 4
			if cornerBits[rotIdx] == expected[i] {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestShift = shift
		}
		if score == 4 {
			return shift, nil
		}
	}

	// Accept the best match even if not perfect (may have noise).
	if bestScore >= 2 {
		return bestShift, nil
	}

	return 0, fmt.Errorf("aztec detector: rotation not found (best score %d)", bestScore)
}

// readCornerBits reads the orientation bit pattern at the given corner of
// the bullseye's outer ring.
//
// Corner indices: 0=NE (top-right), 1=SE (bottom-right), 2=SW (bottom-left),
// 3=NW (top-left).
//
// The bits at each corner are read from the modules along the outer edge
// of the bullseye. For a compact symbol, these are 3 bits; for full-range,
// 5 bits.
func readCornerBits(image *bitutil.BitMatrix, corners [4]zxinggo.ResultPoint, cornerIdx, bitLen int) int {
	cx := iround(corners[cornerIdx].X)
	cy := iround(corners[cornerIdx].Y)
	w := image.Width()
	h := image.Height()

	val := 0

	// At each corner we read bitLen bits. The reading direction depends
	// on which corner:
	// NE (0): along the top edge from left to right (horizontal)
	// SE (1): along the right edge from top to bottom (vertical)
	// SW (2): along the bottom edge from right to left (horizontal, reversed)
	// NW (3): along the left edge from bottom to top (vertical, reversed)
	switch cornerIdx {
	case 0: // NE corner: read horizontally (left to right)
		for i := 0; i < bitLen; i++ {
			px := cx - bitLen/2 + i
			py := cy
			if px >= 0 && px < w && py >= 0 && py < h && image.Get(px, py) {
				val |= 1 << uint(bitLen-1-i)
			}
		}
	case 1: // SE corner: read vertically (top to bottom)
		for i := 0; i < bitLen; i++ {
			px := cx
			py := cy - bitLen/2 + i
			if px >= 0 && px < w && py >= 0 && py < h && image.Get(px, py) {
				val |= 1 << uint(bitLen-1-i)
			}
		}
	case 2: // SW corner: read horizontally (right to left)
		for i := 0; i < bitLen; i++ {
			px := cx + bitLen/2 - i
			py := cy
			if px >= 0 && px < w && py >= 0 && py < h && image.Get(px, py) {
				val |= 1 << uint(bitLen-1-i)
			}
		}
	case 3: // NW corner: read vertically (bottom to top)
		for i := 0; i < bitLen; i++ {
			px := cx
			py := cy + bitLen/2 - i
			if px >= 0 && px < w && py >= 0 && py < h && image.Get(px, py) {
				val |= 1 << uint(bitLen-1-i)
			}
		}
	}
	return val
}

// readModeMessage reads the mode message bits from the ring of modules just
// outside the bullseye. The bits are read going clockwise starting from the
// side determined by the rotation shift.
//
// Compact: 28 bits in a ring of 7 modules per side (4 sides * 7 = 28).
// Full: 40 bits in a ring of 10 modules per side (4 sides * 10 = 40).
func readModeMessage(image *bitutil.BitMatrix, corners [4]zxinggo.ResultPoint, compact, isMirror bool, shift int) ([]bool, error) {
	var sideLen int
	var totalBits int
	if compact {
		sideLen = 7
		totalBits = 28
	} else {
		sideLen = 10
		totalBits = 40
	}

	// The mode message ring is located 1 module outside the bullseye outer ring.
	// We sample along each of the 4 sides of this ring.
	//
	// Side ordering (clockwise, unrotated):
	//   side 0 = top:    from NW corner toward NE corner
	//   side 1 = right:  from NE corner toward SE corner
	//   side 2 = bottom: from SE corner toward SW corner
	//   side 3 = left:   from SW corner toward NW corner
	//
	// Corners array: 0=NE, 1=SE, 2=SW, 3=NW

	// Map each side to its start and end corner indices and the
	// perpendicular outward offset direction.
	type sideInfo struct {
		startCorner int
		endCorner   int
		offX, offY  float64 // outward offset direction (unit)
	}

	sides := [4]sideInfo{
		{startCorner: 3, endCorner: 0, offX: 0, offY: -1}, // top side
		{startCorner: 0, endCorner: 1, offX: 1, offY: 0},  // right side
		{startCorner: 1, endCorner: 2, offX: 0, offY: 1},  // bottom side
		{startCorner: 2, endCorner: 3, offX: -1, offY: 0}, // left side
	}

	// Estimate the module size from the bullseye corner distances.
	centerX := (corners[0].X + corners[1].X + corners[2].X + corners[3].X) / 4.0
	centerY := (corners[0].Y + corners[1].Y + corners[2].Y + corners[3].Y) / 4.0

	// Distance from center to corners along an axis gives half the bullseye size.
	halfSizeX := (math.Abs(corners[0].X-centerX) + math.Abs(corners[2].X-centerX)) / 2.0
	halfSizeY := (math.Abs(corners[1].Y-centerY) + math.Abs(corners[3].Y-centerY)) / 2.0

	var bullseyeHalf float64
	if compact {
		bullseyeHalf = 3.5 // compact bullseye outer ring is 7x7, half is 3.5
	} else {
		bullseyeHalf = 5.5 // full bullseye outer ring is 11x11, half is 5.5
	}

	moduleX := halfSizeX / bullseyeHalf
	moduleY := halfSizeY / bullseyeHalf
	if moduleX <= 0 {
		moduleX = 1
	}
	if moduleY <= 0 {
		moduleY = 1
	}

	// The mode message ring is 1 module outside the bullseye.
	// We offset sampling positions outward by moduleSize and then
	// inward by half a module to hit module centers.
	offsetDist := 1.5 // 1 module outside + 0.5 for center of that module

	bits := make([]bool, totalBits)
	bitIdx := 0

	for side := 0; side < 4; side++ {
		actualSide := (side + shift) % 4
		si := sides[actualSide]

		sx := corners[si.startCorner].X
		sy := corners[si.startCorner].Y
		ex := corners[si.endCorner].X
		ey := corners[si.endCorner].Y

		// Apply outward offset to both start and end points.
		oX := si.offX * offsetDist * moduleX
		oY := si.offY * offsetDist * moduleY
		sx += oX
		sy += oY
		ex += oX
		ey += oY

		// Sample sideLen modules along this side.
		for j := 0; j < sideLen; j++ {
			t := (float64(j) + 0.5) / float64(sideLen)
			px := iround(sx + t*(ex-sx))
			py := iround(sy + t*(ey-sy))

			w := image.Width()
			h := image.Height()
			if px >= 0 && px < w && py >= 0 && py < h {
				if isMirror {
					bits[totalBits-1-bitIdx] = image.Get(px, py)
				} else {
					bits[bitIdx] = image.Get(px, py)
				}
			}
			bitIdx++
		}
	}

	return bits, nil
}

// ---------------------------------------------------------------------------
// Step 4: Sample the grid
// ---------------------------------------------------------------------------

// sampleGrid performs a perspective transform and samples the full Aztec
// symbol grid.
//
// The four bullseye corner points define the coordinate system. We expand
// them outward to encompass all data layers and use the perspective transform
// to resample the image into a regular grid.
func sampleGrid(image *bitutil.BitMatrix,
	cornerNE, cornerSE, cornerSW, cornerNW zxinggo.ResultPoint,
	compact bool, nbLayers, shift int,
) (*bitutil.BitMatrix, []zxinggo.ResultPoint, error) {

	dimension := getDimension(compact, nbLayers)
	if dimension <= 0 {
		return nil, nil, fmt.Errorf("aztec detector: invalid dimension %d", dimension)
	}

	// The center of the symbol in image coordinates.
	centerX := (cornerNE.X + cornerSE.X + cornerSW.X + cornerNW.X) / 4.0
	centerY := (cornerNE.Y + cornerSE.Y + cornerSW.Y + cornerNW.Y) / 4.0

	// The bullseye outer ring half-size in modules.
	var bullseyeHalf float64
	if compact {
		bullseyeHalf = 3.5 // outer ring of compact bullseye is 7x7
	} else {
		bullseyeHalf = 5.5 // outer ring of full bullseye is 11x11
	}

	// Estimate module size from the bullseye corners.
	avgDist := 0.0
	for _, c := range []zxinggo.ResultPoint{cornerNE, cornerSE, cornerSW, cornerNW} {
		dx := c.X - centerX
		dy := c.Y - centerY
		avgDist += math.Sqrt(dx*dx + dy*dy)
	}
	avgDist /= 4.0

	// The bullseye corners are at a diagonal distance of bullseyeHalf * sqrt(2)
	// from center.
	moduleSize := avgDist / (bullseyeHalf * math.Sqrt2)
	if moduleSize <= 0 {
		return nil, nil, fmt.Errorf("aztec detector: invalid module size")
	}

	// Compute the four corners of the full symbol.
	halfDim := float64(dimension) / 2.0
	scaleFactor := halfDim * moduleSize / avgDist

	topRight := zxinggo.ResultPoint{
		X: centerX + (cornerNE.X-centerX)*scaleFactor,
		Y: centerY + (cornerNE.Y-centerY)*scaleFactor,
	}
	bottomRight := zxinggo.ResultPoint{
		X: centerX + (cornerSE.X-centerX)*scaleFactor,
		Y: centerY + (cornerSE.Y-centerY)*scaleFactor,
	}
	bottomLeft := zxinggo.ResultPoint{
		X: centerX + (cornerSW.X-centerX)*scaleFactor,
		Y: centerY + (cornerSW.Y-centerY)*scaleFactor,
	}
	topLeft := zxinggo.ResultPoint{
		X: centerX + (cornerNW.X-centerX)*scaleFactor,
		Y: centerY + (cornerNW.Y-centerY)*scaleFactor,
	}

	// Build the perspective transform.
	// Destination coordinates: the grid corners (with 0.5 offset for module centers).
	dimF := float64(dimension)
	xform := transform.QuadrilateralToQuadrilateral(
		0.5, 0.5,
		dimF-0.5, 0.5,
		dimF-0.5, dimF-0.5,
		0.5, dimF-0.5,
		topLeft.X, topLeft.Y,
		topRight.X, topRight.Y,
		bottomRight.X, bottomRight.Y,
		bottomLeft.X, bottomLeft.Y,
	)

	sampler := &transform.DefaultGridSampler{}
	bits, err := sampler.SampleGridTransform(image, dimension, dimension, xform)
	if err != nil {
		return nil, nil, fmt.Errorf("aztec detector: grid sampling failed: %w", err)
	}

	// Correct for symbol rotation. The data was sampled assuming rotation 0.
	// If the symbol is rotated, we rotate the sampled grid accordingly.
	if shift > 0 {
		bits.Rotate(shift * 90)
	}

	return bits, []zxinggo.ResultPoint{topLeft, topRight, bottomRight, bottomLeft}, nil
}

// getDimension returns the side length (in modules) of the full Aztec symbol
// including all data layers and any reference grid lines.
//
//	Compact: dimension = 4 * nbLayers + 11
//	Full:    dimension = 4 * nbLayers + 14 + 2 * numRefGrids
//
// where numRefGrids = max(0, floor(((4*nbLayers+14)/2 - 13) / 15)).
// The reference grid adds alignment lines every 16 modules from center.
func getDimension(compact bool, nbLayers int) int {
	if compact {
		return 4*nbLayers + 11
	}
	d := 4*nbLayers + 14
	numRefGrids := (d/2 - 13) / 15
	if numRefGrids < 0 {
		numRefGrids = 0
	}
	return d + 2*numRefGrids
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
		found          bool
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

func iround(f float64) int {
	return int(math.Round(f))
}
