// Package detector implements QR code detection in binary images.
package detector

import (
	"math"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
	"github.com/ericlevine/zxinggo/internal"
	"github.com/ericlevine/zxinggo/qrcode/decoder"
	"github.com/ericlevine/zxinggo/transform"
)

// FinderPattern represents a finder pattern with position and module size.
type FinderPattern struct {
	X, Y              float64
	EstimatedModuleSize float64
	Count             int
}

// FinderPatternInfo holds the three finder patterns.
type FinderPatternInfo struct {
	BottomLeft, TopLeft, TopRight *FinderPattern
}

// AlignmentPattern represents an alignment pattern.
type AlignmentPattern struct {
	X, Y              float64
	EstimatedModuleSize float64
}

// Detector detects QR codes in binary images.
type Detector struct {
	image *bitutil.BitMatrix
}

// NewDetector creates a new Detector for the given image.
func NewDetector(image *bitutil.BitMatrix) *Detector {
	return &Detector{image: image}
}

// Detect detects a QR code and returns the sampled bit matrix and corner points.
func (d *Detector) Detect(pureBarcode bool) (*internal.DetectorResult, error) {
	info, err := d.findFinderPatterns(pureBarcode)
	if err != nil {
		return nil, err
	}
	return d.processFinderPatternInfo(info)
}

func (d *Detector) processFinderPatternInfo(info *FinderPatternInfo) (*internal.DetectorResult, error) {
	topLeft := info.TopLeft
	topRight := info.TopRight
	bottomLeft := info.BottomLeft

	moduleSize := d.calculateModuleSize(topLeft, topRight, bottomLeft)
	if moduleSize < 1.0 {
		return nil, zxinggo.ErrNotFound
	}

	dimension, err := computeDimension(topLeft, topRight, bottomLeft, moduleSize)
	if err != nil {
		return nil, err
	}

	provisionalVersion, err := decoder.GetProvisionalVersionForDimension(dimension)
	if err != nil {
		return nil, err
	}

	// Look for alignment pattern
	var alignmentPattern *AlignmentPattern
	if len(provisionalVersion.AlignmentPatternCenters) > 0 {
		bottomRightX := topRight.X - topLeft.X + bottomLeft.X
		bottomRightY := topRight.Y - topLeft.Y + bottomLeft.Y

		correctionToTopLeft := 1.0 - 3.0/float64(dimension-7)
		estAlignmentX := int(topLeft.X + correctionToTopLeft*(bottomRightX-topLeft.X))
		estAlignmentY := int(topLeft.Y + correctionToTopLeft*(bottomRightY-topLeft.Y))

		for i := 4; i <= 16; i <<= 1 {
			ap := d.findAlignmentInRegion(moduleSize, estAlignmentX, estAlignmentY, float64(i))
			if ap != nil {
				alignmentPattern = ap
				break
			}
		}
	}

	xform := createTransform(topLeft, topRight, bottomLeft, alignmentPattern, dimension)
	sampler := &transform.DefaultGridSampler{}
	bits, err := sampler.SampleGridTransform(d.image, dimension, dimension, xform)
	if err != nil {
		return nil, err
	}

	var points []internal.ResultPoint
	if alignmentPattern != nil {
		points = []internal.ResultPoint{
			{X: bottomLeft.X, Y: bottomLeft.Y},
			{X: topLeft.X, Y: topLeft.Y},
			{X: topRight.X, Y: topRight.Y},
			{X: alignmentPattern.X, Y: alignmentPattern.Y},
		}
	} else {
		points = []internal.ResultPoint{
			{X: bottomLeft.X, Y: bottomLeft.Y},
			{X: topLeft.X, Y: topLeft.Y},
			{X: topRight.X, Y: topRight.Y},
		}
	}

	return internal.NewDetectorResult(bits, points), nil
}

func computeDimension(topLeft, topRight, bottomLeft *FinderPattern, moduleSize float64) (int, error) {
	tltrDist := distance(topLeft, topRight)
	tlblDist := distance(topLeft, bottomLeft)
	dimension := int(math.Round((tltrDist/moduleSize+tlblDist/moduleSize)/2.0)) + 7
	switch dimension % 4 {
	case 0:
		dimension++
	case 2:
		dimension--
	case 3:
		return 0, zxinggo.ErrNotFound
	}
	return dimension, nil
}

func distance(a, b *FinderPattern) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return math.Sqrt(dx*dx + dy*dy)
}

func (d *Detector) calculateModuleSize(topLeft, topRight, bottomLeft *FinderPattern) float64 {
	return (d.calculateModuleSizeOneWay(topLeft, topRight) +
		d.calculateModuleSizeOneWay(topLeft, bottomLeft)) / 2.0
}

