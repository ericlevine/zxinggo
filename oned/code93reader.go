package oned

import (
	"math"
	"strings"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

const code93AlphabetString = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ-. $/+%abcd*"

var code93CharacterEncodings = [48]int{
	0x114, 0x148, 0x144, 0x142, 0x128, 0x124, 0x122, 0x150, 0x112, 0x10A, // 0-9
	0x1A8, 0x1A4, 0x1A2, 0x194, 0x192, 0x18A, 0x168, 0x164, 0x162, 0x134, // A-J
	0x11A, 0x158, 0x14C, 0x146, 0x12C, 0x116, 0x1B4, 0x1B2, 0x1AC, 0x1A6, // K-T
	0x196, 0x19A, 0x16C, 0x166, 0x136, 0x13A, // U-Z
	0x12E, 0x1D4, 0x1D2, 0x1CA, 0x16E, 0x176, 0x1AE, // - . space $ / + %
	0x126, 0x1DA, 0x1D6, 0x132, 0x15E, // a b c d *
}

var code93AsteriskEncoding = code93CharacterEncodings[47]

// Code93Reader decodes Code 93 barcodes.
type Code93Reader struct {
	counters []int
}

// NewCode93Reader creates a new Code 93 reader.
func NewCode93Reader() *Code93Reader {
	return &Code93Reader{
		counters: make([]int, 6),
	}
}

// DecodeRow decodes a Code 93 barcode from a single row.
func (r *Code93Reader) DecodeRow(rowNumber int, row *bitutil.BitArray, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	start, err := r.findAsteriskPattern(row)
	if err != nil {
		return nil, err
	}
	nextStart := row.GetNextSet(start[1])
	end := row.Size()

	counters := r.counters
	for i := range counters {
		counters[i] = 0
	}

	var result strings.Builder

	var decodedChar byte
	var lastStart int
	for {
		if err := RecordPattern(row, nextStart, counters); err != nil {
			return nil, err
		}
		pattern := code93ToPattern(counters)
		if pattern < 0 {
			return nil, zxinggo.ErrNotFound
		}
		decodedChar, err = code93PatternToChar(pattern)
		if err != nil {
			return nil, err
		}
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
	s := result.String()
	s = s[:len(s)-1] // remove trailing asterisk

	lastPatternSize := 0
	for _, c := range counters {
		lastPatternSize += c
	}

	// Should be at least one more black module
	if nextStart == end || !row.Get(nextStart) {
		return nil, zxinggo.ErrNotFound
	}

	if len(s) < 2 {
		return nil, zxinggo.ErrNotFound
	}

	if err := code93CheckChecksums(s); err != nil {
		return nil, err
	}
	// Remove checksum digits
	s = s[:len(s)-2]

	decoded, err := code93DecodeExtended(s)
	if err != nil {
		return nil, err
	}

	left := float64(start[1]+start[0]) / 2.0
	right := float64(lastStart) + float64(lastPatternSize)/2.0
	res := zxinggo.NewResult(
		decoded, nil,
		[]zxinggo.ResultPoint{
			{X: left, Y: float64(rowNumber)},
			{X: right, Y: float64(rowNumber)},
		},
		zxinggo.FormatCode93,
	)
	res.PutMetadata(zxinggo.MetadataSymbologyIdentifier, "]G0")
	return res, nil
}

func (r *Code93Reader) findAsteriskPattern(row *bitutil.BitArray) ([2]int, error) {
	width := row.Size()
	rowOffset := row.GetNextSet(0)

	counters := r.counters
	for i := range counters {
		counters[i] = 0
	}
	patternStart := rowOffset
	isWhite := false
	patternLength := len(counters)
	counterPosition := 0

	for i := rowOffset; i < width; i++ {
		if row.Get(i) != isWhite {
			counters[counterPosition]++
		} else {
			if counterPosition == patternLength-1 {
				if code93ToPattern(counters) == code93AsteriskEncoding {
					return [2]int{patternStart, i}, nil
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

func code93ToPattern(counters []int) int {
	sum := 0
	for _, c := range counters {
		sum += c
	}
	pattern := 0
	max := len(counters)
	for i := 0; i < max; i++ {
		scaled := int(math.Round(float64(counters[i]) * 9.0 / float64(sum)))
		if scaled < 1 || scaled > 4 {
			return -1
		}
		if (i & 0x01) == 0 {
			for j := 0; j < scaled; j++ {
				pattern = (pattern << 1) | 0x01
			}
		} else {
			pattern <<= uint(scaled)
		}
	}
	return pattern
}

func code93PatternToChar(pattern int) (byte, error) {
	for i, enc := range code93CharacterEncodings {
		if enc == pattern {
			return code93AlphabetString[i], nil
		}
	}
	return 0, zxinggo.ErrNotFound
}

func code93DecodeExtended(encoded string) (string, error) {
	length := len(encoded)
	var decoded strings.Builder
	for i := 0; i < length; i++ {
		c := encoded[i]
		if c >= 'a' && c <= 'd' {
			if i >= length-1 {
				return "", zxinggo.ErrFormat
			}
			next := encoded[i+1]
			var decodedChar byte
			switch c {
			case 'd':
				if next >= 'A' && next <= 'Z' {
					decodedChar = next + 32
				} else {
					return "", zxinggo.ErrFormat
				}
			case 'a':
				if next >= 'A' && next <= 'Z' {
					decodedChar = next - 64
				} else {
					return "", zxinggo.ErrFormat
				}
			case 'b':
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
				} else if next >= 'X' && next <= 'Z' {
					decodedChar = 127
				} else {
					return "", zxinggo.ErrFormat
				}
			case 'c':
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

func code93CheckChecksums(result string) error {
	length := len(result)
	if err := code93CheckOneChecksum(result, length-2, 20); err != nil {
		return err
	}
	return code93CheckOneChecksum(result, length-1, 15)
}

func code93CheckOneChecksum(result string, checkPosition, weightMax int) error {
	weight := 1
	total := 0
	for i := checkPosition - 1; i >= 0; i-- {
		total += weight * strings.IndexByte(code93AlphabetString, result[i])
		weight++
		if weight > weightMax {
			weight = 1
		}
	}
	if result[checkPosition] != code93AlphabetString[total%47] {
		return zxinggo.ErrChecksum
	}
	return nil
}
