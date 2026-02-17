// Package oned implements one-dimensional barcode reading and writing.
package oned

import (
	"math"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// RowDecoder decodes a single row of a 1D barcode.
type RowDecoder interface {
	// DecodeRow attempts to decode a barcode from a single row.
	DecodeRow(rowNumber int, row *bitutil.BitArray, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error)
}

// DecodeOneD decodes a 1D barcode from an image by scanning rows from the
// middle outward. It tries each row forward and reversed.
func DecodeOneD(image *zxinggo.BinaryBitmap, decoder RowDecoder, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	width := image.Width()
	height := image.Height()
	row := bitutil.NewBitArray(width)

	tryHarder := opts != nil && opts.TryHarder
	rowStep := height >> 5
	if tryHarder {
		rowStep = height >> 8
	}
	if rowStep < 1 {
		rowStep = 1
	}

	maxLines := 15
	if tryHarder {
		maxLines = height
	}

	middle := height / 2
	for x := 0; x < maxLines; x++ {
		rowStepsAboveOrBelow := (x + 1) / 2
		isAbove := (x & 0x01) == 0
		rowNumber := middle
		if isAbove {
			rowNumber += rowStep * rowStepsAboveOrBelow
		} else {
			rowNumber -= rowStep * rowStepsAboveOrBelow
		}
		if rowNumber < 0 || rowNumber >= height {
			break
		}

		var err error
		row, err = image.BlackRow(rowNumber, row)
		if err != nil {
			continue
		}

		for attempt := 0; attempt < 2; attempt++ {
			if attempt == 1 {
				row.Reverse()
			}
			result, err := decoder.DecodeRow(rowNumber, row, opts)
			if err != nil {
				continue
			}
			if attempt == 1 {
				result.PutMetadata(zxinggo.MetadataOrientation, 180)
				if result.Points != nil && len(result.Points) >= 2 {
					result.Points[0] = zxinggo.ResultPoint{
						X: float64(width) - result.Points[0].X - 1,
						Y: result.Points[0].Y,
					}
					result.Points[1] = zxinggo.ResultPoint{
						X: float64(width) - result.Points[1].X - 1,
						Y: result.Points[1].Y,
					}
				}
			}
			return result, nil
		}
	}
	return nil, zxinggo.ErrNotFound
}

// RecordPattern records the widths of successive runs of black and white
// pixels in a row, starting at the given position.
func RecordPattern(row *bitutil.BitArray, start int, counters []int) error {
	numCounters := len(counters)
	for i := range counters {
		counters[i] = 0
	}
	end := row.Size()
	if start >= end {
		return zxinggo.ErrNotFound
	}
	isWhite := !row.Get(start)
	counterPosition := 0
	i := start
	for i < end {
		if row.Get(i) != isWhite {
			counters[counterPosition]++
		} else {
			counterPosition++
			if counterPosition == numCounters {
				break
			}
			counters[counterPosition] = 1
			isWhite = !isWhite
		}
		i++
	}
	if !(counterPosition == numCounters || (counterPosition == numCounters-1 && i == end)) {
		return zxinggo.ErrNotFound
	}
	return nil
}

// RecordPatternInReverse records a pattern by first walking backwards to find
// the start of the pattern, then recording forward.
func RecordPatternInReverse(row *bitutil.BitArray, start int, counters []int) error {
	numTransitionsLeft := len(counters)
	last := row.Get(start)
	for start > 0 && numTransitionsLeft >= 0 {
		start--
		if row.Get(start) != last {
			numTransitionsLeft--
			last = !last
		}
	}
	if numTransitionsLeft >= 0 {
		return zxinggo.ErrNotFound
	}
	return RecordPattern(row, start+1, counters)
}

// PatternMatchVariance determines how closely observed counter widths match
// a target pattern. Returns the ratio of total variance to pattern size.
// Returns +Inf if any individual counter exceeds maxIndividualVariance.
func PatternMatchVariance(counters []int, pattern []int, maxIndividualVariance float64) float64 {
	numCounters := len(counters)
	total := 0
	patternLength := 0
	for i := 0; i < numCounters; i++ {
		total += counters[i]
		patternLength += pattern[i]
	}
	if total < patternLength {
		return math.Inf(1)
	}

	unitBarWidth := float64(total) / float64(patternLength)
	maxIndividualVariance *= unitBarWidth

	totalVariance := 0.0
	for i := 0; i < numCounters; i++ {
		counter := float64(counters[i])
		scaledPattern := float64(pattern[i]) * unitBarWidth
		variance := counter - scaledPattern
		if variance < 0 {
			variance = -variance
		}
		if variance > maxIndividualVariance {
			return math.Inf(1)
		}
		totalVariance += variance
	}
	return totalVariance / float64(total)
}