func (d *Detector) calculateModuleSizeOneWay(pattern, otherPattern *FinderPattern) float64 {
	moduleSizeEst1 := d.sizeOfBlackWhiteBlackRunBothWays(
		int(pattern.X), int(pattern.Y), int(otherPattern.X), int(otherPattern.Y))
	moduleSizeEst2 := d.sizeOfBlackWhiteBlackRunBothWays(
		int(otherPattern.X), int(otherPattern.Y), int(pattern.X), int(pattern.Y))
	if math.IsNaN(moduleSizeEst1) {
		return moduleSizeEst2 / 7.0
	}
	if math.IsNaN(moduleSizeEst2) {
		return moduleSizeEst1 / 7.0
	}
	return (moduleSizeEst1 + moduleSizeEst2) / 14.0
}

func (d *Detector) sizeOfBlackWhiteBlackRunBothWays(fromX, fromY, toX, toY int) float64 {
	result := d.sizeOfBlackWhiteBlackRun(fromX, fromY, toX, toY)

	// Now in the other direction
	scale := 1.0
	otherToX := fromX - (toX - fromX)
	if otherToX < 0 {
		scale = float64(fromX) / float64(fromX-otherToX)
		otherToX = 0
	} else if otherToX >= d.image.Width() {
		scale = float64(d.image.Width()-1-fromX) / float64(otherToX-fromX)
		otherToX = d.image.Width() - 1
	}
	otherToY := int(float64(fromY) - float64(toY-fromY)*scale)

	scale = 1.0
	if otherToY < 0 {
		scale = float64(fromY) / float64(fromY-otherToY)
		otherToY = 0
	} else if otherToY >= d.image.Height() {
		scale = float64(d.image.Height()-1-fromY) / float64(otherToY-fromY)
		otherToY = d.image.Height() - 1
	}
	otherToX = int(float64(fromX) + float64(otherToX-fromX)*scale)

	result += d.sizeOfBlackWhiteBlackRun(fromX, fromY, otherToX, otherToY)
	return result - 1.0
}

