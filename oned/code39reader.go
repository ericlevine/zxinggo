package oned

import (
	"strings"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

const code39Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ-. $/+%"

var code39CharacterEncodings = [43]int{
	0x034, 0x121, 0x061, 0x160, 0x031, 0x130, 0x070, 0x025, 0x124, 0x064, // 0-9
	0x109, 0x049, 0x148, 0x019, 0x118, 0x058, 0x00D, 0x10C, 0x04C, 0x01C, // A-J
	0x103, 0x043, 0x142, 0x013, 0x112, 0x052, 0x007, 0x106, 0x046, 0x016, // K-T
	0x181, 0x0C1, 0x1C0, 0x091, 0x190, 0x0D0, 0x085, 0x184, 0x0C4, 0x0A8, // U-$
	0x0A2, 0x08A, 0x02A, // /-%
}

const code39AsteriskEncoding = 0x094

// Code39Reader decodes Code 39 barcodes.
type Code39Reader struct {
	usingCheckDigit bool
	extendedMode    bool
}

// NewCode39Reader creates a new Code 39 reader.
func NewCode39Reader() *Code39Reader {
	return &Code39Reader{}
}

// NewCode39ReaderWithCheckDigit creates a Code 39 reader that validates a check digit.
func NewCode39ReaderWithCheckDigit(usingCheckDigit, extendedMode bool) *Code39Reader {
	return &Code39Reader{usingCheckDigit: usingCheckDigit, extendedMode: extendedMode}
}

// DecodeRow decodes a Code 39 barcode from a single row.
func (r *Code39Reader) DecodeRow(rowNumber int, row *bitutil.BitArray, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	counters := make([]int, 9)
	var result strings.Builder

	start, err := findCode39AsteriskPattern(row, counters)
	if err != nil {
		return nil, err
	}
	nextStart := row.GetNextSet(start[1])
	end := row.Size()

	var decodedChar byte
	var lastStart int
	for {
		if err := RecordPattern(row, nextStart, counters); err != nil {
			return nil, err
		}
		pattern := toNarrowWidePattern(counters)
		if pattern < 0 {
			return nil, zxinggo.ErrNotFound
		}
		ch, err := code39PatternToChar(pattern)
		if err != nil {
			return nil, err
		}
		decodedChar = ch
		result.WriteByte(decodedChar)
		lastStart = nextStart
		for _, c := range counters {
			nextStart += c
		}
		nextStart = row.GetNextSet(nextStart)
		if decodedChar == '*' {
			break
		}
	}
	// Remove trailing asterisk
	s := result.String()
	s = s[:len(s)-1]

	lastPatternSize := 0
	for _, c := range counters {
		lastPatternSize += c
	}
	whiteSpaceAfterEnd := nextStart - lastStart - lastPatternSize
	if nextStart != end && whiteSpaceAfterEnd*2 < lastPatternSize {
		return nil, zxinggo.ErrNotFound
	}

	if r.usingCheckDigit || (opts != nil && opts.AssumeCode39CheckDigit) {
		max := len(s) - 1
		total := 0
		for i := 0; i < max; i++ {
			total += strings.IndexByte(code39Alphabet, s[i])
		}
		if s[max] != code39Alphabet[total%43] {
			return nil, zxinggo.ErrChecksum
		}
		s = s[:max]
	}

	if len(s) == 0 {
		return nil, zxinggo.ErrNotFound
	}

	var resultString string
	if r.extendedMode {
		resultString, err = decodeCode39Extended(s)
		if err != nil {
			return nil, err
		}
	} else {
		resultString = s
	}

	left := float64(start[1]+start[0]) / 2.0
	right := float64(lastStart) + float64(lastPatternSize)/2.0
	res := zxinggo.NewResult(
		resultString, nil,
		[]zxinggo.ResultPoint{
			{X: left, Y: float64(rowNumber)},
			{X: right, Y: float64(rowNumber)},
		},
		zxinggo.FormatCode39,
	)
	res.PutMetadata(zxinggo.MetadataSymbologyIdentifier, "]A0")
	return res, nil
}

func findCode39AsteriskPattern(row *bitutil.BitArray, counters []int) ([2]int, error) {
	width := row.Size()
	rowOffset := row.GetNextSet(0)

	counterPosition := 0
	patternStart := rowOffset
	isWhite := false
	patternLength := len(counters)

	for i := rowOffset; i < width; i++ {
		if row.Get(i) != isWhite {
			counters[counterPosition]++
		} else {
			if counterPosition == patternLength-1 {
				if toNarrowWidePattern(counters) == code39AsteriskEncoding {
					whiteStart := patternStart - (i-patternStart)/2
					if whiteStart < 0 {
						whiteStart = 0
					}
					if row.IsRange(whiteStart, patternStart, false) {
						return [2]int{patternStart, i}, nil
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
	return [2]int{}, zxinggo.ErrNotFound
}

func toNarrowWidePattern(counters []int) int {
	numCounters := len(counters)
	maxNarrowCounter := 0
	var wideCounters int
	for {
		minCounter := int(^uint(0) >> 1) // MaxInt
		for _, c := range counters {
			if c < minCounter && c > maxNarrowCounter {
				minCounter = c
			}
		}
		maxNarrowCounter = minCounter
		wideCounters = 0
		totalWideCountersWidth := 0
		pattern := 0
		for i := 0; i < numCounters; i++ {
			if counters[i] > maxNarrowCounter {
				pattern |= 1 << uint(numCounters-1-i)
				wideCounters++
				totalWideCountersWidth += counters[i]
			}
		}
		if wideCounters == 3 {
			for i := 0; i < numCounters && wideCounters > 0; i++ {
				if counters[i] > maxNarrowCounter {
					wideCounters--
					if counters[i]*2 >= totalWideCountersWidth {
						return -1
					}
				}
			}
			return pattern
		}
		if wideCounters <= 3 {
			break
		}
	}
	return -1
}

func code39PatternToChar(pattern int) (byte, error) {
	for i, enc := range code39CharacterEncodings {
		if enc == pattern {
			return code39Alphabet[i], nil
		}
	}
	if pattern == code39AsteriskEncoding {
		return '*', nil
	}
	return 0, zxinggo.ErrNotFound
}

func decodeCode39Extended(encoded string) (string, error) {
	var decoded strings.Builder
	for i := 0; i < len(encoded); i++ {
		c := encoded[i]
		if c == '+' || c == '$' || c == '%' || c == '/' {
			if i+1 >= len(encoded) {
				return "", zxinggo.ErrFormat
			}
			next := encoded[i+1]
			var decodedChar byte
			switch c {
			case '+':
				if next >= 'A' && next <= 'Z' {
					decodedChar = next + 32
				} else {
					return "", zxinggo.ErrFormat
				}
			case '$':
				if next >= 'A' && next <= 'Z' {
					decodedChar = next - 64
				} else {
					return "", zxinggo.ErrFormat
				}
			case '%':
				if next >= 'A' && next <= 'E' {
					decodedChar = next - 38
				} else if next >= 'F' && next <= 'J' {
					decodedChar = next - 11
				} else if next >= 'K' && next <= 'O' {
					decodedChar = next + 16
				} else if next >= 'P' && next <= 'T' {
					decodedChar = next + 43
				} else if next == 'U' {
					decodedChar = 0
				} else if next == 'V' {
					decodedChar = '@'
				} else if next == 'W' {
					decodedChar = '`'
				} else if next == 'X' || next == 'Y' || next == 'Z' {
					decodedChar = 127
				} else {
					return "", zxinggo.ErrFormat
				}
			case '/':
				if next >= 'A' && next <= 'O' {
					decodedChar = next - 32
				} else if next == 'Z' {
					decodedChar = ':'
				} else {
					return "", zxinggo.ErrFormat
				}
			}
			decoded.WriteByte(decodedChar)
			i++
		} else {
			decoded.WriteByte(c)
		}
	}
	return decoded.String(), nil
}
