package oned

import (
	"fmt"
	"strings"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// Code93Writer encodes Code 93 barcodes.
type Code93Writer struct{}

// NewCode93Writer creates a new Code 93 writer.
func NewCode93Writer() *Code93Writer {
	return &Code93Writer{}
}

// Encode encodes the given contents into a Code 93 barcode BitMatrix.
func (w *Code93Writer) Encode(contents string, format zxinggo.Format, width, height int, opts *zxinggo.EncodeOptions) (*bitutil.BitMatrix, error) {
	if format != zxinggo.FormatCode93 {
		return nil, fmt.Errorf("can only encode CODE_93, but got %s", format)
	}
	code, err := w.encode(contents)
	if err != nil {
		return nil, err
	}
	return RenderOneDCode(code, width, height), nil
}

func (w *Code93Writer) encode(contents string) ([]bool, error) {
	contents = code93ConvertToExtended(contents)
	length := len(contents)
	if length > 80 {
		return nil, fmt.Errorf("requested contents should be less than 80 digits long after converting to extended encoding, but got %d", length)
	}

	// length of code + 2 start/stop characters + 2 checksums, each of 9 bits, plus a termination bar
	codeWidth := (length + 2 + 2) * 9 + 1
	result := make([]bool, codeWidth)

	// start character (*)
	pos := code93AppendPattern(result, 0, code93AsteriskEncoding)

	for i := 0; i < length; i++ {
		indexInString := strings.IndexByte(code93AlphabetString, contents[i])
		pos += code93AppendPattern(result, pos, code93CharacterEncodings[indexInString])
	}

	// add two checksums
	check1 := code93ComputeChecksumIndex(contents, 20)
	pos += code93AppendPattern(result, pos, code93CharacterEncodings[check1])

	contents += string(code93AlphabetString[check1])

	check2 := code93ComputeChecksumIndex(contents, 15)
	pos += code93AppendPattern(result, pos, code93CharacterEncodings[check2])

	// end character (*)
	pos += code93AppendPattern(result, pos, code93AsteriskEncoding)

	// termination bar (single black bar)
	result[pos] = true

	return result, nil
}

func code93AppendPattern(target []bool, pos int, a int) int {
	for i := 0; i < 9; i++ {
		if a&(1<<uint(8-i)) != 0 {
			target[pos+i] = true
		}
	}
	return 9
}

func code93ComputeChecksumIndex(contents string, maxWeight int) int {
	weight := 1
	total := 0
	for i := len(contents) - 1; i >= 0; i-- {
		indexInString := strings.IndexByte(code93AlphabetString, contents[i])
		total += indexInString * weight
		weight++
		if weight > maxWeight {
			weight = 1
		}
	}
	return total % 47
}

func code93ConvertToExtended(contents string) string {
	length := len(contents)
	var ext strings.Builder
	ext.Grow(length * 2)
	for i := 0; i < length; i++ {
		c := contents[i]
		if c == 0 {
			ext.WriteString("bU")
		} else if c <= 26 {
			ext.WriteByte('a')
			ext.WriteByte('A' + c - 1)
		} else if c <= 31 {
			ext.WriteByte('b')
			ext.WriteByte('A' + c - 27)
		} else if c == ' ' || c == '$' || c == '%' || c == '+' {
			ext.WriteByte(c)
		} else if c <= ',' {
			ext.WriteByte('c')
			ext.WriteByte('A' + c - '!')
		} else if c <= '9' {
			ext.WriteByte(c)
		} else if c == ':' {
			ext.WriteString("cZ")
		} else if c <= '?' {
			ext.WriteByte('b')
			ext.WriteByte('F' + c - ';')
		} else if c == '@' {
			ext.WriteString("bV")
		} else if c <= 'Z' {
			ext.WriteByte(c)
		} else if c <= '_' {
			ext.WriteByte('b')
			ext.WriteByte('K' + c - '[')
		} else if c == '`' {
			ext.WriteString("bW")
		} else if c <= 'z' {
			ext.WriteByte('d')
			ext.WriteByte('A' + c - 'a')
		} else if c <= 127 {
			ext.WriteByte('b')
			ext.WriteByte('P' + c - '{')
		} else {
			// non-encodable; skip
			ext.WriteByte(c)
		}
	}
	return ext.String()
}
