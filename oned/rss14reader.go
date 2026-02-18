package oned

import (
	"fmt"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// RSS14Reader decodes RSS-14 barcodes, including truncated and stacked variants.
// Ported from Java ZXing RSS14Reader.
type RSS14Reader struct {
	possibleLeftPairs  []rssPair
	possibleRightPairs []rssPair
	// Reusable scratch buffers
	decodeFinderCounters  [4]int
	dataCharacterCounters [8]int
	oddRoundingErrors     [4]float64
	evenRoundingErrors    [4]float64
	oddCounts             [4]int
	evenCounts            [4]int
}

func NewRSS14Reader() *RSS14Reader {
	return &RSS14Reader{}
}

var rss14OutsideEvenTotalSubset = []int{1, 10, 34, 70, 126}
var rss14InsideOddTotalSubset = []int{4, 20, 48, 81}
var rss14OutsideGsum = []int{0, 161, 961, 2015, 2715}
var rss14InsideGsum = []int{0, 336, 1036, 1516}
var rss14OutsideOddWidest = []int{8, 6, 4, 3, 1}
var rss14InsideOddWidest = []int{2, 4, 6, 8}

var rss14FinderPatterns = [][]int{
	{3, 8, 2, 1},
	{3, 5, 5, 1},
	{3, 3, 7, 1},
	{3, 1, 9, 1},
	{2, 7, 4, 1},
	{2, 5, 6, 1},
	{2, 3, 8, 1},
	{1, 5, 7, 1},
	{1, 3, 9, 1},
}

func (r *RSS14Reader) DecodeRow(rowNumber int, row *bitutil.BitArray, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	leftPair := r.decodePair(row, false, rowNumber)
	r.addOrTally(true, leftPair)
	row.Reverse()
	rightPair := r.decodePair(row, true, rowNumber)
	r.addOrTally(false, rightPair)
	row.Reverse()

	for i := range r.possibleLeftPairs {
		left := &r.possibleLeftPairs[i]
		if left.count > 1 {
			for j := range r.possibleRightPairs {
				right := &r.possibleRightPairs[j]
				if right.count > 1 && rss14CheckChecksum(left, right) {
					return rss14ConstructResult(left, right), nil
				}
			}
		}
	}
	return nil, zxinggo.ErrNotFound
}

func (r *RSS14Reader) addOrTally(isLeft bool, pair *rssPair) {
	if pair == nil {
		return
	}
	var list *[]rssPair
	if isLeft {
		list = &r.possibleLeftPairs
	} else {
		list = &r.possibleRightPairs
	}
	for i := range *list {
		if (*list)[i].value == pair.value {
			(*list)[i].count++
			return
		}
	}
	pair.count = 1
	*list = append(*list, *pair)
}

func rss14ConstructResult(leftPair, rightPair *rssPair) *zxinggo.Result {
	symbolValue := int64(4537077)*int64(leftPair.value) + int64(rightPair.value)
	text := fmt.Sprintf("%d", symbolValue)

	// Pad to 13 digits
	buf := make([]byte, 0, 14)
	for i := 13 - len(text); i > 0; i-- {
		buf = append(buf, '0')
	}
	buf = append(buf, []byte(text)...)

	// Compute check digit
	checkDigit := 0
	for i := 0; i < 13; i++ {
		digit := int(buf[i] - '0')
		if i&1 == 0 {
			checkDigit += 3 * digit
		} else {
			checkDigit += digit
		}
	}
	checkDigit = 10 - (checkDigit % 10)
	if checkDigit == 10 {
		checkDigit = 0
	}
	buf = append(buf, byte('0'+checkDigit))

	result := zxinggo.NewResult(
		string(buf),
		nil,
		[]zxinggo.ResultPoint{
			leftPair.finderPattern.resultPoints[0],
			leftPair.finderPattern.resultPoints[1],
			rightPair.finderPattern.resultPoints[0],
			rightPair.finderPattern.resultPoints[1],
		},
		zxinggo.FormatRSS14,
	)
	result.PutMetadata(zxinggo.MetadataSymbologyIdentifier, "]e0")
	return result
}

func rss14CheckChecksum(leftPair, rightPair *rssPair) bool {
	checkValue := (leftPair.checksumPortion + 16*rightPair.checksumPortion) % 79
	targetCheckValue := 9*leftPair.finderPattern.value + rightPair.finderPattern.value
	if targetCheckValue > 72 {
		targetCheckValue--
	}
	if targetCheckValue > 8 {
		targetCheckValue--
	}
	return checkValue == targetCheckValue
}

func (r *RSS14Reader) decodePair(row *bitutil.BitArray, right bool, rowNumber int) *rssPair {
	startEnd, err := r.findFinderPattern(row, right)
	if err != nil {
		return nil
	}
	pattern, err := r.parseFoundFinderPattern(row, rowNumber, right, startEnd)
	if err != nil {
		return nil
	}

	outside, err := r.decodeDataCharacter(row, pattern, true)
	if err != nil {
		return nil
	}
	inside, err := r.decodeDataCharacter(row, pattern, false)
	if err != nil {
		return nil
	}

	return &rssPair{
		value:           1597*outside.value + inside.value,
		checksumPortion: outside.checksumPortion + 4*inside.checksumPortion,
		finderPattern:   *pattern,
	}
}

func (r *RSS14Reader) decodeDataCharacter(row *bitutil.BitArray, pattern *rssFinderPattern, outsideChar bool) (*rssDataCharacter, error) {
	counters := r.dataCharacterCounters[:]
	for i := range counters {
		counters[i] = 0
	}

	if outsideChar {
		if err := RecordPatternInReverse(row, pattern.startEnd[0], counters); err != nil {
			return nil, err
		}
	} else {
		if err := RecordPattern(row, pattern.startEnd[1], counters); err != nil {
			return nil, err
		}
		// reverse it
		for i, j := 0, len(counters)-1; i < j; i, j = i+1, j-1 {
			counters[i], counters[j] = counters[j], counters[i]
		}
	}

	numModules := 16
	if !outsideChar {
		numModules = 15
	}
	elementWidth := float64(sumInts(counters)) / float64(numModules)

	oddCounts := r.oddCounts[:]
	evenCounts := r.evenCounts[:]
	oddRoundingErrors := r.oddRoundingErrors[:]
	evenRoundingErrors := r.evenRoundingErrors[:]

	for i := 0; i < len(counters); i++ {
		value := float64(counters[i]) / elementWidth
		count := int(value + 0.5)
		if count < 1 {
			count = 1
		} else if count > 8 {
			count = 8
		}
		offset := i / 2
		if i&1 == 0 {
			oddCounts[offset] = count
			oddRoundingErrors[offset] = value - float64(count)
		} else {
			evenCounts[offset] = count
			evenRoundingErrors[offset] = value - float64(count)
		}
	}

	if err := r.adjustOddEvenCounts14(outsideChar, numModules); err != nil {
		return nil, err
	}

	oddSum := 0
	oddChecksumPortion := 0
	for i := len(oddCounts) - 1; i >= 0; i-- {
		oddChecksumPortion *= 9
		oddChecksumPortion += oddCounts[i]
		oddSum += oddCounts[i]
	}
	evenChecksumPortion := 0
	evenSum := 0
	for i := len(evenCounts) - 1; i >= 0; i-- {
		evenChecksumPortion *= 9
		evenChecksumPortion += evenCounts[i]
		evenSum += evenCounts[i]
	}
	checksumPortion := oddChecksumPortion + 3*evenChecksumPortion

	if outsideChar {
		if oddSum&1 != 0 || oddSum > 12 || oddSum < 4 {
			return nil, zxinggo.ErrNotFound
		}
		group := (12 - oddSum) / 2
		oddWidest := rss14OutsideOddWidest[group]
		evenWidest := 9 - oddWidest
		vOdd := getRSSvalue(oddCounts, oddWidest, false)
		vEven := getRSSvalue(evenCounts, evenWidest, true)
		tEven := rss14OutsideEvenTotalSubset[group]
		gSum := rss14OutsideGsum[group]
		return &rssDataCharacter{value: vOdd*tEven + vEven + gSum, checksumPortion: checksumPortion}, nil
	}

	if evenSum&1 != 0 || evenSum > 10 || evenSum < 4 {
		return nil, zxinggo.ErrNotFound
	}
	group := (10 - evenSum) / 2
	oddWidest := rss14InsideOddWidest[group]
	evenWidest := 9 - oddWidest
	vOdd := getRSSvalue(oddCounts, oddWidest, true)
	vEven := getRSSvalue(evenCounts, evenWidest, false)
	tOdd := rss14InsideOddTotalSubset[group]
	gSum := rss14InsideGsum[group]
	return &rssDataCharacter{value: vEven*tOdd + vOdd + gSum, checksumPortion: checksumPortion}, nil
}

func (r *RSS14Reader) findFinderPattern(row *bitutil.BitArray, rightFinderPattern bool) ([2]int, error) {
	counters := r.decodeFinderCounters[:]
	counters[0] = 0
	counters[1] = 0
	counters[2] = 0
	counters[3] = 0

	width := row.Size()
	isWhite := false
	rowOffset := 0
	for rowOffset < width {
		isWhite = !row.Get(rowOffset)
		if rightFinderPattern == isWhite {
			break
		}
		rowOffset++
	}

	counterPosition := 0
	patternStart := rowOffset
	for x := rowOffset; x < width; x++ {
		if row.Get(x) != isWhite {
			counters[counterPosition]++
		} else {
			if counterPosition == 3 {
				if rssIsFinderPattern(counters) {
					return [2]int{patternStart, x}, nil
				}
				patternStart += counters[0] + counters[1]
				counters[0] = counters[2]
				counters[1] = counters[3]
				counters[2] = 0
				counters[3] = 0
				counterPosition--
			} else {
				counterPosition++
			}
			counters[counterPosition] = 1
			isWhite = !isWhite
		}
	}
	return [2]int{}, zxinggo.ErrNotFound
}

func (r *RSS14Reader) parseFoundFinderPattern(row *bitutil.BitArray, rowNumber int, right bool, startEnd [2]int) (*rssFinderPattern, error) {
	// Actually we found elements 2-5
	firstIsBlack := row.Get(startEnd[0])
	firstElementStart := startEnd[0] - 1
	for firstElementStart >= 0 && firstIsBlack != row.Get(firstElementStart) {
		firstElementStart--
	}
	firstElementStart++
	firstCounter := startEnd[0] - firstElementStart

	// Make 'counters' hold 1-4
	counters := r.decodeFinderCounters[:]
	copy(counters[1:], counters[:3])
	counters[0] = firstCounter

	value, err := rssParseFinderValue(counters, rss14FinderPatterns)
	if err != nil {
		return nil, err
	}

	start := firstElementStart
	end := startEnd[1]
	if right {
		start = row.Size() - 1 - start
		end = row.Size() - 1 - end
	}
	return &rssFinderPattern{
		value:    value,
		startEnd: [2]int{firstElementStart, startEnd[1]},
		resultPoints: [2]zxinggo.ResultPoint{
			{X: float64(start), Y: float64(rowNumber)},
			{X: float64(end), Y: float64(rowNumber)},
		},
	}, nil
}

func (r *RSS14Reader) adjustOddEvenCounts14(outsideChar bool, numModules int) error {
	oddSum := sumInts(r.oddCounts[:])
	evenSum := sumInts(r.evenCounts[:])

	incrementOdd := false
	decrementOdd := false
	incrementEven := false
	decrementEven := false

	if outsideChar {
		if oddSum > 12 {
			decrementOdd = true
		} else if oddSum < 4 {
			incrementOdd = true
		}
		if evenSum > 12 {
			decrementEven = true
		} else if evenSum < 4 {
			incrementEven = true
		}
	} else {
		if oddSum > 11 {
			decrementOdd = true
		} else if oddSum < 5 {
			incrementOdd = true
		}
		if evenSum > 10 {
			decrementEven = true
		} else if evenSum < 4 {
			incrementEven = true
		}
	}

	mismatch := oddSum + evenSum - numModules
	oddParityBad := false
	if outsideChar {
		oddParityBad = (oddSum & 1) == 1
	} else {
		oddParityBad = (oddSum & 1) == 0
	}
	evenParityBad := (evenSum & 1) == 1

	switch mismatch {
	case 1:
		if oddParityBad {
			if evenParityBad {
				return zxinggo.ErrNotFound
			}
			decrementOdd = true
		} else {
			if !evenParityBad {
				return zxinggo.ErrNotFound
			}
			decrementEven = true
		}
	case -1:
		if oddParityBad {
			if evenParityBad {
				return zxinggo.ErrNotFound
			}
			incrementOdd = true
		} else {
			if !evenParityBad {
				return zxinggo.ErrNotFound
			}
			incrementEven = true
		}
	case 0:
		if oddParityBad {
			if !evenParityBad {
				return zxinggo.ErrNotFound
			}
			if oddSum < evenSum {
				incrementOdd = true
				decrementEven = true
			} else {
				decrementOdd = true
				incrementEven = true
			}
		} else {
			if evenParityBad {
				return zxinggo.ErrNotFound
			}
		}
	default:
		return zxinggo.ErrNotFound
	}

	if incrementOdd {
		if decrementOdd {
			return zxinggo.ErrNotFound
		}
		rssIncrement(r.oddCounts[:], r.oddRoundingErrors[:])
	}
	if decrementOdd {
		rssDecrement(r.oddCounts[:], r.oddRoundingErrors[:])
	}
	if incrementEven {
		if decrementEven {
			return zxinggo.ErrNotFound
		}
		rssIncrement(r.evenCounts[:], r.oddRoundingErrors[:])
	}
	if decrementEven {
		rssDecrement(r.evenCounts[:], r.evenRoundingErrors[:])
	}
	return nil
}
