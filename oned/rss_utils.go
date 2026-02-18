package oned

import (
	"math"

	zxinggo "github.com/ericlevine/zxinggo"
)

// RSS utility types and functions, ported from Java ZXing's
// AbstractRSSReader, DataCharacter, FinderPattern, Pair, and RSSUtils.

const (
	rssMaxAvgVariance          = 0.2
	rssMaxIndividualVariance   = 0.45
	rssMinFinderPatternRatio   = 9.5 / 12.0
	rssMaxFinderPatternRatio   = 12.5 / 14.0
)

// rssDataCharacter encapsulates a single character value with checksum info.
type rssDataCharacter struct {
	value           int
	checksumPortion int
}

// rssFinderPattern encapsulates an RSS barcode finder pattern.
type rssFinderPattern struct {
	value        int
	startEnd     [2]int
	resultPoints [2]zxinggo.ResultPoint
}

// rssPair is a left or right pair in RSS-14, consisting of a data value, checksum, and finder pattern.
type rssPair struct {
	value           int
	checksumPortion int
	finderPattern   rssFinderPattern
	count           int
}

// rssParseFinderValue matches counters to known finder patterns and returns the index.
func rssParseFinderValue(counters []int, finderPatterns [][]int) (int, error) {
	for value := 0; value < len(finderPatterns); value++ {
		if PatternMatchVariance(counters, finderPatterns[value], rssMaxIndividualVariance) < rssMaxAvgVariance {
			return value, nil
		}
	}
	return 0, zxinggo.ErrNotFound
}

// rssIsFinderPattern checks if the given counters match the RSS finder pattern ratio constraints.
func rssIsFinderPattern(counters []int) bool {
	firstTwoSum := counters[0] + counters[1]
	sum := firstTwoSum + counters[2] + counters[3]
	ratio := float64(firstTwoSum) / float64(sum)
	if ratio >= rssMinFinderPatternRatio && ratio <= rssMaxFinderPatternRatio {
		minCounter := math.MaxInt32
		maxCounter := math.MinInt32
		for _, c := range counters {
			if c > maxCounter {
				maxCounter = c
			}
			if c < minCounter {
				minCounter = c
			}
		}
		return maxCounter < 10*minCounter
	}
	return false
}

// rssIncrement adjusts array by incrementing the element with the largest positive rounding error.
func rssIncrement(array []int, errors []float64) {
	index := 0
	biggestError := errors[0]
	for i := 1; i < len(array); i++ {
		if errors[i] > biggestError {
			biggestError = errors[i]
			index = i
		}
	}
	array[index]++
}

// rssDecrement adjusts array by decrementing the element with the largest negative rounding error.
func rssDecrement(array []int, errors []float64) {
	index := 0
	biggestError := errors[0]
	for i := 1; i < len(array); i++ {
		if errors[i] < biggestError {
			biggestError = errors[i]
			index = i
		}
	}
	array[index]--
}

// sumInts returns the sum of an int slice.
func sumInts(a []int) int {
	s := 0
	for _, v := range a {
		s += v
	}
	return s
}

// combins computes n-choose-r.
func combins(n, r int) int {
	var maxDenom, minDenom int
	if n-r > r {
		minDenom = r
		maxDenom = n - r
	} else {
		minDenom = n - r
		maxDenom = r
	}
	val := 1
	j := 1
	for i := n; i > maxDenom; i-- {
		val *= i
		if j <= minDenom {
			val /= j
			j++
		}
	}
	for j <= minDenom {
		val /= j
		j++
	}
	return val
}

// getRSSvalue computes the RSS symbol value from element widths.
func getRSSvalue(widths []int, maxWidth int, noNarrow bool) int {
	n := 0
	for _, w := range widths {
		n += w
	}
	val := 0
	narrowMask := 0
	elements := len(widths)
	for bar := 0; bar < elements-1; bar++ {
		elmWidth := 1
		narrowMask |= 1 << uint(bar) // init: assume narrow
		for elmWidth < widths[bar] {
			subVal := combins(n-elmWidth-1, elements-bar-2)
			if noNarrow && narrowMask == 0 &&
				n-elmWidth-(elements-bar-1) >= elements-bar-1 {
				subVal -= combins(n-elmWidth-(elements-bar), elements-bar-2)
			}
			if elements-bar-1 > 1 {
				lessVal := 0
				for mxwElement := n - elmWidth - (elements - bar - 2); mxwElement > maxWidth; mxwElement-- {
					lessVal += combins(n-elmWidth-mxwElement-1, elements-bar-3)
				}
				subVal -= lessVal * (elements - 1 - bar)
			} else if n-elmWidth > maxWidth {
				subVal--
			}
			val += subVal
			elmWidth++
			narrowMask &^= 1 << uint(bar) // increment: no longer narrow
		}
		n -= elmWidth
	}
	return val
}
