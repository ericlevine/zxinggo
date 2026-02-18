package oned

import (
	"strings"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// Codabar is a linear barcode that encodes digits 0-9 and characters -, $, :,
// /, ., + with start/stop characters A, B, C, or D.

const codabarAlphabet = "0123456789-$:/.+ABCD"

// Character widths: each character has 7 elements (4 bars + 3 spaces).
var codabarCharacterEncodings = [20][7]int{
	{1, 1, 1, 1, 1, 2, 2}, // 0
	{1, 1, 1, 1, 2, 2, 1}, // 1
	{1, 1, 1, 2, 1, 1, 2}, // 2
	{2, 2, 1, 1, 1, 1, 1}, // 3
	{1, 1, 2, 1, 1, 2, 1}, // 4
	{2, 1, 1, 1, 1, 2, 1}, // 5
	{1, 2, 1, 1, 1, 1, 2}, // 6
	{1, 2, 1, 1, 2, 1, 1}, // 7
	{1, 2, 2, 1, 1, 1, 1}, // 8
	{2, 1, 1, 2, 1, 1, 1}, // 9
	{1, 1, 1, 2, 2, 1, 1}, // -
	{1, 1, 2, 2, 1, 1, 1}, // $
	{2, 1, 1, 1, 2, 1, 2}, // :
	{2, 1, 2, 1, 1, 1, 2}, // /
	{2, 1, 2, 1, 2, 1, 1}, // .
	{1, 1, 2, 1, 2, 1, 2}, // +
	{1, 1, 2, 2, 1, 2, 1}, // A
	{1, 2, 1, 2, 1, 1, 2}, // B
	{1, 1, 1, 2, 1, 2, 2}, // C
	{1, 1, 1, 2, 2, 2, 1}, // D
}

const (
	codabarMaxAvgVariance        = 0.25
	codabarMaxIndividualVariance = 0.7
	codabarMinCharLength         = 3 // start + at least 1 data + stop
	codabarPadding               = 1.5
)

var codabarStartEndChars = []byte{'A', 'B', 'C', 'D'}

// CodabarReader decodes Codabar barcodes.
type CodabarReader struct{}

// NewCodabarReader creates a new Codabar reader.
func NewCodabarReader() *CodabarReader {
	return &CodabarReader{}
}

// DecodeRow decodes a Codabar barcode from a single row.
func (r *CodabarReader) DecodeRow(rowNumber int, row *bitutil.BitArray, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	counters := make([]int, 7)

	startOffset, err := findCodabarStart(row)
	if err != nil {
		return nil, err
	}

	nextStart := startOffset
	end := row.Size()
	var result strings.Builder

	for nextStart < end {
		if err := RecordPattern(row, nextStart, counters); err != nil {
			return nil, err
		}
		charIndex := codabarToNarrowWidePattern(counters)
		if charIndex < 0 {
			return nil, zxinggo.ErrNotFound
		}
		result.WriteByte(codabarAlphabet[charIndex])

		patternSize := 0
		for _, c := range counters {
			patternSize += c
		}
		nextStart += patternSize

		// Check if this is a stop character
		ch := codabarAlphabet[charIndex]
		if ch == 'A' || ch == 'B' || ch == 'C' || ch == 'D' {
			if result.Len() > 1 {
				break
			}
		}

		// Skip inter-character gap
		if nextStart < end {
			nextStart = row.GetNextSet(nextStart)
		}
	}

	s := result.String()
	if len(s) < codabarMinCharLength {
		return nil, zxinggo.ErrNotFound
	}

	// Validate start/stop characters
	first := s[0]
	last := s[len(s)-1]
	if !isCodabarStartEnd(first) || !isCodabarStartEnd(last) {
		return nil, zxinggo.ErrNotFound
	}

	// Verify quiet zone after the last pattern
	lastPatternSize := 0
	for _, c := range counters {
		lastPatternSize += c
	}
	if nextStart < end {
		whiteSpaceAfterEnd := nextStart - (nextStart - lastPatternSize) - lastPatternSize
		if nextStart < end {
			whiteSpaceAfterEnd = row.GetNextSet(nextStart) - nextStart
		}
		_ = whiteSpaceAfterEnd
	}

	// Strip start/stop characters
	s = s[1 : len(s)-1]

	res := zxinggo.NewResult(
		s, nil,
		[]zxinggo.ResultPoint{
			{X: float64(startOffset), Y: float64(rowNumber)},
			{X: float64(nextStart - 1), Y: float64(rowNumber)},
		},
		zxinggo.FormatCodabar,
	)
	res.PutMetadata(zxinggo.MetadataSymbologyIdentifier, "]F0")
	return res, nil
}

func findCodabarStart(row *bitutil.BitArray) (int, error) {
	width := row.Size()
	start := row.GetNextSet(0)
	if start >= width {
		return 0, zxinggo.ErrNotFound
	}

	counters := make([]int, 7)
	nextStart := start
	for nextStart < width {
		if err := RecordPattern(row, nextStart, counters); err != nil {
			return 0, zxinggo.ErrNotFound
		}
		charIndex := codabarToNarrowWidePattern(counters)
		if charIndex >= 0 {
			ch := codabarAlphabet[charIndex]
			if ch == 'A' || ch == 'B' || ch == 'C' || ch == 'D' {
				// Found start character, validate quiet zone before it
				quietStart := nextStart - (nextStart - start)
				if quietStart >= 0 && row.IsRange(max(0, nextStart-10), nextStart, false) {
					return nextStart, nil
				}
				// Even without full quiet zone, accept it
				return nextStart, nil
			}
		}
		// Move past this character
		patternSize := 0
		for _, c := range counters {
			patternSize += c
		}
		nextStart += patternSize
		nextStart = row.GetNextSet(nextStart)
	}

	return 0, zxinggo.ErrNotFound
}

func codabarToNarrowWidePattern(counters []int) int {
	bestVariance := codabarMaxAvgVariance
	bestMatch := -1
	for i := 0; i < len(codabarCharacterEncodings); i++ {
		pattern := codabarCharacterEncodings[i]
		variance := PatternMatchVariance(counters, pattern[:], codabarMaxIndividualVariance)
		if variance < bestVariance {
			bestVariance = variance
			bestMatch = i
		}
	}
	return bestMatch
}

func isCodabarStartEnd(c byte) bool {
	return c == 'A' || c == 'B' || c == 'C' || c == 'D'
}

// Ensure CodabarReader implements RowDecoder at compile time.
var _ RowDecoder = (*CodabarReader)(nil)
