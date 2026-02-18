package oned

import (
	"math"
	"strings"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// Codabar is a linear barcode that encodes digits 0-9 and characters -, $, :,
// /, ., + with start/stop characters A, B, C, or D.
//
// This is a faithful port of the Java ZXing CodaBarReader.

// These values are critical for determining how permissive the decoding
// will be. All stripe sizes must be within the window these define, as
// compared to the average stripe size.
const (
	codabarMaxAcceptable   = 2.0
	codabarPadding         = 1.5
	codabarMinCharLength   = 3 // start + at least 1 data + stop
)

const codabarAlphabet = "0123456789-$:/.+ABCD"

// These represent the encodings of characters, as patterns of wide and narrow
// bars. The 7 least-significant bits of each int correspond to the pattern of
// wide and narrow, with 1s representing "wide" and 0s representing narrow.
var codabarCharacterEncodings = [20]int{
	0x003, 0x006, 0x009, 0x060, 0x012, 0x042, 0x021, 0x024, 0x030, 0x048, // 0-9
	0x00c, 0x018, 0x045, 0x051, 0x054, 0x015, 0x01A, 0x029, 0x00B, 0x00E, // -$:/.+ABCD
}

var codabarStartEndEncoding = [4]byte{'A', 'B', 'C', 'D'}

// CodabarReader decodes Codabar barcodes.
// It keeps instance variables to avoid reallocations, matching the Java implementation.
type CodabarReader struct {
	counters      []int
	counterLength int
}

// NewCodabarReader creates a new Codabar reader.
func NewCodabarReader() *CodabarReader {
	return &CodabarReader{
		counters:      make([]int, 80),
		counterLength: 0,
	}
}

// DecodeRow decodes a Codabar barcode from a single row.
func (r *CodabarReader) DecodeRow(rowNumber int, row *bitutil.BitArray, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	// Clear counters.
	for i := range r.counters {
		r.counters[i] = 0
	}

	if err := r.setCounters(row); err != nil {
		return nil, err
	}
	startOffset, err := r.findStartPattern()
	if err != nil {
		return nil, err
	}
	nextStart := startOffset

	var decodeRowResult []int // stores character offsets into the alphabet table

	for {
		charOffset := r.toNarrowWidePattern(nextStart)
		if charOffset == -1 {
			return nil, zxinggo.ErrNotFound
		}
		// Store the position in the alphabet table.
		decodeRowResult = append(decodeRowResult, charOffset)
		nextStart += 8
		// Stop as soon as we see the end character.
		if len(decodeRowResult) > 1 &&
			codabarArrayContains(codabarStartEndEncoding[:], codabarAlphabet[charOffset]) {
			break
		}
		if nextStart >= r.counterLength {
			break
		}
	}

	// Look for whitespace after pattern:
	trailingWhitespace := r.counters[nextStart-1]
	lastPatternSize := 0
	for i := -8; i < -1; i++ {
		lastPatternSize += r.counters[nextStart+i]
	}

	// We need to see whitespace equal to 50% of the last pattern size,
	// otherwise this is probably a false positive. The exception is if we are
	// at the end of the row. (I.e. the barcode barely fits.)
	if nextStart < r.counterLength && trailingWhitespace < lastPatternSize/2 {
		return nil, zxinggo.ErrNotFound
	}

	if err := r.validatePattern(startOffset, decodeRowResult); err != nil {
		return nil, err
	}

	// Translate character table offsets to actual characters.
	var result strings.Builder
	for _, offset := range decodeRowResult {
		result.WriteByte(codabarAlphabet[offset])
	}

	// Ensure a valid start and end character.
	s := result.String()
	if !codabarArrayContains(codabarStartEndEncoding[:], s[0]) {
		return nil, zxinggo.ErrNotFound
	}
	if !codabarArrayContains(codabarStartEndEncoding[:], s[len(s)-1]) {
		return nil, zxinggo.ErrNotFound
	}

	// Remove stop/start characters and check if a long enough string is contained.
	if len(s) <= codabarMinCharLength {
		// Almost surely a false positive (start + stop + at least 1 character)
		return nil, zxinggo.ErrNotFound
	}

	// Strip start/end characters (no ReturnCodabarStartEnd option in Go).
	s = s[1 : len(s)-1]

	runningCount := 0
	for i := 0; i < startOffset; i++ {
		runningCount += r.counters[i]
	}
	left := float64(runningCount)
	for i := startOffset; i < nextStart-1; i++ {
		runningCount += r.counters[i]
	}
	right := float64(runningCount)

	res := zxinggo.NewResult(
		s, nil,
		[]zxinggo.ResultPoint{
			{X: left, Y: float64(rowNumber)},
			{X: right, Y: float64(rowNumber)},
		},
		zxinggo.FormatCodabar,
	)
	res.PutMetadata(zxinggo.MetadataSymbologyIdentifier, "]F0")
	return res, nil
}

// validatePattern validates the pattern using statistical thresholds,
// faithfully porting the Java CodaBarReader.validatePattern method.
func (r *CodabarReader) validatePattern(start int, charOffsets []int) error {
	// First, sum up the total size of our four categories of stripe sizes.
	sizes := [4]int{0, 0, 0, 0}
	counts := [4]int{0, 0, 0, 0}
	end := len(charOffsets) - 1

	// We break out of this loop in the middle, in order to handle
	// inter-character spaces properly.
	pos := start
	for i := 0; i <= end; i++ {
		pattern := codabarCharacterEncodings[charOffsets[i]]
		for j := 6; j >= 0; j-- {
			// Even j = bars, while odd j = spaces. Categories 2 and 3 are for
			// long stripes, while 0 and 1 are for short stripes.
			category := (j & 1) + (pattern & 1) * 2
			sizes[category] += r.counters[pos+j]
			counts[category]++
			pattern >>= 1
		}
		// We ignore the inter-character space - it could be of any size.
		pos += 8
	}

	// Calculate our allowable size thresholds using fixed-point math.
	maxes := [4]float64{}
	mins := [4]float64{}
	// Define the threshold of acceptability to be the midpoint between the
	// average small stripe and the average large stripe. No stripe lengths
	// should be on the "wrong" side of that line.
	for i := 0; i < 2; i++ {
		mins[i] = 0.0 // Accept arbitrarily small "short" stripes.
		mins[i+2] = (float64(sizes[i])/float64(counts[i]) + float64(sizes[i+2])/float64(counts[i+2])) / 2.0
		maxes[i] = mins[i+2]
		maxes[i+2] = (float64(sizes[i+2])*codabarMaxAcceptable + codabarPadding) / float64(counts[i+2])
	}

	// Now verify that all of the stripes are within the thresholds.
	pos = start
	for i := 0; i <= end; i++ {
		pattern := codabarCharacterEncodings[charOffsets[i]]
		for j := 6; j >= 0; j-- {
			// Even j = bars, while odd j = spaces. Categories 2 and 3 are for
			// long stripes, while 0 and 1 are for short stripes.
			category := (j & 1) + (pattern & 1) * 2
			size := float64(r.counters[pos+j])
			if size < mins[category] || size > maxes[category] {
				return zxinggo.ErrNotFound
			}
			pattern >>= 1
		}
		pos += 8
	}

	return nil
}

// setCounters records the size of all runs of white and black pixels, starting
// with white. This is just like RecordPattern, except it records all the
// counters, and uses the builtin "counters" member for storage.
func (r *CodabarReader) setCounters(row *bitutil.BitArray) error {
	r.counterLength = 0
	// Start from the first white bit.
	i := row.GetNextUnset(0)
	end := row.Size()
	if i >= end {
		return zxinggo.ErrNotFound
	}
	isWhite := true
	count := 0
	for i < end {
		if row.Get(i) != isWhite {
			count++
		} else {
			r.counterAppend(count)
			count = 1
			isWhite = !isWhite
		}
		i++
	}
	r.counterAppend(count)
	return nil
}

// counterAppend appends a counter value, growing the slice if needed.
func (r *CodabarReader) counterAppend(e int) {
	r.counters[r.counterLength] = e
	r.counterLength++
	if r.counterLength >= len(r.counters) {
		temp := make([]int, r.counterLength*2)
		copy(temp, r.counters)
		r.counters = temp
	}
}

// findStartPattern scans the counter array for a valid start pattern.
func (r *CodabarReader) findStartPattern() (int, error) {
	for i := 1; i < r.counterLength; i += 2 {
		charOffset := r.toNarrowWidePattern(i)
		if charOffset != -1 && codabarArrayContains(codabarStartEndEncoding[:], codabarAlphabet[charOffset]) {
			// Look for whitespace before start pattern, >= 50% of width of start pattern.
			// We make an exception if the whitespace is the first element.
			patternSize := 0
			for j := i; j < i+7; j++ {
				patternSize += r.counters[j]
			}
			if i == 1 || r.counters[i-1] >= patternSize/2 {
				return i, nil
			}
		}
	}
	return 0, zxinggo.ErrNotFound
}

// toNarrowWidePattern determines the narrow/wide pattern at the given position
// in the counter array. Assumes that counters[position] is a bar.
func (r *CodabarReader) toNarrowWidePattern(position int) int {
	end := position + 7
	if end >= r.counterLength {
		return -1
	}

	theCounters := r.counters

	maxBar := 0
	minBar := math.MaxInt32
	for j := position; j < end; j += 2 {
		currentCounter := theCounters[j]
		if currentCounter < minBar {
			minBar = currentCounter
		}
		if currentCounter > maxBar {
			maxBar = currentCounter
		}
	}
	thresholdBar := (minBar + maxBar) / 2

	maxSpace := 0
	minSpace := math.MaxInt32
	for j := position + 1; j < end; j += 2 {
		currentCounter := theCounters[j]
		if currentCounter < minSpace {
			minSpace = currentCounter
		}
		if currentCounter > maxSpace {
			maxSpace = currentCounter
		}
	}
	thresholdSpace := (minSpace + maxSpace) / 2

	bitmask := 1 << 7
	pattern := 0
	for i := 0; i < 7; i++ {
		threshold := thresholdBar
		if (i & 1) != 0 {
			threshold = thresholdSpace
		}
		bitmask >>= 1
		if theCounters[position+i] > threshold {
			pattern |= bitmask
		}
	}

	for i := 0; i < len(codabarCharacterEncodings); i++ {
		if codabarCharacterEncodings[i] == pattern {
			return i
		}
	}
	return -1
}

// codabarArrayContains checks whether a byte slice contains the given key.
func codabarArrayContains(array []byte, key byte) bool {
	for _, c := range array {
		if c == key {
			return true
		}
	}
	return false
}

// Ensure CodabarReader implements RowDecoder at compile time.
var _ RowDecoder = (*CodabarReader)(nil)
