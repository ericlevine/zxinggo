package oned

import (
	"math"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// RSSExpandedReader decodes RSS Expanded barcodes.
// Ported from Java ZXing RSSExpandedReader.
type RSSExpandedReader struct {
	pairs          []expandedPair
	rows           []expandedRow
	startEnd       [2]int
	startFromEven  bool
	// Reusable scratch buffers
	decodeFinderCounters  [4]int
	dataCharacterCounters [8]int
	oddRoundingErrors     [4]float64
	evenRoundingErrors    [4]float64
	oddCounts             [4]int
	evenCounts            [4]int
}

func NewRSSExpandedReader() *RSSExpandedReader {
	return &RSSExpandedReader{}
}

var rssExpandedSymbolWidest = []int{7, 5, 4, 3, 1}
var rssExpandedEvenTotalSubset = []int{4, 20, 52, 104, 204}
var rssExpandedGsum = []int{0, 348, 1388, 2948, 3988}

var rssExpandedFinderPatterns = [][]int{
	{1, 8, 4, 1}, // A
	{3, 6, 4, 1}, // B
	{3, 4, 6, 1}, // C
	{3, 2, 8, 1}, // D
	{2, 6, 5, 1}, // E
	{2, 2, 9, 1}, // F
}

var rssExpandedWeights = [][]int{
	{1, 3, 9, 27, 81, 32, 96, 77},
	{20, 60, 180, 118, 143, 7, 21, 63},
	{189, 145, 13, 39, 117, 140, 209, 205},
	{193, 157, 49, 147, 19, 57, 171, 91},
	{62, 186, 136, 197, 169, 85, 44, 132},
	{185, 133, 188, 142, 4, 12, 36, 108},
	{113, 128, 173, 97, 80, 29, 87, 50},
	{150, 28, 84, 41, 123, 158, 52, 156},
	{46, 138, 203, 187, 139, 206, 196, 166},
	{76, 17, 51, 153, 37, 111, 122, 155},
	{43, 129, 176, 106, 107, 110, 119, 146},
	{16, 48, 144, 10, 30, 90, 59, 177},
	{109, 116, 137, 200, 178, 112, 125, 164},
	{70, 210, 208, 202, 184, 130, 179, 115},
	{134, 191, 151, 31, 93, 68, 204, 190},
	{148, 22, 66, 198, 172, 94, 71, 2},
	{6, 18, 54, 162, 64, 192, 154, 40},
	{120, 149, 25, 75, 14, 42, 126, 167},
	{79, 26, 78, 23, 69, 207, 199, 175},
	{103, 98, 83, 38, 114, 131, 182, 124},
	{161, 61, 183, 127, 170, 88, 53, 159},
	{55, 165, 73, 8, 24, 72, 5, 15},
	{45, 135, 194, 160, 58, 174, 100, 89},
}

var rssExpandedFinderPatternSequences = [][]int{
	{0, 0},
	{0, 1, 1},
	{0, 2, 1, 3},
	{0, 4, 1, 3, 2},
	{0, 4, 1, 3, 3, 5},
	{0, 4, 1, 3, 4, 5, 5},
	{0, 0, 1, 1, 2, 2, 3, 3},
	{0, 0, 1, 1, 2, 2, 3, 4, 4},
	{0, 0, 1, 1, 2, 2, 3, 4, 5, 5},
	{0, 0, 1, 1, 2, 3, 3, 4, 4, 5, 5},
}

const (
	rssExpandedFinderPatternModules          = 15.0
	rssExpandedDataCharacterModules          = 17.0
	rssExpandedMaxFinderPatternDistVariance  = 0.1
	rssExpandedMaxPairs                      = 11
)

func (r *RSSExpandedReader) DecodeRow(rowNumber int, row *bitutil.BitArray, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	// Try starting from even=false first, then even=true
	r.startFromEven = false
	result, err := r.tryDecodeRow(rowNumber, row)
	if err == nil {
		return result, nil
	}
	r.startFromEven = true
	return r.tryDecodeRow(rowNumber, row)
}

func (r *RSSExpandedReader) tryDecodeRow(rowNumber int, row *bitutil.BitArray) (*zxinggo.Result, error) {
	pairs, err := r.decodeRow2pairs(rowNumber, row)
	if err != nil {
		return nil, err
	}
	return rssExpandedConstructResult(pairs)
}

func (r *RSSExpandedReader) decodeRow2pairs(rowNumber int, row *bitutil.BitArray) ([]expandedPair, error) {
	r.pairs = r.pairs[:0]
	for {
		pair, err := r.retrieveNextPair(row, r.pairs, rowNumber)
		if err != nil {
			if len(r.pairs) == 0 {
				return nil, err
			}
			break
		}
		r.pairs = append(r.pairs, *pair)
	}

	if r.checkExpandedChecksum() && isValidSequence(r.pairs, true) {
		return r.pairs, nil
	}

	tryStackedDecode := len(r.rows) > 0
	r.storeRow(rowNumber)
	if tryStackedDecode {
		ps := r.checkRows(false)
		if ps != nil {
			return ps, nil
		}
		ps = r.checkRows(true)
		if ps != nil {
			return ps, nil
		}
	}

	return nil, zxinggo.ErrNotFound
}

func (r *RSSExpandedReader) checkRows(reverse bool) []expandedPair {
	if len(r.rows) > 25 {
		r.rows = r.rows[:0]
		return nil
	}
	r.pairs = r.pairs[:0]
	if reverse {
		reverseExpandedRows(r.rows)
	}
	ps := r.checkRowsRecursive(nil, 0)
	if reverse {
		reverseExpandedRows(r.rows)
	}
	return ps
}

func reverseExpandedRows(rows []expandedRow) {
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
}

func (r *RSSExpandedReader) checkRowsRecursive(collectedRows []expandedRow, currentRow int) []expandedPair {
	for i := currentRow; i < len(r.rows); i++ {
		row := r.rows[i]
		r.pairs = append(r.pairs, row.pairs...)
		addSize := len(row.pairs)

		if isValidSequence(r.pairs, false) {
			if r.checkExpandedChecksum() {
				result := make([]expandedPair, len(r.pairs))
				copy(result, r.pairs)
				return result
			}
			collectedRows = append(collectedRows, row)
			ps := r.checkRowsRecursive(collectedRows, i+1)
			if ps != nil {
				return ps
			}
			collectedRows = collectedRows[:len(collectedRows)-1]
			r.pairs = r.pairs[:len(r.pairs)-addSize]
		} else {
			r.pairs = r.pairs[:len(r.pairs)-addSize]
		}
	}
	return nil
}

func isValidSequence(pairs []expandedPair, complete bool) bool {
	for _, sequence := range rssExpandedFinderPatternSequences {
		sizeOk := false
		if complete {
			sizeOk = len(pairs) == len(sequence)
		} else {
			sizeOk = len(pairs) <= len(sequence)
		}
		if sizeOk {
			stop := true
			for j := 0; j < len(pairs); j++ {
				if pairs[j].finderPattern.value != sequence[j] {
					stop = false
					break
				}
			}
			if stop {
				return true
			}
		}
	}
	return false
}

func mayFollow(pairs []expandedPair, value int) bool {
	if len(pairs) == 0 {
		return true
	}
	for _, sequence := range rssExpandedFinderPatternSequences {
		if len(pairs)+1 <= len(sequence) {
			for i := len(pairs); i < len(sequence); i++ {
				if sequence[i] == value {
					matched := true
					for j := 0; j < len(pairs); j++ {
						if sequence[i-j-1] != pairs[len(pairs)-j-1].finderPattern.value {
							matched = false
							break
						}
					}
					if matched {
						return true
					}
				}
			}
		}
	}
	return false
}

func (r *RSSExpandedReader) storeRow(rowNumber int) {
	insertPos := 0
	prevIsSame := false
	nextIsSame := false
	for insertPos < len(r.rows) {
		erow := &r.rows[insertPos]
		if erow.rowNumber > rowNumber {
			nextIsSame = erow.isEquivalent(r.pairs)
			break
		}
		prevIsSame = erow.isEquivalent(r.pairs)
		insertPos++
	}
	if nextIsSame || prevIsSame {
		return
	}
	if isPartialRow(r.pairs, r.rows) {
		return
	}
	newRow := newExpandedRow(r.pairs, rowNumber)
	// insert at insertPos
	r.rows = append(r.rows, expandedRow{})
	copy(r.rows[insertPos+1:], r.rows[insertPos:])
	r.rows[insertPos] = newRow
	removePartialRows(r.pairs, &r.rows)
}

func removePartialRows(pairs []expandedPair, rows *[]expandedRow) {
	n := 0
	for _, row := range *rows {
		if len(row.pairs) != len(pairs) {
			allFound := true
			for _, p := range row.pairs {
				found := false
				for _, pp := range pairs {
					if expandedPairEqual(p, pp) {
						found = true
						break
					}
				}
				if !found {
					allFound = false
					break
				}
			}
			if allFound {
				continue // remove this row
			}
		}
		(*rows)[n] = row
		n++
	}
	*rows = (*rows)[:n]
}

func isPartialRow(pairs []expandedPair, rows []expandedRow) bool {
	for _, row := range rows {
		allFound := true
		for _, p := range pairs {
			found := false
			for _, pp := range row.pairs {
				if expandedPairEqual(p, pp) {
					found = true
					break
				}
			}
			if !found {
				allFound = false
				break
			}
		}
		if allFound {
			return true
		}
	}
	return false
}

func (r *RSSExpandedReader) checkExpandedChecksum() bool {
	if len(r.pairs) == 0 {
		return false
	}
	firstPair := r.pairs[0]
	checkCharacter := firstPair.leftChar
	firstCharacter := firstPair.rightChar
	if firstCharacter == nil {
		return false
	}
	checksum := firstCharacter.checksumPortion
	s := 2
	for i := 1; i < len(r.pairs); i++ {
		currentPair := r.pairs[i]
		checksum += currentPair.leftChar.checksumPortion
		s++
		if currentPair.rightChar != nil {
			checksum += currentPair.rightChar.checksumPortion
			s++
		}
	}
	checksum %= 211
	checkCharacterValue := 211*(s-4) + checksum
	return checkCharacterValue == checkCharacter.value
}

func (r *RSSExpandedReader) getNextSecondBar(row *bitutil.BitArray, initialPos int) int {
	var currentPos int
	if row.Get(initialPos) {
		currentPos = row.GetNextUnset(initialPos)
		currentPos = row.GetNextSet(currentPos)
	} else {
		currentPos = row.GetNextSet(initialPos)
		currentPos = row.GetNextUnset(currentPos)
	}
	return currentPos
}

func (r *RSSExpandedReader) retrieveNextPair(row *bitutil.BitArray, previousPairs []expandedPair, rowNumber int) (*expandedPair, error) {
	isOddPattern := len(previousPairs)%2 == 0
	if r.startFromEven {
		isOddPattern = !isOddPattern
	}

	var pattern *rssFinderPattern
	var leftChar *rssDataCharacter
	forcedOffset := -1
	for {
		if err := r.findNextPair(row, previousPairs, forcedOffset); err != nil {
			return nil, err
		}
		pattern = r.parseFoundExpandedFinderPattern(row, rowNumber, isOddPattern, previousPairs)
		if pattern == nil {
			forcedOffset = r.getNextSecondBar(row, r.startEnd[0])
			continue
		}
		var err error
		leftChar, err = r.decodeExpandedDataCharacter(row, pattern, isOddPattern, true)
		if err != nil {
			forcedOffset = r.getNextSecondBar(row, r.startEnd[0])
			continue
		}
		break
	}

	if len(previousPairs) > 0 && previousPairs[len(previousPairs)-1].mustBeLast() {
		return nil, zxinggo.ErrNotFound
	}

	var rightChar *rssDataCharacter
	rc, err := r.decodeExpandedDataCharacter(row, pattern, isOddPattern, false)
	if err == nil {
		rightChar = rc
	}
	return &expandedPair{leftChar: leftChar, rightChar: rightChar, finderPattern: *pattern}, nil
}

func (r *RSSExpandedReader) findNextPair(row *bitutil.BitArray, previousPairs []expandedPair, forcedOffset int) error {
	counters := r.decodeFinderCounters[:]
	counters[0] = 0
	counters[1] = 0
	counters[2] = 0
	counters[3] = 0

	width := row.Size()

	var rowOffset int
	if forcedOffset >= 0 {
		rowOffset = forcedOffset
	} else if len(previousPairs) == 0 {
		rowOffset = 0
	} else {
		lastPair := previousPairs[len(previousPairs)-1]
		rowOffset = lastPair.finderPattern.startEnd[1]
	}
	searchingEvenPair := len(previousPairs)%2 != 0
	if r.startFromEven {
		searchingEvenPair = !searchingEvenPair
	}

	isWhite := false
	for rowOffset < width {
		isWhite = !row.Get(rowOffset)
		if !isWhite {
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
				if searchingEvenPair {
					reverseInts(counters)
				}
				if rssIsFinderPattern(counters) {
					r.startEnd[0] = patternStart
					r.startEnd[1] = x
					if searchingEvenPair {
						reverseInts(counters)
					}
					return nil
				}
				if searchingEvenPair {
					reverseInts(counters)
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
	return zxinggo.ErrNotFound
}

func reverseInts(a []int) {
	for i, j := 0, len(a)-1; i < j; i, j = i+1, j-1 {
		a[i], a[j] = a[j], a[i]
	}
}

func (r *RSSExpandedReader) parseFoundExpandedFinderPattern(row *bitutil.BitArray, rowNumber int, oddPattern bool, previousPairs []expandedPair) *rssFinderPattern {
	var firstCounter int
	var start, end int

	if oddPattern {
		firstElementStart := r.startEnd[0] - 1
		for firstElementStart >= 0 && !row.Get(firstElementStart) {
			firstElementStart--
		}
		firstElementStart++
		firstCounter = r.startEnd[0] - firstElementStart
		start = firstElementStart
		end = r.startEnd[1]
	} else {
		start = r.startEnd[0]
		end = row.GetNextUnset(r.startEnd[1] + 1)
		firstCounter = end - r.startEnd[1]
	}

	counters := r.decodeFinderCounters[:]
	copy(counters[1:], counters[:3])
	counters[0] = firstCounter

	value, err := rssParseFinderValue(counters, rssExpandedFinderPatterns)
	if err != nil {
		return nil
	}

	if !mayFollow(previousPairs, value) {
		return nil
	}

	// Check distance from previous pair
	if len(previousPairs) > 0 {
		prev := previousPairs[len(previousPairs)-1]
		prevStart := prev.finderPattern.startEnd[0]
		prevEnd := prev.finderPattern.startEnd[1]
		prevWidth := prevEnd - prevStart
		charWidth := (float64(prevWidth) / rssExpandedFinderPatternModules) * rssExpandedDataCharacterModules
		minX := float64(prevEnd) + 2*charWidth*(1-rssExpandedMaxFinderPatternDistVariance)
		maxX := float64(prevEnd) + 2*charWidth*(1+rssExpandedMaxFinderPatternDistVariance)
		if float64(start) < minX || float64(start) > maxX {
			return nil
		}
	}

	return &rssFinderPattern{
		value:    value,
		startEnd: [2]int{start, end},
		resultPoints: [2]zxinggo.ResultPoint{
			{X: float64(start), Y: float64(rowNumber)},
			{X: float64(end), Y: float64(rowNumber)},
		},
	}
}

func (r *RSSExpandedReader) decodeExpandedDataCharacter(row *bitutil.BitArray, pattern *rssFinderPattern, isOddPattern, leftChar bool) (*rssDataCharacter, error) {
	counters := r.dataCharacterCounters[:]
	for i := range counters {
		counters[i] = 0
	}

	if leftChar {
		if err := RecordPatternInReverse(row, pattern.startEnd[0], counters); err != nil {
			return nil, err
		}
	} else {
		if err := RecordPattern(row, pattern.startEnd[1], counters); err != nil {
			return nil, err
		}
		for i, j := 0, len(counters)-1; i < j; i, j = i+1, j-1 {
			counters[i], counters[j] = counters[j], counters[i]
		}
	}

	numModules := 17
	elementWidth := float64(sumInts(counters)) / float64(numModules)

	// Sanity check element width vs pattern width
	expectedElementWidth := float64(pattern.startEnd[1]-pattern.startEnd[0]) / 15.0
	if math.Abs(elementWidth-expectedElementWidth)/expectedElementWidth > 0.3 {
		return nil, zxinggo.ErrNotFound
	}

	oddCounts := r.oddCounts[:]
	evenCounts := r.evenCounts[:]
	oddRoundingErrors := r.oddRoundingErrors[:]
	evenRoundingErrors := r.evenRoundingErrors[:]

	for i := 0; i < len(counters); i++ {
		value := float64(counters[i]) / elementWidth
		count := int(value + 0.5)
		if count < 1 {
			if value < 0.3 {
				return nil, zxinggo.ErrNotFound
			}
			count = 1
		} else if count > 8 {
			if value > 8.7 {
				return nil, zxinggo.ErrNotFound
			}
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

	if err := r.adjustOddEvenCountsExpanded(numModules); err != nil {
		return nil, err
	}

	weightRowNumber := 4*pattern.value + boolToInt(!isOddPattern)*2 + boolToInt(!leftChar) - 1

	oddSum := 0
	oddChecksumPortion := 0
	for i := len(oddCounts) - 1; i >= 0; i-- {
		if isNotA1left(pattern, isOddPattern, leftChar) {
			oddChecksumPortion += oddCounts[i] * rssExpandedWeights[weightRowNumber][2*i]
		}
		oddSum += oddCounts[i]
	}
	evenChecksumPortion := 0
	for i := len(evenCounts) - 1; i >= 0; i-- {
		if isNotA1left(pattern, isOddPattern, leftChar) {
			evenChecksumPortion += evenCounts[i] * rssExpandedWeights[weightRowNumber][2*i+1]
		}
	}
	checksumPortion := oddChecksumPortion + evenChecksumPortion

	if oddSum&1 != 0 || oddSum > 13 || oddSum < 4 {
		return nil, zxinggo.ErrNotFound
	}
	group := (13 - oddSum) / 2
	oddWidest := rssExpandedSymbolWidest[group]
	evenWidest := 9 - oddWidest
	vOdd := getRSSvalue(oddCounts, oddWidest, true)
	vEven := getRSSvalue(evenCounts, evenWidest, false)
	tEven := rssExpandedEvenTotalSubset[group]
	gSum := rssExpandedGsum[group]
	value := vOdd*tEven + vEven + gSum

	return &rssDataCharacter{value: value, checksumPortion: checksumPortion}, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func isNotA1left(pattern *rssFinderPattern, isOddPattern, leftChar bool) bool {
	return !(pattern.value == 0 && isOddPattern && leftChar)
}

func (r *RSSExpandedReader) adjustOddEvenCountsExpanded(numModules int) error {
	oddSum := sumInts(r.oddCounts[:])
	evenSum := sumInts(r.evenCounts[:])

	incrementOdd := false
	decrementOdd := false
	if oddSum > 13 {
		decrementOdd = true
	} else if oddSum < 4 {
		incrementOdd = true
	}
	incrementEven := false
	decrementEven := false
	if evenSum > 13 {
		decrementEven = true
	} else if evenSum < 4 {
		incrementEven = true
	}

	mismatch := oddSum + evenSum - numModules
	oddParityBad := (oddSum & 1) == 1
	evenParityBad := (evenSum & 1) == 0

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
