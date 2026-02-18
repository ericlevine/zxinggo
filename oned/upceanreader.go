package oned

import (
	"strings"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

const (
	upceanMaxAvgVariance        = 0.48
	upceanMaxIndividualVariance = 0.7
)

// UPC/EAN guard patterns.
var (
	UPCEANStartEndPattern = []int{1, 1, 1}
	UPCEANMiddlePattern   = []int{1, 1, 1, 1, 1}
	UPCEANEndPattern      = []int{1, 1, 1, 1, 1, 1}
)

// LPatterns contains the "odd" or "L" patterns for encoding UPC/EAN digits.
var LPatterns = [10][]int{
	{3, 2, 1, 1}, // 0
	{2, 2, 2, 1}, // 1
	{2, 1, 2, 2}, // 2
	{1, 4, 1, 1}, // 3
	{1, 1, 3, 2}, // 4
	{1, 2, 3, 1}, // 5
	{1, 1, 1, 4}, // 6
	{1, 3, 1, 2}, // 7
	{1, 2, 1, 3}, // 8
	{3, 1, 1, 2}, // 9
}

// LAndGPatterns includes both the L and G patterns.
// Indices 0-9 are L patterns, 10-19 are G patterns (reversed L patterns).
var LAndGPatterns [20][]int

func init() {
	for i := 0; i < 10; i++ {
		LAndGPatterns[i] = LPatterns[i]
	}
	for i := 10; i < 20; i++ {
		widths := LPatterns[i-10]
		reversed := make([]int, len(widths))
		for j := 0; j < len(widths); j++ {
			reversed[j] = widths[len(widths)-j-1]
		}
		LAndGPatterns[i] = reversed
	}
}

// UPCEANMiddleDecoder decodes the middle portion of a UPC/EAN barcode.
type UPCEANMiddleDecoder interface {
	// DecodeMiddle decodes the middle portion of the barcode.
	// Returns the row offset after the middle, and the decoded digits are appended to result.
	DecodeMiddle(row *bitutil.BitArray, startRange [2]int, result *strings.Builder) (int, error)

	// BarcodeFormat returns the format this decoder handles.
	BarcodeFormat() zxinggo.Format
}

// DecodeUPCEAN decodes a UPC/EAN barcode from a row using the given middle decoder.
func DecodeUPCEAN(rowNumber int, row *bitutil.BitArray, decoder UPCEANMiddleDecoder, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	startRange, err := findUPCEANStartGuardPattern(row)
	if err != nil {
		return nil, err
	}

	var result strings.Builder
	endStart, err := decoder.DecodeMiddle(row, startRange, &result)
	if err != nil {
		return nil, err
	}

	endRange, err := findUPCEANEndGuardPattern(row, endStart, decoder.BarcodeFormat())
	if err != nil {
		return nil, err
	}

	// Quiet zone check after barcode
	end := endRange[1]
	quietEnd := end + (end - endRange[0])
	if quietEnd >= row.Size() || !row.IsRange(end, quietEnd, false) {
		return nil, zxinggo.ErrNotFound
	}

	resultString := result.String()
	if len(resultString) < 8 {
		return nil, zxinggo.ErrFormat
	}

	format := decoder.BarcodeFormat()
	checksumStr := resultString
	if format == zxinggo.FormatUPCE {
		checksumStr = ConvertUPCEtoUPCA(resultString)
	}
	if !CheckStandardUPCEANChecksum(checksumStr) {
		return nil, zxinggo.ErrChecksum
	}
	left := float64(startRange[1]+startRange[0]) / 2.0
	right := float64(endRange[1]+endRange[0]) / 2.0
	res := zxinggo.NewResult(
		resultString, nil,
		[]zxinggo.ResultPoint{
			{X: left, Y: float64(rowNumber)},
			{X: right, Y: float64(rowNumber)},
		},
		format,
	)

	symbologyID := "0"
	if format == zxinggo.FormatEAN8 {
		symbologyID = "4"
	}
	res.PutMetadata(zxinggo.MetadataSymbologyIdentifier, "]E"+symbologyID)
	return res, nil
}

// CheckStandardUPCEANChecksum verifies the UPC/EAN checksum.
func CheckStandardUPCEANChecksum(s string) bool {
	length := len(s)
	if length == 0 {
		return false
	}
	check := int(s[length-1] - '0')
	return GetStandardUPCEANChecksum(s[:length-1]) == check
}

// GetStandardUPCEANChecksum computes the UPC/EAN check digit for a string of digits
// (without the check digit itself).
func GetStandardUPCEANChecksum(s string) int {
	length := len(s)
	sum := 0
	for i := length - 1; i >= 0; i -= 2 {
		d := int(s[i] - '0')
		if d < 0 || d > 9 {
			return -1
		}
		sum += d
	}
	sum *= 3
	for i := length - 2; i >= 0; i -= 2 {
		d := int(s[i] - '0')
		if d < 0 || d > 9 {
			return -1
		}
		sum += d
	}
	return (1000 - sum) % 10
}

func findUPCEANStartGuardPattern(row *bitutil.BitArray) ([2]int, error) {
	counters := make([]int, len(UPCEANStartEndPattern))
	nextStart := 0
	for {
		for i := range counters {
			counters[i] = 0
		}
		startRange, err := findUPCEANGuardPattern(row, nextStart, false, UPCEANStartEndPattern, counters)
		if err != nil {
			return [2]int{}, err
		}
		start := startRange[0]
		nextStart = startRange[1]
		quietStart := start - (nextStart - start)
		if quietStart >= 0 && row.IsRange(quietStart, start, false) {
			return startRange, nil
		}
	}
}

func findUPCEANEndGuardPattern(row *bitutil.BitArray, endStart int, format zxinggo.Format) ([2]int, error) {
	if format == zxinggo.FormatUPCE {
		return findUPCEANGuardPattern(row, endStart, true, UPCEANEndPattern, make([]int, len(UPCEANEndPattern)))
	}
	return findUPCEANGuardPattern(row, endStart, false, UPCEANStartEndPattern, make([]int, len(UPCEANStartEndPattern)))
}

func findUPCEANGuardPattern(row *bitutil.BitArray, rowOffset int, whiteFirst bool, pattern, counters []int) ([2]int, error) {
	width := row.Size()
	if whiteFirst {
		rowOffset = row.GetNextUnset(rowOffset)
	} else {
		rowOffset = row.GetNextSet(rowOffset)
	}
	counterPosition := 0
	patternStart := rowOffset
	patternLength := len(pattern)
	isWhite := whiteFirst

	for x := rowOffset; x < width; x++ {
		if row.Get(x) != isWhite {
			counters[counterPosition]++
		} else {
			if counterPosition == patternLength-1 {
				if PatternMatchVariance(counters, pattern, upceanMaxIndividualVariance) < upceanMaxAvgVariance {
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

// FindUPCEANMiddleGuardPattern finds the middle guard pattern.
func FindUPCEANMiddleGuardPattern(row *bitutil.BitArray, rowOffset int) ([2]int, error) {
	return findUPCEANGuardPattern(row, rowOffset, true, UPCEANMiddlePattern, make([]int, len(UPCEANMiddlePattern)))
}

// DecodeUPCEANDigit attempts to decode a single UPC/EAN-encoded digit.
func DecodeUPCEANDigit(row *bitutil.BitArray, counters []int, rowOffset int, patterns [][]int) (int, error) {
	if err := RecordPattern(row, rowOffset, counters); err != nil {
		return 0, err
	}
	bestVariance := upceanMaxAvgVariance
	bestMatch := -1
	for i, pattern := range patterns {
		variance := PatternMatchVariance(counters, pattern, upceanMaxIndividualVariance)
		if variance < bestVariance {
			bestVariance = variance
			bestMatch = i
		}
	}
	if bestMatch >= 0 {
		return bestMatch, nil
	}
	return 0, zxinggo.ErrNotFound
}
