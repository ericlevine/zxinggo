package oned

import (
	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// expandedPair represents a pair of data characters with a finder pattern in RSS Expanded.
type expandedPair struct {
	leftChar      *rssDataCharacter
	rightChar     *rssDataCharacter
	finderPattern rssFinderPattern
}

func (p *expandedPair) mustBeLast() bool {
	return p.rightChar == nil
}

func expandedPairEqual(a, b expandedPair) bool {
	if a.leftChar == nil != (b.leftChar == nil) {
		return false
	}
	if a.leftChar != nil && (a.leftChar.value != b.leftChar.value || a.leftChar.checksumPortion != b.leftChar.checksumPortion) {
		return false
	}
	if a.rightChar == nil != (b.rightChar == nil) {
		return false
	}
	if a.rightChar != nil && (a.rightChar.value != b.rightChar.value || a.rightChar.checksumPortion != b.rightChar.checksumPortion) {
		return false
	}
	return a.finderPattern.value == b.finderPattern.value
}

// expandedRow represents one row of an RSS Expanded Stacked symbol.
type expandedRow struct {
	pairs     []expandedPair
	rowNumber int
}

func newExpandedRow(pairs []expandedPair, rowNumber int) expandedRow {
	cp := make([]expandedPair, len(pairs))
	copy(cp, pairs)
	return expandedRow{pairs: cp, rowNumber: rowNumber}
}

func (r *expandedRow) isEquivalent(otherPairs []expandedPair) bool {
	if len(r.pairs) != len(otherPairs) {
		return false
	}
	for i := range r.pairs {
		if !expandedPairEqual(r.pairs[i], otherPairs[i]) {
			return false
		}
	}
	return true
}

// buildExpandedBitArray builds a BitArray from the expanded pairs.
func buildExpandedBitArray(pairs []expandedPair) *bitutil.BitArray {
	charNumber := len(pairs)*2 - 1
	if pairs[len(pairs)-1].rightChar == nil {
		charNumber--
	}

	size := 12 * charNumber
	binary := bitutil.NewBitArray(size)
	accPos := 0

	firstPair := pairs[0]
	firstValue := firstPair.rightChar.value
	for i := 11; i >= 0; i-- {
		if (firstValue & (1 << uint(i))) != 0 {
			binary.Set(accPos)
		}
		accPos++
	}

	for i := 1; i < len(pairs); i++ {
		currentPair := pairs[i]
		leftValue := currentPair.leftChar.value
		for j := 11; j >= 0; j-- {
			if (leftValue & (1 << uint(j))) != 0 {
				binary.Set(accPos)
			}
			accPos++
		}
		if currentPair.rightChar != nil {
			rightValue := currentPair.rightChar.value
			for j := 11; j >= 0; j-- {
				if (rightValue & (1 << uint(j))) != 0 {
					binary.Set(accPos)
				}
				accPos++
			}
		}
	}
	return binary
}

// rssExpandedConstructResult constructs the final result from expanded pairs.
func rssExpandedConstructResult(pairs []expandedPair) (*zxinggo.Result, error) {
	binary := buildExpandedBitArray(pairs)
	resultingString, err := parseExpandedInformation(binary)
	if err != nil {
		return nil, err
	}

	firstPoints := pairs[0].finderPattern.resultPoints
	lastPoints := pairs[len(pairs)-1].finderPattern.resultPoints

	result := zxinggo.NewResult(
		resultingString,
		nil,
		[]zxinggo.ResultPoint{firstPoints[0], firstPoints[1], lastPoints[0], lastPoints[1]},
		zxinggo.FormatRSSExpanded,
	)
	result.PutMetadata(zxinggo.MetadataSymbologyIdentifier, "]e0")
	return result, nil
}
