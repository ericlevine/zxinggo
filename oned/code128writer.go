package oned

import (
	"fmt"
	"strconv"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// Escape characters used to specify FNC codes in Code 128 input.
const (
	Code128EscapeFNC1 = '\u00f1'
	Code128EscapeFNC2 = '\u00f2'
	Code128EscapeFNC3 = '\u00f3'
	Code128EscapeFNC4 = '\u00f4'
)

// Code128Writer encodes Code 128 barcodes.
type Code128Writer struct{}

// NewCode128Writer creates a new Code 128 writer.
func NewCode128Writer() *Code128Writer {
	return &Code128Writer{}
}

// Encode encodes the given contents into a Code 128 barcode BitMatrix.
func (w *Code128Writer) Encode(contents string, format zxinggo.Format, width, height int, opts *zxinggo.EncodeOptions) (*bitutil.BitMatrix, error) {
	if format != zxinggo.FormatCode128 {
		return nil, fmt.Errorf("can only encode CODE_128, but got %s", format)
	}

	forcedCodeSet := -1
	if opts != nil && opts.ForceCodeSet != "" {
		switch opts.ForceCodeSet {
		case "A":
			forcedCodeSet = code128CodeA
		case "B":
			forcedCodeSet = code128CodeB
		case "C":
			forcedCodeSet = code128CodeC
		default:
			return nil, fmt.Errorf("unsupported code set hint: %s", opts.ForceCodeSet)
		}
	}

	if err := checkCode128Contents(contents, forcedCodeSet); err != nil {
		return nil, err
	}

	code, err := encodeCode128Fast(contents, forcedCodeSet)
	if err != nil {
		return nil, err
	}
	return RenderOneDCode(code, width, height), nil
}

func checkCode128Contents(contents string, forcedCodeSet int) error {
	for i := 0; i < len(contents); i++ {
		c := rune(contents[i])
		switch c {
		case Code128EscapeFNC1, Code128EscapeFNC2, Code128EscapeFNC3, Code128EscapeFNC4:
			// OK
		default:
			if c > 127 {
				return fmt.Errorf("bad character in input: ASCII value=%d", c)
			}
		}
		switch forcedCodeSet {
		case code128CodeA:
			if c > 95 && c <= 127 {
				return fmt.Errorf("bad character in input for forced code set A: ASCII value=%d", c)
			}
		case code128CodeB:
			if c < 32 {
				return fmt.Errorf("bad character in input for forced code set B: ASCII value=%d", c)
			}
		case code128CodeC:
			if (c < 48 || (c > 57 && c <= 127)) || c == Code128EscapeFNC2 || c == Code128EscapeFNC3 || c == Code128EscapeFNC4 {
				return fmt.Errorf("bad character in input for forced code set C: ASCII value=%d", c)
			}
		}
	}
	return nil
}

// code128CType classifies characters for Code C lookahead.
type code128CType int

const (
	code128Uncodable  code128CType = iota
	code128OneDigit
	code128TwoDigits
	code128FNC1Found
)

func findCode128CType(value string, start int) code128CType {
	last := len(value)
	if start >= last {
		return code128Uncodable
	}
	c := rune(value[start])
	if c == Code128EscapeFNC1 {
		return code128FNC1Found
	}
	if c < '0' || c > '9' {
		return code128Uncodable
	}
	if start+1 >= last {
		return code128OneDigit
	}
	c = rune(value[start+1])
	if c < '0' || c > '9' {
		return code128OneDigit
	}
	return code128TwoDigits
}

func chooseCode128(value string, start, oldCode int) int {
	lookahead := findCode128CType(value, start)
	if lookahead == code128OneDigit {
		if oldCode == code128CodeA {
			return code128CodeA
		}
		return code128CodeB
	}
	if lookahead == code128Uncodable {
		if start < len(value) {
			c := rune(value[start])
			if c < ' ' || (oldCode == code128CodeA && (c < '`' || (c >= Code128EscapeFNC1 && c <= Code128EscapeFNC4))) {
				return code128CodeA
			}
		}
		return code128CodeB
	}
	if oldCode == code128CodeA && lookahead == code128FNC1Found {
		return code128CodeA
	}
	if oldCode == code128CodeC {
		return code128CodeC
	}
	if oldCode == code128CodeB {
		if lookahead == code128FNC1Found {
			return code128CodeB
		}
		lookahead = findCode128CType(value, start+2)
		if lookahead == code128Uncodable || lookahead == code128OneDigit {
			return code128CodeB
		}
		if lookahead == code128FNC1Found {
			lookahead = findCode128CType(value, start+3)
			if lookahead == code128TwoDigits {
				return code128CodeC
			}
			return code128CodeB
		}
		index := start + 4
		for findCode128CType(value, index) == code128TwoDigits {
			index += 2
		}
		if findCode128CType(value, index) == code128OneDigit {
			return code128CodeB
		}
		return code128CodeC
	}
	// oldCode == 0: choosing initial code
	if lookahead == code128FNC1Found {
		lookahead = findCode128CType(value, start+1)
	}
	if lookahead == code128TwoDigits {
		return code128CodeC
	}
	return code128CodeB
}

func encodeCode128Fast(contents string, forcedCodeSet int) ([]bool, error) {
	length := len(contents)
	var patterns [][]int
	checkSum := 0
	checkWeight := 1
	codeSet := 0
	position := 0

	for position < length {
		var newCodeSet int
		if forcedCodeSet == -1 {
			newCodeSet = chooseCode128(contents, position, codeSet)
		} else {
			newCodeSet = forcedCodeSet
		}

		var patternIndex int
		if newCodeSet == codeSet {
			c := rune(contents[position])
			switch c {
			case Code128EscapeFNC1:
				patternIndex = code128FNC1
			case Code128EscapeFNC2:
				patternIndex = code128FNC2
			case Code128EscapeFNC3:
				patternIndex = code128FNC3
			case Code128EscapeFNC4:
				if codeSet == code128CodeA {
					patternIndex = code128FNC4A
				} else {
					patternIndex = code128FNC4B
				}
			default:
				switch codeSet {
				case code128CodeA:
					patternIndex = int(c) - ' '
					if patternIndex < 0 {
						patternIndex += '`'
					}
				case code128CodeB:
					patternIndex = int(c) - ' '
				default: // code C
					if position+1 == length {
						return nil, fmt.Errorf("bad number of characters for digit only encoding")
					}
					val, err := strconv.Atoi(contents[position : position+2])
					if err != nil {
						return nil, err
					}
					patternIndex = val
					position++
				}
			}
			position++
		} else {
			if codeSet == 0 {
				switch newCodeSet {
				case code128CodeA:
					patternIndex = code128StartA
				case code128CodeB:
					patternIndex = code128StartB
				default:
					patternIndex = code128StartC
				}
			} else {
				patternIndex = newCodeSet
			}
			codeSet = newCodeSet
		}

		patterns = append(patterns, Code128Patterns[patternIndex])
		checkSum += patternIndex * checkWeight
		if position != 0 {
			checkWeight++
		}
	}

	return produceCode128Result(patterns, checkSum), nil
}

func produceCode128Result(patterns [][]int, checkSum int) []bool {
	checkSum %= 103
	patterns = append(patterns, Code128Patterns[checkSum])
	patterns = append(patterns, Code128Patterns[code128Stop])

	codeWidth := 0
	for _, pattern := range patterns {
		for _, w := range pattern {
			codeWidth += w
		}
	}

	result := make([]bool, codeWidth)
	pos := 0
	for _, pattern := range patterns {
		pos += AppendPattern(result, pos, pattern, true)
	}
	return result
}
