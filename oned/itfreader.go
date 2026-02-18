package oned

import (
	"strings"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// ITF encodes pairs of digits: the first digit of each pair is encoded in the
// bars and the second in the spaces. ITF-14 is ITF with a fixed length of 14.

const (
	itfMaxAvgVariance           = 0.38
	itfMaxIndividualVariance2x  = 0.5  // 2x wide lines
	itfMaxIndividualVariance3x  = 0.75 // 3x wide lines
)

// Patterns of narrow/wide for digits 0-9, duplicated for 2x and 3x wide.
// Indices 0-9 use 2x (w=2), indices 10-19 use 3x (W=3).
var itfPatterns = [20][5]int{
	{1, 1, 2, 2, 1}, // 0 (2x)
	{2, 1, 1, 1, 2}, // 1
	{1, 2, 1, 1, 2}, // 2
	{2, 2, 1, 1, 1}, // 3
	{1, 1, 2, 1, 2}, // 4
	{2, 1, 2, 1, 1}, // 5
	{1, 2, 2, 1, 1}, // 6
	{1, 1, 1, 2, 2}, // 7
	{2, 1, 1, 2, 1}, // 8
	{1, 2, 1, 2, 1}, // 9
	{1, 1, 3, 3, 1}, // 0 (3x)
	{3, 1, 1, 1, 3}, // 1
	{1, 3, 1, 1, 3}, // 2
	{3, 3, 1, 1, 1}, // 3
	{1, 1, 3, 1, 3}, // 4
	{3, 1, 3, 1, 1}, // 5
	{1, 3, 3, 1, 1}, // 6
	{1, 1, 1, 3, 3}, // 7
	{3, 1, 1, 3, 1}, // 8
	{1, 3, 1, 3, 1}, // 9
}

// Start/end patterns: narrow bar, narrow space, narrow bar, narrow space
var itfStartPattern = []int{1, 1, 1, 1}
var itfEndPatternReversed = [2][]int{
	{1, 1, 2}, // 2x
	{1, 1, 3}, // 3x
}

// ITFReader decodes ITF (Interleaved 2 of 5) barcodes.
type ITFReader struct {
	narrowLineWidth int
}

// NewITFReader creates a new ITF reader.
func NewITFReader() *ITFReader {
	return &ITFReader{narrowLineWidth: -1}
}

// DecodeRow decodes an ITF barcode from a single row.
func (r *ITFReader) DecodeRow(rowNumber int, row *bitutil.BitArray, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	startRange, err := r.decodeStart(row)
	if err != nil {
		return nil, err
	}
	endRange, err := r.decodeEnd(row)
	if err != nil {
		return nil, err
	}

	var result strings.Builder
	err = r.decodeMiddle(row, startRange[1], endRange[0], &result)
	if err != nil {
		return nil, err
	}
	resultString := result.String()

	allowedLengths := []int{6, 8, 10, 12, 14}
	if opts != nil && len(opts.AllowedLengths) > 0 {
		allowedLengths = opts.AllowedLengths
	}

	lengthOK := false
	maxAllowedLength := 0
	for _, length := range allowedLengths {
		if len(resultString) == length {
			lengthOK = true
			break
		}
		if length > maxAllowedLength {
			maxAllowedLength = length
		}
	}
	if !lengthOK && len(resultString) > maxAllowedLength {
		lengthOK = true
	}
	if !lengthOK {
		return nil, zxinggo.ErrFormat
	}

	res := zxinggo.NewResult(
		resultString, nil,
		[]zxinggo.ResultPoint{
			{X: float64(startRange[1]), Y: float64(rowNumber)},
			{X: float64(endRange[0]), Y: float64(rowNumber)},
		},
		zxinggo.FormatITF,
	)
	res.PutMetadata(zxinggo.MetadataSymbologyIdentifier, "]I0")
	return res, nil
}

func (r *ITFReader) decodeMiddle(row *bitutil.BitArray, payloadStart, payloadEnd int, result *strings.Builder) error {
	counterDigitPair := make([]int, 10)
	counterBlack := make([]int, 5)
	counterWhite := make([]int, 5)

	for payloadStart < payloadEnd {
		if err := RecordPattern(row, payloadStart, counterDigitPair); err != nil {
			return err
		}

		for k := 0; k < 5; k++ {
			twoK := 2 * k
			counterBlack[k] = counterDigitPair[twoK]
			counterWhite[k] = counterDigitPair[twoK+1]
		}

		bestMatch, err := decodeITFDigit(counterBlack)
		if err != nil {
			return err
		}
		result.WriteByte('0' + byte(bestMatch))

		bestMatch, err = decodeITFDigit(counterWhite)
		if err != nil {
			return err
		}
		result.WriteByte('0' + byte(bestMatch))

		for _, count := range counterDigitPair {
			payloadStart += count
		}
	}
	return nil
}

func (r *ITFReader) decodeStart(row *bitutil.BitArray) ([2]int, error) {
	endStart, err := skipWhiteSpace(row)
	if err != nil {
		return [2]int{}, err
	}

	startRange, err := findITFGuardPattern(row, endStart, itfStartPattern)
	if err != nil {
		return [2]int{}, err
	}

	r.narrowLineWidth = (startRange[1] - startRange[0]) / 4

	err = r.validateQuietZone(row, startRange[0])
	if err != nil {
		return [2]int{}, err
	}

	return startRange, nil
}

func (r *ITFReader) validateQuietZone(row *bitutil.BitArray, startPattern int) error {
	quietZoneSize := r.narrowLineWidth * 10
	if quietZoneSize < 1 {
		quietZoneSize = 1
	}
	quietStart := startPattern - quietZoneSize
	if quietStart < 0 {
		quietStart = 0
	}
	if !row.IsRange(quietStart, startPattern, false) {
		return zxinggo.ErrNotFound
	}
	return nil
}

func (r *ITFReader) decodeEnd(row *bitutil.BitArray) ([2]int, error) {
	// For end pattern, we scan from the end backwards.
	row.Reverse()
	defer row.Reverse()

	endStart, err := skipWhiteSpace(row)
	if err != nil {
		return [2]int{}, err
	}

	// Try 2x end pattern first, fall back to 3x
	endRange, err := findITFGuardPattern(row, endStart, itfEndPatternReversed[0])
	if err != nil {
		endRange, err = findITFGuardPattern(row, endStart, itfEndPatternReversed[1])
		if err != nil {
			return [2]int{}, err
		}
	}

	if err := r.validateQuietZone(row, endRange[0]); err != nil {
		return [2]int{}, err
	}

	// Now un-reverse the coordinates
	temp := row.Size() - endRange[0]
	endRange[0] = row.Size() - endRange[1]
	endRange[1] = temp

	return endRange, nil
}

func skipWhiteSpace(row *bitutil.BitArray) (int, error) {
	width := row.Size()
	endStart := row.GetNextSet(0)
	if endStart == width {
		return 0, zxinggo.ErrNotFound
	}
	return endStart, nil
}

func findITFGuardPattern(row *bitutil.BitArray, rowOffset int, pattern []int) ([2]int, error) {
	patternLength := len(pattern)
	counters := make([]int, patternLength)
	width := row.Size()
	isWhite := false

	counterPosition := 0
	patternStart := rowOffset
	for x := rowOffset; x < width; x++ {
		if row.Get(x) != isWhite {
			counters[counterPosition]++
		} else {
			if counterPosition == patternLength-1 {
				if PatternMatchVariance(counters, pattern, itfMaxIndividualVariance2x) < itfMaxAvgVariance {
					return [2]int{patternStart, x}, nil
				}
				patternStart += counters[0] + counters[1]
				copy(counters, counters[2:counterPosition+1])
				counters[counterPosition-1] = 0
				counters[counterPosition] = 0
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

func decodeITFDigit(counters []int) (int, error) {
	bestVariance := float64(itfMaxAvgVariance)
	bestMatch := -1
	for i := 0; i < 20; i++ {
		pattern := itfPatterns[i]
		maxVariance := float64(itfMaxIndividualVariance2x)
		if i > 9 {
			maxVariance = itfMaxIndividualVariance3x
		}
		variance := PatternMatchVariance(counters, pattern[:], maxVariance)
		if variance < bestVariance {
			bestVariance = variance
			bestMatch = i
		} else if variance == bestVariance {
			bestMatch = -1 // ambiguous match
		}
	}
	if bestMatch >= 0 {
		return bestMatch % 10, nil
	}
	return -1, zxinggo.ErrNotFound
}

// Ensure ITFReader implements RowDecoder at compile time.
var _ RowDecoder = (*ITFReader)(nil)