func (d *Detector) sizeOfBlackWhiteBlackRun(fromX, fromY, toX, toY int) float64 {
	steep := false
	dx := abs(toX - fromX)
	dy := abs(toY - fromY)
	if dy > dx {
		steep = true
		fromX, fromY = fromY, fromX
		toX, toY = toY, toX
		dx, dy = dy, dx
	}

	xstep := 1
	if fromX > toX {
		xstep = -1
	}
	ystep := 1
	if fromY > toY {
		ystep = -1
	}

	state := 0 // looking for black, then white, then black
	xLimit := toX + xstep
	e := -dx
	for x := fromX; x != xLimit; x += xstep {
		realX := x
		realY := fromY + (x-fromX)*dy/dx*ystep
		if steep {
			realX = fromY + (x-fromX)*dy/dx*ystep
			realY = x
		}

		if realX < 0 || realX >= d.image.Width() || realY < 0 || realY >= d.image.Height() {
			break
		}

		if state == 1 == d.image.Get(realX, realY) { // black in state 1, or white otherwise
			if state == 2 {
				return math.Sqrt(float64((x-fromX)*(x-fromX)) + float64(((x-fromX)*dy/dx)*((x-fromX)*dy/dx)))
			}
			state++
		}
		e += 2 * dy
		if e > 0 {
			if fromY == toY {
				break
			}
			fromY += ystep
			e -= 2 * dx
		}
	}

	if state == 2 {
		return math.Sqrt(float64((toX-fromX+xstep)*(toX-fromX+xstep)) + float64((toY-fromY)*(toY-fromY)))
	}
	return math.NaN()
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func createTransform(topLeft, topRight, bottomLeft *FinderPattern, alignmentPattern *AlignmentPattern, dimension int) *transform.PerspectiveTransform {
	dimMinusThree := float64(dimension) - 3.5
	var bottomRightX, bottomRightY, sourceBottomRightX, sourceBottomRightY float64

	if alignmentPattern != nil {
		bottomRightX = alignmentPattern.X
		bottomRightY = alignmentPattern.Y
		sourceBottomRightX = dimMinusThree - 3.0
		sourceBottomRightY = sourceBottomRightX
	} else {
		bottomRightX = (topRight.X - topLeft.X) + bottomLeft.X
		bottomRightY = (topRight.Y - topLeft.Y) + bottomLeft.Y
		sourceBottomRightX = dimMinusThree
		sourceBottomRightY = dimMinusThree
	}

	return transform.QuadrilateralToQuadrilateral(
		3.5, 3.5, dimMinusThree, 3.5, sourceBottomRightX, sourceBottomRightY, 3.5, dimMinusThree,
		topLeft.X, topLeft.Y, topRight.X, topRight.Y, bottomRightX, bottomRightY, bottomLeft.X, bottomLeft.Y,
	)
}

func (d *Detector) findAlignmentInRegion(overallEstModuleSize float64, estAlignmentX, estAlignmentY int, allowanceFactor float64) *AlignmentPattern {
	allowance := int(allowanceFactor * overallEstModuleSize)
	alignmentAreaLeftX := max(0, estAlignmentX-allowance)
	alignmentAreaTopY := max(0, estAlignmentY-allowance)
	alignmentAreaRightX := min(d.image.Width()-1, estAlignmentX+allowance)
	alignmentAreaBottomY := min(d.image.Height()-1, estAlignmentY+allowance)

	searchWidth := alignmentAreaRightX - alignmentAreaLeftX
	searchHeight := alignmentAreaBottomY - alignmentAreaTopY
	if searchWidth < 0 || searchHeight < 0 {
		return nil
	}

	return d.findAlignmentPattern(alignmentAreaLeftX, alignmentAreaTopY, searchWidth, searchHeight, overallEstModuleSize)
}

func (d *Detector) findAlignmentPattern(startX, startY, width, height int, moduleSize float64) *AlignmentPattern {
	// Simple alignment pattern finder: search for a 1:1:1 black/white/black pattern
	middleY := startY + height/2
	for dy := 0; dy < height; dy++ {
		y := middleY
		if dy%2 == 0 {
			y += (dy + 1) / 2
		} else {
			y -= (dy + 1) / 2
		}
		if y < startY || y >= startY+height {
			continue
		}

		stateCount := [3]int{}
		state := 0
		for x := startX; x < startX+width; x++ {
			if d.image.Get(x, y) {
				if state == 1 {
					state = 2
				}
				stateCount[state]++
			} else {
				if state == 2 {
					if foundAlignmentPattern(stateCount, moduleSize) {
						centerX := float64(x) - float64(stateCount[2]) - float64(stateCount[1])/2.0
						centerY := d.crossCheckVerticalAlignment(int(centerX), y, 2*stateCount[1], moduleSize)
						if !math.IsNaN(centerY) {
							return &AlignmentPattern{X: centerX, Y: centerY, EstimatedModuleSize: moduleSize}
						}
					}
					stateCount[0] = stateCount[2]
					stateCount[1] = 1
					stateCount[2] = 0
					state = 1
				} else {
					state++
					stateCount[state]++
				}
			}
		}
		if state == 2 && foundAlignmentPattern(stateCount, moduleSize) {
			centerX := float64(startX+width) - float64(stateCount[2]) - float64(stateCount[1])/2.0
			centerY := d.crossCheckVerticalAlignment(int(centerX), y, 2*stateCount[1], moduleSize)
			if !math.IsNaN(centerY) {
				return &AlignmentPattern{X: centerX, Y: centerY, EstimatedModuleSize: moduleSize}
			}
		}
	}
	return nil
}

func foundAlignmentPattern(stateCount [3]int, moduleSize float64) bool {
	maxVariance := moduleSize / 2.0
	for _, count := range stateCount {
		if math.Abs(float64(count)-moduleSize) >= maxVariance {
			return false
		}
	}
	return true
}

func (d *Detector) crossCheckVerticalAlignment(centerX, startY, maxCount int, moduleSize float64) float64 {
	maxY := d.image.Height()
	stateCount := [3]int{}

	y := startY
	for y >= 0 && d.image.Get(centerX, y) && stateCount[1] <= maxCount {
		stateCount[1]++
		y--
	}
	if y < 0 || stateCount[1] > maxCount {
		return math.NaN()
	}
	for y >= 0 && !d.image.Get(centerX, y) && stateCount[0] <= maxCount {
		stateCount[0]++
		y--
	}
	if stateCount[0] > maxCount {
		return math.NaN()
	}

	y = startY + 1
	for y < maxY && d.image.Get(centerX, y) && stateCount[1] <= maxCount {
		stateCount[1]++
		y++
	}
	if y == maxY || stateCount[1] > maxCount {
		return math.NaN()
	}
	for y < maxY && !d.image.Get(centerX, y) && stateCount[2] <= maxCount {
		stateCount[2]++
		y++
	}
	if stateCount[2] > maxCount {
		return math.NaN()
	}

	total := stateCount[0] + stateCount[1] + stateCount[2]
	if 5*abs(total-int(moduleSize*3)) >= int(moduleSize*3) {
		return math.NaN()
	}
	return float64(y-stateCount[2]) - float64(stateCount[1])/2.0
}

// findFinderPatterns is a simplified finder pattern detection.
func (d *Detector) findFinderPatterns(pureBarcode bool) (*FinderPatternInfo, error) {
	height := d.image.Height()
	width := d.image.Width()

	skip := (3 * height) / (4 * 97)
	if skip < 3 {
		skip = 3
	}
	if pureBarcode {
		skip = 1
	}

	var possibleCenters []*FinderPattern

	for y := skip - 1; y < height; y += skip {
		stateCount := [5]int{}
		state := 0
		for x := 0; x < width; x++ {
			if d.image.Get(x, y) { // black pixel
				if state&1 == 1 { // was counting white
					state++
				}
				stateCount[state]++
			} else { // white pixel
				if state&1 == 0 { // was counting black
					if state == 4 {
						// We've found a complete 5-state pattern
						if foundFinderPattern(stateCount) {
							confirmed := d.handlePossibleCenter(stateCount, y, x, &possibleCenters)
							if confirmed {
								if len(possibleCenters) >= 3 {
									best := selectBestPatterns(possibleCenters)
									if best != nil {
										return orderFinderPatterns(best), nil
									}
								}
							}
						}
						stateCount[0] = stateCount[2]
						stateCount[1] = stateCount[3]
						stateCount[2] = stateCount[4]
						stateCount[3] = 1
						stateCount[4] = 0
						state = 3
					} else {
						state++
						stateCount[state]++
					}
				} else {
					stateCount[state]++
				}
			}
		}
		if state == 4 && foundFinderPattern(stateCount) {
			d.handlePossibleCenter(stateCount, y, width, &possibleCenters)
		}
	}

	best := selectBestPatterns(possibleCenters)
	if best == nil {
		return nil, zxinggo.ErrNotFound
	}
	return orderFinderPatterns(best), nil
}

func foundFinderPattern(stateCount [5]int) bool {
	totalModuleSize := 0
	for i := 0; i < 5; i++ {
		count := stateCount[i]
		if count == 0 {
			return false
		}
		totalModuleSize += count
	}
	if totalModuleSize < 7 {
		return false
	}
	moduleSize := float64(totalModuleSize) / 7.0
	maxVariance := moduleSize / 2.0
	return math.Abs(moduleSize-float64(stateCount[0])) < maxVariance &&
		math.Abs(moduleSize-float64(stateCount[1])) < maxVariance &&
		math.Abs(3*moduleSize-float64(stateCount[2])) < 3*maxVariance &&
		math.Abs(moduleSize-float64(stateCount[3])) < maxVariance &&
		math.Abs(moduleSize-float64(stateCount[4])) < maxVariance
}

func (d *Detector) handlePossibleCenter(stateCount [5]int, i, j int, possibleCenters *[]*FinderPattern) bool {
	total := stateCount[0] + stateCount[1] + stateCount[2] + stateCount[3] + stateCount[4]
	centerJ := float64(j) - float64(stateCount[4]+stateCount[3]) - float64(stateCount[2])/2.0
	centerI := d.crossCheckVerticalFinder(i, int(centerJ), stateCount[2], total)
	if math.IsNaN(centerI) {
		return false
	}

	estModuleSize := float64(total) / 7.0
	for idx, center := range *possibleCenters {
		if center.aboutEquals(estModuleSize, centerI, centerJ) {
			(*possibleCenters)[idx] = center.combineEstimate(centerI, centerJ, estModuleSize)
			return true
		}
	}
	*possibleCenters = append(*possibleCenters, &FinderPattern{
		X: centerJ, Y: centerI, EstimatedModuleSize: estModuleSize, Count: 1,
	})
	return false
}

func (fp *FinderPattern) aboutEquals(moduleSize, i, j float64) bool {
	if math.Abs(i-fp.Y) <= moduleSize && math.Abs(j-fp.X) <= moduleSize {
		moduleSizeDiff := math.Abs(moduleSize - fp.EstimatedModuleSize)
		return moduleSizeDiff <= 1.0 || moduleSizeDiff <= fp.EstimatedModuleSize
	}
	return false
}

func (fp *FinderPattern) combineEstimate(i, j, newModuleSize float64) *FinderPattern {
	combinedCount := fp.Count + 1
	combinedX := (float64(fp.Count)*fp.X + j) / float64(combinedCount)
	combinedY := (float64(fp.Count)*fp.Y + i) / float64(combinedCount)
	combinedModuleSize := (float64(fp.Count)*fp.EstimatedModuleSize + newModuleSize) / float64(combinedCount)
	return &FinderPattern{X: combinedX, Y: combinedY, EstimatedModuleSize: combinedModuleSize, Count: combinedCount}
}

func (d *Detector) crossCheckVerticalFinder(startI, centerJ, maxCount, originalTotal int) float64 {
	maxI := d.image.Height()
	stateCount := [5]int{}

	i := startI
	for i >= 0 && d.image.Get(centerJ, i) {
		stateCount[2]++
		i--
	}
	if i < 0 {
		return math.NaN()
	}
	for i >= 0 && !d.image.Get(centerJ, i) && stateCount[1] <= maxCount {
		stateCount[1]++
		i--
	}
	if i < 0 || stateCount[1] > maxCount {
		return math.NaN()
	}
	for i >= 0 && d.image.Get(centerJ, i) && stateCount[0] <= maxCount {
		stateCount[0]++
		i--
	}
	if stateCount[0] > maxCount {
		return math.NaN()
	}

	i = startI + 1
	for i < maxI && d.image.Get(centerJ, i) {
		stateCount[2]++
		i++
	}
	if i == maxI {
		return math.NaN()
	}
	for i < maxI && !d.image.Get(centerJ, i) && stateCount[3] <= maxCount {
		stateCount[3]++
		i++
	}
	if i == maxI || stateCount[3] > maxCount {
		return math.NaN()
	}
	for i < maxI && d.image.Get(centerJ, i) && stateCount[4] <= maxCount {
		stateCount[4]++
		i++
	}
	if stateCount[4] > maxCount {
		return math.NaN()
	}

	totalNew := stateCount[0] + stateCount[1] + stateCount[2] + stateCount[3] + stateCount[4]
	if 5*abs(totalNew-originalTotal) >= 2*originalTotal {
		return math.NaN()
	}

	if foundFinderPattern(stateCount) {
		return float64(i-stateCount[4]-stateCount[3]) - float64(stateCount[2])/2.0
	}
	return math.NaN()
}

func selectBestPatterns(possibleCenters []*FinderPattern) []*FinderPattern {
	if len(possibleCenters) < 3 {
		return nil
	}

	// Select the 3 patterns with highest count
	if len(possibleCenters) == 3 {
		return possibleCenters
	}

	// Find average module size
	totalModuleSize := 0.0
	for _, center := range possibleCenters {
		totalModuleSize += center.EstimatedModuleSize
	}
	average := totalModuleSize / float64(len(possibleCenters))

	// Filter out patterns with module size too different from average
	var filtered []*FinderPattern
	for _, center := range possibleCenters {
		if math.Abs(center.EstimatedModuleSize-average) <= 0.5*average {
			filtered = append(filtered, center)
		}
	}

	if len(filtered) < 3 {
		filtered = possibleCenters
	}
	if len(filtered) < 3 {
		return nil
	}

	// Return the 3 with highest count
	// Simple selection: just take the first 3 with count >= 2, or all if not enough
	var best []*FinderPattern
	for _, c := range filtered {
		if c.Count >= 2 {
			best = append(best, c)
		}
	}
	if len(best) >= 3 {
		return best[:3]
	}
	return filtered[:3]
}

func orderFinderPatterns(patterns []*FinderPattern) *FinderPatternInfo {
	// Order as bottom-left, top-left, top-right
	d01 := distanceFP(patterns[0], patterns[1])
	d12 := distanceFP(patterns[1], patterns[2])
	d02 := distanceFP(patterns[0], patterns[2])

	var pointA, pointB, pointC *FinderPattern
	if d12 >= d01 && d12 >= d02 {
		pointA = patterns[0]
		pointB = patterns[1]
		pointC = patterns[2]
	} else if d02 >= d01 && d02 >= d12 {
		pointA = patterns[1]
		pointB = patterns[0]
		pointC = patterns[2]
	} else {
		pointA = patterns[2]
		pointB = patterns[0]
		pointC = patterns[1]
	}

	// Use cross product to ensure correct order
	cross := (pointB.X-pointA.X)*(pointC.Y-pointA.Y) - (pointB.Y-pointA.Y)*(pointC.X-pointA.X)
	if cross < 0 {
		pointB, pointC = pointC, pointB
	}

	return &FinderPatternInfo{
		BottomLeft: pointB,
		TopLeft:    pointA,
		TopRight:   pointC,
	}
}

func distanceFP(a, b *FinderPattern) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return math.Sqrt(dx*dx + dy*dy)
}
