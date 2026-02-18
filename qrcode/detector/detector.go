// Package detector implements QR code detection in binary images.
package detector

import (
	"math"
	"sort"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
	"github.com/ericlevine/zxinggo/internal"
	"github.com/ericlevine/zxinggo/qrcode/decoder"
	"github.com/ericlevine/zxinggo/transform"
)

const (
	centerQuorum = 2
	minSkip      = 3
	maxModules   = 97
)

// FinderPattern represents a finder pattern with position and module size.
type FinderPattern struct {
	X, Y                float64
	EstimatedModuleSize float64
	Count               int
}

// FinderPatternInfo holds the three finder patterns.
type FinderPatternInfo struct {
	BottomLeft, TopLeft, TopRight *FinderPattern
}

// AlignmentPattern represents an alignment pattern.
type AlignmentPattern struct {
	X, Y                float64
	EstimatedModuleSize float64
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

func (ap *AlignmentPattern) aboutEquals(moduleSize, i, j float64) bool {
	if math.Abs(i-ap.Y) <= moduleSize && math.Abs(j-ap.X) <= moduleSize {
		moduleSizeDiff := math.Abs(moduleSize - ap.EstimatedModuleSize)
		return moduleSizeDiff <= 1.0 || moduleSizeDiff <= ap.EstimatedModuleSize
	}
	return false
}

func (ap *AlignmentPattern) combineEstimate(i, j, newModuleSize float64) *AlignmentPattern {
	combinedX := (ap.X + j) / 2.0
	combinedY := (ap.Y + i) / 2.0
	combinedModuleSize := (ap.EstimatedModuleSize + newModuleSize) / 2.0
	return &AlignmentPattern{X: combinedX, Y: combinedY, EstimatedModuleSize: combinedModuleSize}
}

// --- FinderPatternFinder ---

type finderPatternFinder struct {
	image              *bitutil.BitMatrix
	possibleCenters    []*FinderPattern
	hasSkipped         bool
	crossCheckStateCount [5]int
}

func (f *finderPatternFinder) getCrossCheckStateCount() *[5]int {
	f.crossCheckStateCount = [5]int{}
	return &f.crossCheckStateCount
}

func (f *finderPatternFinder) find(tryHarder bool) (*FinderPatternInfo, error) {
	maxI := f.image.Height()
	maxJ := f.image.Width()

	iSkip := (3 * maxI) / (4 * maxModules)
	if iSkip < minSkip || tryHarder {
		iSkip = minSkip
	}

	done := false
	stateCount := [5]int{}
	for i := iSkip - 1; i < maxI && !done; i += iSkip {
		stateCount = [5]int{}
		currentState := 0
		for j := 0; j < maxJ; j++ {
			if f.image.Get(j, i) {
				// Black pixel
				if currentState&1 == 1 { // was counting white
					currentState++
				}
				stateCount[currentState]++
			} else {
				// White pixel
				if currentState&1 == 0 { // was counting black
					if currentState == 4 {
						if foundPatternCross(stateCount) {
							confirmed := f.handlePossibleCenter(stateCount, i, j)
							if confirmed {
								iSkip = 2
								if f.hasSkipped {
									done = f.haveMultiplyConfirmedCenters()
								} else {
									rowSkip := f.findRowSkip()
									if rowSkip > stateCount[2] {
										i += rowSkip - stateCount[2] - iSkip
										j = maxJ - 1
									}
								}
								currentState = 0
								stateCount = [5]int{}
							} else {
								doShiftCounts2(&stateCount)
								currentState = 3
								continue
							}
						} else {
							doShiftCounts2(&stateCount)
							currentState = 3
						}
					} else {
						currentState++
						stateCount[currentState]++
					}
				} else {
					stateCount[currentState]++
				}
			}
		}
		if foundPatternCross(stateCount) {
			confirmed := f.handlePossibleCenter(stateCount, i, maxJ)
			if confirmed {
				iSkip = stateCount[0]
				if f.hasSkipped {
					done = f.haveMultiplyConfirmedCenters()
				}
			}
		}
	}

	patterns, err := f.selectBestPatterns()
	if err != nil {
		return nil, err
	}
	return orderFinderPatterns(patterns), nil
}

func foundPatternCross(stateCount [5]int) bool {
	totalModuleSize := 0
	for i := 0; i < 5; i++ {
		if stateCount[i] == 0 {
			return false
		}
		totalModuleSize += stateCount[i]
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

func foundPatternDiagonal(stateCount [5]int) bool {
	totalModuleSize := 0
	for i := 0; i < 5; i++ {
		if stateCount[i] == 0 {
			return false
		}
		totalModuleSize += stateCount[i]
	}
	if totalModuleSize < 7 {
		return false
	}
	moduleSize := float64(totalModuleSize) / 7.0
	maxVariance := moduleSize / 1.333
	return math.Abs(moduleSize-float64(stateCount[0])) < maxVariance &&
		math.Abs(moduleSize-float64(stateCount[1])) < maxVariance &&
		math.Abs(3*moduleSize-float64(stateCount[2])) < 3*maxVariance &&
		math.Abs(moduleSize-float64(stateCount[3])) < maxVariance &&
		math.Abs(moduleSize-float64(stateCount[4])) < maxVariance
}

func doShiftCounts2(stateCount *[5]int) {
	stateCount[0] = stateCount[2]
	stateCount[1] = stateCount[3]
	stateCount[2] = stateCount[4]
	stateCount[3] = 1
	stateCount[4] = 0
}

func centerFromEnd(stateCount [5]int, end int) float64 {
	return float64(end-stateCount[4]-stateCount[3]) - float64(stateCount[2])/2.0
}

func (f *finderPatternFinder) crossCheckDiagonal(centerI, centerJ int) bool {
	sc := f.getCrossCheckStateCount()

	i := 0
	for centerI >= i && centerJ >= i && f.image.Get(centerJ-i, centerI-i) {
		sc[2]++
		i++
	}
	if sc[2] == 0 {
		return false
	}
	for centerI >= i && centerJ >= i && !f.image.Get(centerJ-i, centerI-i) {
		sc[1]++
		i++
	}
	if sc[1] == 0 {
		return false
	}
	for centerI >= i && centerJ >= i && f.image.Get(centerJ-i, centerI-i) {
		sc[0]++
		i++
	}
	if sc[0] == 0 {
		return false
	}

	maxI := f.image.Height()
	maxJ := f.image.Width()

	i = 1
	for centerI+i < maxI && centerJ+i < maxJ && f.image.Get(centerJ+i, centerI+i) {
		sc[2]++
		i++
	}
	for centerI+i < maxI && centerJ+i < maxJ && !f.image.Get(centerJ+i, centerI+i) {
		sc[3]++
		i++
	}
	if sc[3] == 0 {
		return false
	}
	for centerI+i < maxI && centerJ+i < maxJ && f.image.Get(centerJ+i, centerI+i) {
		sc[4]++
		i++
	}
	if sc[4] == 0 {
		return false
	}

	return foundPatternDiagonal(*sc)
}

func (f *finderPatternFinder) crossCheckVertical(startI, centerJ, maxCount, originalStateCountTotal int) float64 {
	maxI := f.image.Height()
	sc := f.getCrossCheckStateCount()

	i := startI
	for i >= 0 && f.image.Get(centerJ, i) {
		sc[2]++
		i--
	}
	if i < 0 {
		return math.NaN()
	}
	for i >= 0 && !f.image.Get(centerJ, i) && sc[1] <= maxCount {
		sc[1]++
		i--
	}
	if i < 0 || sc[1] > maxCount {
		return math.NaN()
	}
	for i >= 0 && f.image.Get(centerJ, i) && sc[0] <= maxCount {
		sc[0]++
		i--
	}
	if sc[0] > maxCount {
		return math.NaN()
	}

	i = startI + 1
	for i < maxI && f.image.Get(centerJ, i) {
		sc[2]++
		i++
	}
	if i == maxI {
		return math.NaN()
	}
	for i < maxI && !f.image.Get(centerJ, i) && sc[3] < maxCount {
		sc[3]++
		i++
	}
	if i == maxI || sc[3] >= maxCount {
		return math.NaN()
	}
	for i < maxI && f.image.Get(centerJ, i) && sc[4] < maxCount {
		sc[4]++
		i++
	}
	if sc[4] >= maxCount {
		return math.NaN()
	}

	stateCountTotal := sc[0] + sc[1] + sc[2] + sc[3] + sc[4]
	if 5*intAbs(stateCountTotal-originalStateCountTotal) >= 2*originalStateCountTotal {
		return math.NaN()
	}

	if foundPatternCross(*sc) {
		return centerFromEnd(*sc, i)
	}
	return math.NaN()
}

func (f *finderPatternFinder) crossCheckHorizontal(startJ, centerI, maxCount, originalStateCountTotal int) float64 {
	maxJ := f.image.Width()
	sc := f.getCrossCheckStateCount()

	j := startJ
	for j >= 0 && f.image.Get(j, centerI) {
		sc[2]++
		j--
	}
	if j < 0 {
		return math.NaN()
	}
	for j >= 0 && !f.image.Get(j, centerI) && sc[1] <= maxCount {
		sc[1]++
		j--
	}
	if j < 0 || sc[1] > maxCount {
		return math.NaN()
	}
	for j >= 0 && f.image.Get(j, centerI) && sc[0] <= maxCount {
		sc[0]++
		j--
	}
	if sc[0] > maxCount {
		return math.NaN()
	}

	j = startJ + 1
	for j < maxJ && f.image.Get(j, centerI) {
		sc[2]++
		j++
	}
	if j == maxJ {
		return math.NaN()
	}
	for j < maxJ && !f.image.Get(j, centerI) && sc[3] < maxCount {
		sc[3]++
		j++
	}
	if j == maxJ || sc[3] >= maxCount {
		return math.NaN()
	}
	for j < maxJ && f.image.Get(j, centerI) && sc[4] < maxCount {
		sc[4]++
		j++
	}
	if sc[4] >= maxCount {
		return math.NaN()
	}

	stateCountTotal := sc[0] + sc[1] + sc[2] + sc[3] + sc[4]
	if 5*intAbs(stateCountTotal-originalStateCountTotal) >= originalStateCountTotal {
		return math.NaN()
	}

	if foundPatternCross(*sc) {
		return centerFromEnd(*sc, j)
	}
	return math.NaN()
}

func (f *finderPatternFinder) handlePossibleCenter(stateCount [5]int, i, j int) bool {
	stateCountTotal := stateCount[0] + stateCount[1] + stateCount[2] + stateCount[3] + stateCount[4]
	centerJ := centerFromEnd(stateCount, j)
	centerI := f.crossCheckVertical(i, int(centerJ), stateCount[2], stateCountTotal)
	if math.IsNaN(centerI) {
		return false
	}

	centerJ = f.crossCheckHorizontal(int(centerJ), int(centerI), stateCount[2], stateCountTotal)
	if math.IsNaN(centerJ) || !f.crossCheckDiagonal(int(centerI), int(centerJ)) {
		return false
	}

	estimatedModuleSize := float64(stateCountTotal) / 7.0
	found := false
	for idx, center := range f.possibleCenters {
		if center.aboutEquals(estimatedModuleSize, centerI, centerJ) {
			f.possibleCenters[idx] = center.combineEstimate(centerI, centerJ, estimatedModuleSize)
			found = true
			break
		}
	}
	if !found {
		f.possibleCenters = append(f.possibleCenters, &FinderPattern{
			X: centerJ, Y: centerI, EstimatedModuleSize: estimatedModuleSize, Count: 1,
		})
	}
	return true
}

func (f *finderPatternFinder) findRowSkip() int {
	if len(f.possibleCenters) <= 1 {
		return 0
	}
	var firstConfirmedCenter *FinderPattern
	for _, center := range f.possibleCenters {
		if center.Count >= centerQuorum {
			if firstConfirmedCenter == nil {
				firstConfirmedCenter = center
			} else {
				f.hasSkipped = true
				return int(math.Abs(firstConfirmedCenter.X-center.X)-
					math.Abs(firstConfirmedCenter.Y-center.Y)) / 2
			}
		}
	}
	return 0
}

func (f *finderPatternFinder) haveMultiplyConfirmedCenters() bool {
	confirmedCount := 0
	totalModuleSize := 0.0
	n := len(f.possibleCenters)
	for _, pattern := range f.possibleCenters {
		if pattern.Count >= centerQuorum {
			confirmedCount++
			totalModuleSize += pattern.EstimatedModuleSize
		}
	}
	if confirmedCount < 3 {
		return false
	}
	average := totalModuleSize / float64(n)
	totalDeviation := 0.0
	for _, pattern := range f.possibleCenters {
		totalDeviation += math.Abs(pattern.EstimatedModuleSize - average)
	}
	return totalDeviation <= 0.05*totalModuleSize
}

func squaredDistance(a, b *FinderPattern) float64 {
	x := a.X - b.X
	y := a.Y - b.Y
	return x*x + y*y
}

func (f *finderPatternFinder) selectBestPatterns() ([]*FinderPattern, error) {
	if len(f.possibleCenters) < 3 {
		return nil, zxinggo.ErrNotFound
	}

	// Remove patterns with count < centerQuorum
	filtered := make([]*FinderPattern, 0, len(f.possibleCenters))
	for _, p := range f.possibleCenters {
		if p.Count >= centerQuorum {
			filtered = append(filtered, p)
		}
	}
	f.possibleCenters = filtered

	if len(f.possibleCenters) < 3 {
		return nil, zxinggo.ErrNotFound
	}

	// Sort by module size ascending
	sort.Slice(f.possibleCenters, func(i, j int) bool {
		return f.possibleCenters[i].EstimatedModuleSize < f.possibleCenters[j].EstimatedModuleSize
	})

	distortion := math.MaxFloat64
	var bestPatterns [3]*FinderPattern

	n := len(f.possibleCenters)
	for i := 0; i < n-2; i++ {
		fpi := f.possibleCenters[i]
		minModuleSize := fpi.EstimatedModuleSize

		for j := i + 1; j < n-1; j++ {
			fpj := f.possibleCenters[j]
			squares0 := squaredDistance(fpi, fpj)

			for k := j + 1; k < n; k++ {
				fpk := f.possibleCenters[k]
				maxModuleSize := fpk.EstimatedModuleSize
				if maxModuleSize > minModuleSize*1.4 {
					continue
				}

				a := squares0
				b := squaredDistance(fpj, fpk)
				c := squaredDistance(fpi, fpk)

				// Sort a, b, c ascending (inline)
				if a < b {
					if b > c {
						if a < c {
							b, c = c, b
						} else {
							a, b, c = c, a, b
						}
					}
				} else {
					if b < c {
						if a < c {
							a, b = b, a
						} else {
							a, b, c = b, c, a
						}
					} else {
						a, c = c, a
					}
				}

				d := math.Abs(c-2*b) + math.Abs(c-2*a)
				if d < distortion {
					distortion = d
					bestPatterns[0] = fpi
					bestPatterns[1] = fpj
					bestPatterns[2] = fpk
				}
			}
		}
	}

	if distortion == math.MaxFloat64 {
		return nil, zxinggo.ErrNotFound
	}

	return bestPatterns[:], nil
}

func orderFinderPatterns(patterns []*FinderPattern) *FinderPatternInfo {
	d01 := distanceFP(patterns[0], patterns[1])
	d12 := distanceFP(patterns[1], patterns[2])
	d02 := distanceFP(patterns[0], patterns[2])

	var pointA, pointB, pointC *FinderPattern
	// pointB = closest to other two (opposite longest side) = topLeft
	// pointA, pointC = endpoints of longest side
	if d12 >= d01 && d12 >= d02 {
		pointB = patterns[0]
		pointA = patterns[1]
		pointC = patterns[2]
	} else if d02 >= d01 && d02 >= d12 {
		pointB = patterns[1]
		pointA = patterns[0]
		pointC = patterns[2]
	} else {
		pointB = patterns[2]
		pointA = patterns[0]
		pointC = patterns[1]
	}

	// Java crossProductZ(A, B, C) = (C-B)×(A-B) z-component
	cross := (pointC.X-pointB.X)*(pointA.Y-pointB.Y) - (pointC.Y-pointB.Y)*(pointA.X-pointB.X)
	if cross < 0 {
		pointA, pointC = pointC, pointA
	}

	return &FinderPatternInfo{
		BottomLeft: pointA,
		TopLeft:    pointB,
		TopRight:   pointC,
	}
}

func distanceFP(a, b *FinderPattern) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return math.Sqrt(dx*dx + dy*dy)
}

// --- AlignmentPatternFinder ---

type alignmentPatternFinder struct {
	image              *bitutil.BitMatrix
	possibleCenters    []*AlignmentPattern
	startX, startY     int
	width, height      int
	moduleSize         float64
	crossCheckStateCount [3]int
}

func (af *alignmentPatternFinder) find() *AlignmentPattern {
	startX := af.startX
	height := af.height
	maxJ := startX + af.width
	middleI := af.startY + height/2

	stateCount := [3]int{}
	for iGen := 0; iGen < height; iGen++ {
		i := middleI
		if iGen&1 == 0 {
			i += (iGen + 1) / 2
		} else {
			i -= (iGen + 1) / 2
		}

		stateCount = [3]int{}
		j := startX
		for j < maxJ && !af.image.Get(j, i) {
			j++
		}
		currentState := 0
		for j < maxJ {
			if af.image.Get(j, i) {
				if currentState == 1 {
					stateCount[1]++
				} else {
					if currentState == 2 {
						if af.foundPatternCross(stateCount) {
							confirmed := af.handlePossibleCenter(stateCount, i, j)
							if confirmed != nil {
								return confirmed
							}
						}
						stateCount[0] = stateCount[2]
						stateCount[1] = 1
						stateCount[2] = 0
						currentState = 1
					} else {
						currentState++
						stateCount[currentState]++
					}
				}
			} else {
				if currentState == 1 {
					currentState++
				}
				stateCount[currentState]++
			}
			j++
		}
		if af.foundPatternCross(stateCount) {
			confirmed := af.handlePossibleCenter(stateCount, i, maxJ)
			if confirmed != nil {
				return confirmed
			}
		}
	}

	if len(af.possibleCenters) > 0 {
		return af.possibleCenters[0]
	}
	return nil
}

func (af *alignmentPatternFinder) foundPatternCross(stateCount [3]int) bool {
	moduleSize := af.moduleSize
	maxVariance := moduleSize / 2.0
	for i := 0; i < 3; i++ {
		if math.Abs(moduleSize-float64(stateCount[i])) >= maxVariance {
			return false
		}
	}
	return true
}

func (af *alignmentPatternFinder) crossCheckVertical(startI, centerJ, maxCount, originalStateCountTotal int) float64 {
	maxI := af.image.Height()
	sc := &af.crossCheckStateCount
	*sc = [3]int{}

	i := startI
	for i >= 0 && af.image.Get(centerJ, i) && sc[1] <= maxCount {
		sc[1]++
		i--
	}
	if i < 0 || sc[1] > maxCount {
		return math.NaN()
	}
	for i >= 0 && !af.image.Get(centerJ, i) && sc[0] <= maxCount {
		sc[0]++
		i--
	}
	if sc[0] > maxCount {
		return math.NaN()
	}

	i = startI + 1
	for i < maxI && af.image.Get(centerJ, i) && sc[1] <= maxCount {
		sc[1]++
		i++
	}
	if i == maxI || sc[1] > maxCount {
		return math.NaN()
	}
	for i < maxI && !af.image.Get(centerJ, i) && sc[2] <= maxCount {
		sc[2]++
		i++
	}
	if sc[2] > maxCount {
		return math.NaN()
	}

	stateCountTotal := sc[0] + sc[1] + sc[2]
	if 5*intAbs(stateCountTotal-originalStateCountTotal) >= 2*originalStateCountTotal {
		return math.NaN()
	}

	if af.foundPatternCross(*sc) {
		return float64(i-sc[2]) - float64(sc[1])/2.0
	}
	return math.NaN()
}

func (af *alignmentPatternFinder) handlePossibleCenter(stateCount [3]int, i, j int) *AlignmentPattern {
	stateCountTotal := stateCount[0] + stateCount[1] + stateCount[2]
	centerJ := float64(j-stateCount[2]) - float64(stateCount[1])/2.0
	centerI := af.crossCheckVertical(i, int(centerJ), 2*stateCount[1], stateCountTotal)
	if math.IsNaN(centerI) {
		return nil
	}
	estimatedModuleSize := float64(stateCount[0]+stateCount[1]+stateCount[2]) / 3.0
	for _, center := range af.possibleCenters {
		if center.aboutEquals(estimatedModuleSize, centerI, centerJ) {
			return center.combineEstimate(centerI, centerJ, estimatedModuleSize)
		}
	}
	af.possibleCenters = append(af.possibleCenters, &AlignmentPattern{
		X: centerJ, Y: centerI, EstimatedModuleSize: estimatedModuleSize,
	})
	return nil
}

// --- Detector ---

// Detector detects QR codes in binary images.
type Detector struct {
	image *bitutil.BitMatrix
}

// NewDetector creates a new Detector for the given image.
func NewDetector(image *bitutil.BitMatrix) *Detector {
	return &Detector{image: image}
}

// Detect detects a QR code and returns the sampled bit matrix and corner points.
func (d *Detector) Detect(tryHarder bool) (*internal.DetectorResult, error) {
	finder := &finderPatternFinder{image: d.image}
	info, err := finder.find(tryHarder)
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

	var alignmentPattern *AlignmentPattern
	if len(provisionalVersion.AlignmentPatternCenters) > 0 {
		bottomRightX := topRight.X - topLeft.X + bottomLeft.X
		bottomRightY := topRight.Y - topLeft.Y + bottomLeft.Y

		modulesBetweenFPCenters := provisionalVersion.DimensionForVersion() - 7
		correctionToTopLeft := 1.0 - 3.0/float64(modulesBetweenFPCenters)
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
	tltrCentersDimension := mathRound(distanceFP(topLeft, topRight) / moduleSize)
	tlblCentersDimension := mathRound(distanceFP(topLeft, bottomLeft) / moduleSize)
	dimension := (tltrCentersDimension+tlblCentersDimension)/2 + 7
	switch dimension & 0x03 {
	case 0:
		dimension++
	case 2:
		dimension--
	case 3:
		dimension -= 2
	}
	return dimension, nil
}

// mathRound matches Java's MathUtils.round: (int)(d + 0.5) for positive values.
func mathRound(d float64) int {
	if d < 0 {
		return int(d - 0.5)
	}
	return int(d + 0.5)
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
	steep := intAbs(toY-fromY) > intAbs(toX-fromX)
	if steep {
		fromX, fromY = fromY, fromX
		toX, toY = toY, toX
	}

	dx := intAbs(toX - fromX)
	dy := intAbs(toY - fromY)
	err := -dx / 2
	xstep := 1
	if fromX > toX {
		xstep = -1
	}
	ystep := 1
	if fromY > toY {
		ystep = -1
	}

	state := 0
	xLimit := toX + xstep
	for x, y := fromX, fromY; x != xLimit; x += xstep {
		realX := x
		realY := y
		if steep {
			realX = y
			realY = x
		}

		if (state == 1) == d.image.Get(realX, realY) {
			if state == 2 {
				return distancePt(x, y, fromX, fromY)
			}
			state++
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

	if state == 2 {
		return distancePt(toX+xstep, toY, fromX, fromY)
	}
	return math.NaN()
}

func distancePt(x1, y1, x2, y2 int) float64 {
	dx := float64(x1 - x2)
	dy := float64(y1 - y2)
	return math.Sqrt(dx*dx + dy*dy)
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
	alignmentAreaRightX := min(d.image.Width()-1, estAlignmentX+allowance)
	if float64(alignmentAreaRightX-alignmentAreaLeftX) < overallEstModuleSize*3 {
		return nil
	}
	alignmentAreaTopY := max(0, estAlignmentY-allowance)
	alignmentAreaBottomY := min(d.image.Height()-1, estAlignmentY+allowance)
	if float64(alignmentAreaBottomY-alignmentAreaTopY) < overallEstModuleSize*3 {
		return nil
	}

	finder := &alignmentPatternFinder{
		image:      d.image,
		startX:     alignmentAreaLeftX,
		startY:     alignmentAreaTopY,
		width:      alignmentAreaRightX - alignmentAreaLeftX,
		height:     alignmentAreaBottomY - alignmentAreaTopY,
		moduleSize: overallEstModuleSize,
	}
	return finder.find()
}

func intAbs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
