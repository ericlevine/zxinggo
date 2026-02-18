package detector

import (
	"math"
	"sort"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
	"github.com/ericlevine/zxinggo/internal"
)

const (
	maxModuleCountPerEdge      = 180.0
	minModuleCountPerEdge      = 9.0
	diffModSizeCutoffPercent   = 0.05
	diffModSizeCutoff          = 0.5
)

// DetectMulti detects multiple QR codes in the given image.
func DetectMulti(image *bitutil.BitMatrix, tryHarder bool) ([]*internal.DetectorResult, error) {
	finder := &finderPatternFinder{image: image}

	// Run the multi-finder pattern scan
	infos, err := findMulti(finder, tryHarder)
	if err != nil {
		return nil, err
	}

	det := &Detector{image: image}
	var results []*internal.DetectorResult
	for _, info := range infos {
		result, err := det.processFinderPatternInfo(info)
		if err == nil {
			results = append(results, result)
		}
	}
	if len(results) == 0 {
		return nil, zxinggo.ErrNotFound
	}
	return results, nil
}

func findMulti(f *finderPatternFinder, tryHarder bool) ([]*FinderPatternInfo, error) {
	image := f.image
	maxI := image.Height()
	maxJ := image.Width()

	iSkip := (3 * maxI) / (4 * maxModules)
	if iSkip < minSkip || tryHarder {
		iSkip = minSkip
	}

	stateCount := [5]int{}
	for i := iSkip - 1; i < maxI; i += iSkip {
		stateCount = [5]int{}
		currentState := 0
		for j := 0; j < maxJ; j++ {
			if image.Get(j, i) {
				if currentState&1 == 1 {
					currentState++
				}
				stateCount[currentState]++
			} else {
				if currentState&1 == 0 {
					if currentState == 4 {
						if foundPatternCross(stateCount) && f.handlePossibleCenter(stateCount, i, j) {
							currentState = 0
							stateCount = [5]int{}
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
			f.handlePossibleCenter(stateCount, i, maxJ)
		}
	}

	patternGroups, err := selectMultipleBestPatterns(f.possibleCenters)
	if err != nil {
		return nil, err
	}

	var result []*FinderPatternInfo
	for _, group := range patternGroups {
		info := orderFinderPatterns(group[:])
		result = append(result, info)
	}
	if len(result) == 0 {
		return nil, zxinggo.ErrNotFound
	}
	return result, nil
}

func selectMultipleBestPatterns(possibleCenters []*FinderPattern) ([][3]*FinderPattern, error) {
	// Filter to patterns seen at least twice
	var filtered []*FinderPattern
	for _, fp := range possibleCenters {
		if fp.Count >= 2 {
			filtered = append(filtered, fp)
		}
	}
	size := len(filtered)
	if size < 3 {
		return nil, zxinggo.ErrNotFound
	}

	if size == 3 {
		return [][3]*FinderPattern{{filtered[0], filtered[1], filtered[2]}}, nil
	}

	// Sort by estimated module size descending
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[j].EstimatedModuleSize < filtered[i].EstimatedModuleSize
	})

	var results [][3]*FinderPattern
	for i1 := 0; i1 < size-2; i1++ {
		p1 := filtered[i1]

		for i2 := i1 + 1; i2 < size-1; i2++ {
			p2 := filtered[i2]

			vModSize12A := math.Abs(p1.EstimatedModuleSize - p2.EstimatedModuleSize)
			vModSize12 := vModSize12A / math.Min(p1.EstimatedModuleSize, p2.EstimatedModuleSize)
			if vModSize12A > diffModSizeCutoff && vModSize12 >= diffModSizeCutoffPercent {
				break
			}

			for i3 := i2 + 1; i3 < size; i3++ {
				p3 := filtered[i3]

				vModSize23A := math.Abs(p2.EstimatedModuleSize - p3.EstimatedModuleSize)
				vModSize23 := vModSize23A / math.Min(p2.EstimatedModuleSize, p3.EstimatedModuleSize)
				if vModSize23A > diffModSizeCutoff && vModSize23 >= diffModSizeCutoffPercent {
					break
				}

				test := [3]*FinderPattern{p1, p2, p3}
				// Order using the same ordering as single QR detection
				ordered := orderFinderPatterns(test[:])

				dA := distanceFP(ordered.TopLeft, ordered.BottomLeft)
				dC := distanceFP(ordered.TopRight, ordered.BottomLeft)
				dB := distanceFP(ordered.TopLeft, ordered.TopRight)

				estimatedModuleCount := (dA + dB) / (p1.EstimatedModuleSize * 2.0)
				if estimatedModuleCount > maxModuleCountPerEdge || estimatedModuleCount < minModuleCountPerEdge {
					continue
				}

				vABBC := math.Abs((dA - dB) / math.Min(dA, dB))
				if vABBC >= 0.1 {
					continue
				}

				dCpy := math.Sqrt(dA*dA + dB*dB)
				vPyC := math.Abs((dC - dCpy) / math.Min(dC, dCpy))
				if vPyC >= 0.1 {
					continue
				}

				results = append(results, test)
			}
		}
	}

	if len(results) == 0 {
		return nil, zxinggo.ErrNotFound
	}
	return results, nil
}
