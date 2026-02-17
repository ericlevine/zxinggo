package oned

import (
	"fmt"
	"math"
	"strings"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// Code128 constants
const (
	code128MaxAvgVariance        = 0.25
	code128MaxIndividualVariance = 0.7

	code128Shift  = 98
	code128CodeC  = 99
	code128CodeB  = 100
	code128CodeA  = 101
	code128FNC1   = 102
	code128FNC2   = 97
	code128FNC3   = 96
	code128FNC4A  = 101
	code128FNC4B  = 100
	code128StartA = 103
	code128StartB = 104
	code128StartC = 105
	code128Stop   = 106
)

// Code128Patterns contains the bar patterns for Code 128.
var Code128Patterns = [107][]int{
	{2, 1, 2, 2, 2, 2}, // 0
	{2, 2, 2, 1, 2, 2},
	{2, 2, 2, 2, 2, 1},
	{1, 2, 1, 2, 2, 3},
	{1, 2, 1, 3, 2, 2},
	{1, 3, 1, 2, 2, 2}, // 5
	{1, 2, 2, 2, 1, 3},
	{1, 2, 2, 3, 1, 2},
	{1, 3, 2, 2, 1, 2},
	{2, 2, 1, 2, 1, 3},
	{2, 2, 1, 3, 1, 2}, // 10
	{2, 3, 1, 2, 1, 2},
	{1, 1, 2, 2, 3, 2},
	{1, 2, 2, 1, 3, 2},
	{1, 2, 2, 2, 3, 1},
	{1, 1, 3, 2, 2, 2}, // 15
	{1, 2, 3, 1, 2, 2},
	{1, 2, 3, 2, 2, 1},
	{2, 2, 3, 2, 1, 1},
	{2, 2, 1, 1, 3, 2},
	{2, 2, 1, 2, 3, 1}, // 20
	{2, 1, 3, 2, 1, 2},
	{2, 2, 3, 1, 1, 2},
	{3, 1, 2, 1, 3, 1},
	{3, 1, 1, 2, 2, 2},
	{3, 2, 1, 1, 2, 2}, // 25
	{3, 2, 1, 2, 2, 1},
	{3, 1, 2, 2, 1, 2},
	{3, 2, 2, 1, 1, 2},
	{3, 2, 2, 2, 1, 1},
	{2, 1, 2, 1, 2, 3}, // 30
	{2, 1, 2, 3, 2, 1},
	{2, 3, 2, 1, 2, 1},
	{1, 1, 1, 3, 2, 3},
	{1, 3, 1, 1, 2, 3},
	{1, 3, 1, 3, 2, 1}, // 35
	{1, 1, 2, 3, 1, 3},
	{1, 3, 2, 1, 1, 3},
	{1, 3, 2, 3, 1, 1},
	{2, 1, 1, 3, 1, 3},
	{2, 3, 1, 1, 1, 3}, // 40
	{2, 3, 1, 3, 1, 1},
	{1, 1, 2, 1, 3, 3},
	{1, 1, 2, 3, 3, 1},
	{1, 3, 2, 1, 3, 1},
	{1, 1, 3, 1, 2, 3}, // 45
	{1, 1, 3, 3, 2, 1},
	{1, 3, 3, 1, 2, 1},
	{3, 1, 3, 1, 2, 1},
	{2, 1, 1, 3, 3, 1},
	{2, 3, 1, 1, 3, 1}, // 50
	{2, 1, 3, 1, 1, 3},
	{2, 1, 3, 3, 1, 1},
	{2, 1, 3, 1, 3, 1},
	{3, 1, 1, 1, 2, 3},
	{3, 1, 1, 3, 2, 1}, // 55
	{3, 3, 1, 1, 2, 1},
	{3, 1, 2, 1, 1, 3},
	{3, 1, 2, 3, 1, 1},
	{3, 3, 2, 1, 1, 1},
	{3, 1, 4, 1, 1, 1}, // 60
	{2, 2, 1, 4, 1, 1},
	{4, 3, 1, 1, 1, 1},
	{1, 1, 1, 2, 2, 4},
	{1, 1, 1, 4, 2, 2},
	{1, 2, 1, 1, 2, 4}, // 65
	{1, 2, 1, 4, 2, 1},
	{1, 4, 1, 1, 2, 2},
	{1, 4, 1, 2, 2, 1},
	{1, 1, 2, 2, 1, 4},
	{1, 1, 2, 4, 1, 2}, // 70
	{1, 2, 2, 1, 1, 4},
	{1, 2, 2, 4, 1, 1},
	{1, 4, 2, 1, 1, 2},
	{1, 4, 2, 2, 1, 1},
	{2, 4, 1, 2, 1, 1}, // 75
	{2, 2, 1, 1, 1, 4},
	{4, 1, 3, 1, 1, 1},
	{2, 4, 1, 1, 1, 2},
	{1, 3, 4, 1, 1, 1},
	{1, 1, 1, 2, 4, 2}, // 80
	{1, 2, 1, 1, 4, 2},
	{1, 2, 1, 2, 4, 1},
	{1, 1, 4, 2, 1, 2},
	{1, 2, 4, 1, 1, 2},
	{1, 2, 4, 2, 1, 1}, // 85
	{4, 1, 1, 2, 1, 2},
	{4, 2, 1, 1, 1, 2},
	{4, 2, 1, 2, 1, 1},
	{2, 1, 2, 1, 4, 1},
	{2, 1, 4, 1, 2, 1}, // 90
	{4, 1, 2, 1, 2, 1},
	{1, 1, 1, 1, 4, 3},
	{1, 1, 1, 3, 4, 1},
	{1, 3, 1, 1, 4, 1},
	{1, 1, 4, 1, 1, 3}, // 95
	{1, 1, 4, 3, 1, 1},
	{4, 1, 1, 1, 1, 3},
	{4, 1, 1, 3, 1, 1},
	{1, 1, 3, 1, 4, 1},
	{1, 1, 4, 1, 3, 1}, // 100
	{3, 1, 1, 1, 4, 1},
	{4, 1, 1, 1, 3, 1},
	{2, 1, 1, 4, 1, 2}, // START_A
	{2, 1, 1, 2, 1, 4}, // START_B
	{2, 1, 1, 2, 3, 2}, // START_C
	{2, 3, 3, 1, 1, 1, 2}, // STOP
}

// Code128Reader decodes Code 128 barcodes.
type Code128Reader struct{}

// NewCode128Reader creates a new Code 128 reader.
func NewCode128Reader() *Code128Reader {
	return &Code128Reader{}
}

// DecodeRow decodes a Code 128 barcode from a single row.
func (r *Code128Reader) DecodeRow(rowNumber int, row *bitutil.BitArray, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	convertFNC1 := opts != nil && opts.AssumeGS1
	symbologyModifier := 0

	startPatternInfo, err := findCode128StartPattern(row)
	if err != nil {
		return nil, err
	}
	startCode := startPatternInfo[2]

	var rawCodes []byte
	rawCodes = append(rawCodes, byte(startCode))

	var codeSet int
	switch startCode {
	case code128StartA:
		codeSet = code128CodeA
	case code128StartB:
		codeSet = code128CodeB
	case code128StartC:
		codeSet = code128CodeC
	default:
		return nil, zxinggo.ErrFormat
	}

	done := false
	isNextShifted := false
	var result strings.Builder
	lastStart := startPatternInfo[0]
	nextStart := startPatternInfo[1]
	counters := make([]int, 6)

	lastCode := 0
	code := 0
	checksumTotal := startCode
	multiplier := 0
	lastCharacterWasPrintable := true
	upperMode := false
	shiftUpperMode := false

	for !done {
		unshift := isNextShifted
		isNextShifted = false
		lastCode = code

		code, err = decodeCode128(row, counters, nextStart)
		if err != nil {
			return nil, err
		}
		rawCodes = append(rawCodes, byte(code))

		if code != code128Stop {
			lastCharacterWasPrintable = true
		}
		if code != code128Stop {
			multiplier++
			checksumTotal += multiplier * code
		}

		lastStart = nextStart
		for _, c := range counters {
			nextStart += c
		}

		switch code {
		case code128StartA, code128StartB, code128StartC:
			return nil, zxinggo.ErrFormat
		}

		switch codeSet {
		case code128CodeA:
			if code < 64 {
				ch := byte(' ' + code)
				if shiftUpperMode == upperMode {
					result.WriteByte(ch)
				} else {
					result.WriteByte(ch + 128)
				}
				shiftUpperMode = false
			} else if code < 96 {
				ch := byte(code - 64)
				if shiftUpperMode == upperMode {
					result.WriteByte(ch)
				} else {
					result.WriteByte(ch + 128)
				}
				shiftUpperMode = false
			} else {
				if code != code128Stop {
					lastCharacterWasPrintable = false
				}
				switch code {
				case code128FNC1:
					if result.Len() == 0 {
						symbologyModifier = 1
					} else if result.Len() == 1 {
						symbologyModifier = 2
					}
					if convertFNC1 {
						if result.Len() == 0 {
							result.WriteString("]C1")
						} else {
							result.WriteByte(29)
						}
					}
				case code128FNC2:
					symbologyModifier = 4
				case code128FNC3:
					// do nothing
				case code128FNC4A:
					if !upperMode && shiftUpperMode {
						upperMode = true
						shiftUpperMode = false
					} else if upperMode && shiftUpperMode {
						upperMode = false
						shiftUpperMode = false
					} else {
						shiftUpperMode = true
					}
				case code128Shift:
					isNextShifted = true
					codeSet = code128CodeB
				case code128CodeB:
					codeSet = code128CodeB
				case code128CodeC:
					codeSet = code128CodeC
				case code128Stop:
					done = true
				}
			}
		case code128CodeB:
			if code < 96 {
				ch := byte(' ' + code)
				if shiftUpperMode == upperMode {
					result.WriteByte(ch)
				} else {
					result.WriteByte(ch + 128)
				}
				shiftUpperMode = false
			} else {
				if code != code128Stop {
					lastCharacterWasPrintable = false
				}
				switch code {
				case code128FNC1:
					if result.Len() == 0 {
						symbologyModifier = 1
					} else if result.Len() == 1 {
						symbologyModifier = 2
					}
					if convertFNC1 {
						if result.Len() == 0 {
							result.WriteString("]C1")
						} else {
							result.WriteByte(29)
						}
					}
				case code128FNC2:
					symbologyModifier = 4
				case code128FNC3:
					// do nothing
				case code128FNC4B:
					if !upperMode && shiftUpperMode {
						upperMode = true
						shiftUpperMode = false
					} else if upperMode && shiftUpperMode {
						upperMode = false
						shiftUpperMode = false
					} else {
						shiftUpperMode = true
					}
				case code128Shift:
					isNextShifted = true
					codeSet = code128CodeA
				case code128CodeA:
					codeSet = code128CodeA
				case code128CodeC:
					codeSet = code128CodeC
				case code128Stop:
					done = true
				}
			}
		case code128CodeC:
			if code < 100 {
				if code < 10 {
					result.WriteByte('0')
				}
				result.WriteString(fmt.Sprintf("%d", code))
			} else {
				if code != code128Stop {
					lastCharacterWasPrintable = false
				}
				switch code {
				case code128FNC1:
					if result.Len() == 0 {
						symbologyModifier = 1
					} else if result.Len() == 1 {
						symbologyModifier = 2
					}
					if convertFNC1 {
						if result.Len() == 0 {
							result.WriteString("]C1")
						} else {
							result.WriteByte(29)
						}
					}
				case code128CodeA:
					codeSet = code128CodeA
				case code128CodeB:
					codeSet = code128CodeB
				case code128Stop:
					done = true
				}
			}
		}

		if unshift {
			if codeSet == code128CodeA {
				codeSet = code128CodeB
			} else {
				codeSet = code128CodeA
			}
		}
	}

	lastPatternSize := nextStart - lastStart

	// Check for whitespace after stop pattern
	nextStart = row.GetNextUnset(nextStart)
	endCheck := nextStart + (nextStart-lastStart)/2
	if endCheck > row.Size() {
		endCheck = row.Size()
	}
	if !row.IsRange(nextStart, endCheck, false) {
		return nil, zxinggo.ErrNotFound
	}

	// Validate checksum
	checksumTotal -= multiplier * lastCode
	if checksumTotal%103 != lastCode {
		return nil, zxinggo.ErrChecksum
	}

	resultLength := result.Len()
	if resultLength == 0 {
		return nil, zxinggo.ErrNotFound
	}

	// Remove check digit from result
	s := result.String()
	if resultLength > 0 && lastCharacterWasPrintable {
		if codeSet == code128CodeC {
			if len(s) >= 2 {
				s = s[:len(s)-2]
			}
		} else {
			if len(s) >= 1 {
				s = s[:len(s)-1]
			}
		}
	}

	left := float64(startPatternInfo[1]+startPatternInfo[0]) / 2.0
	right := float64(lastStart) + float64(lastPatternSize)/2.0

	res := zxinggo.NewResult(
		s, rawCodes,
		[]zxinggo.ResultPoint{
			{X: left, Y: float64(rowNumber)},
			{X: right, Y: float64(rowNumber)},
		},
		zxinggo.FormatCode128,
	)
	res.PutMetadata(zxinggo.MetadataSymbologyIdentifier, fmt.Sprintf("]C%d", symbologyModifier))
	return res, nil
}

func findCode128StartPattern(row *bitutil.BitArray) ([3]int, error) {
	width := row.Size()
	rowOffset := row.GetNextSet(0)

	counterPosition := 0
	counters := make([]int, 6)
	patternStart := rowOffset
	isWhite := false
	patternLength := len(counters)

	for i := rowOffset; i < width; i++ {
		if row.Get(i) != isWhite {
			counters[counterPosition]++
		} else {
			if counterPosition == patternLength-1 {
				bestVariance := code128MaxAvgVariance
				bestMatch := -1
				for startCode := code128StartA; startCode <= code128StartC; startCode++ {
					variance := PatternMatchVariance(counters, Code128Patterns[startCode], code128MaxIndividualVariance)
					if variance < bestVariance {
						bestVariance = variance
						bestMatch = startCode
					}
				}
				if bestMatch >= 0 {
					whiteStart := patternStart - (i-patternStart)/2
					if whiteStart < 0 {
						whiteStart = 0
					}
					if row.IsRange(whiteStart, patternStart, false) {
						return [3]int{patternStart, i, bestMatch}, nil
					}
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
	return [3]int{}, zxinggo.ErrNotFound
}

func decodeCode128(row *bitutil.BitArray, counters []int, rowOffset int) (int, error) {
	if err := RecordPattern(row, rowOffset, counters); err != nil {
		return -1, err
	}
	bestVariance := code128MaxAvgVariance
	bestMatch := -1
	for d := 0; d < len(Code128Patterns); d++ {
		pattern := Code128Patterns[d]
		variance := PatternMatchVariance(counters, pattern, code128MaxIndividualVariance)
		if variance < bestVariance {
			bestVariance = variance
			bestMatch = d
		}
	}
	if bestMatch >= 0 {
		return bestMatch, nil
	}
	return -1, zxinggo.ErrNotFound
}

// suppress unused import warning
var _ = math.Inf
