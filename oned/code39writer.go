package oned

import (
	"fmt"
	"strings"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// Code39Writer encodes Code 39 barcodes.
type Code39Writer struct{}

// NewCode39Writer creates a new Code 39 writer.
func NewCode39Writer() *Code39Writer {
	return &Code39Writer{}
}

// Encode encodes the given contents into a Code 39 barcode BitMatrix.
func (w *Code39Writer) Encode(contents string, format zxinggo.Format, width, height int, opts *zxinggo.EncodeOptions) (*bitutil.BitMatrix, error) {
	if format != zxinggo.FormatCode39 {
		return nil, fmt.Errorf("can only encode CODE_39, but got %s", format)
	}
	code, err := w.encode(contents)
	if err != nil {
		return nil, err
	}
	return RenderOneDCode(code, width, height), nil
}

func (w *Code39Writer) encode(contents string) ([]bool, error) {
	length := len(contents)
	if length > 80 {
		return nil, fmt.Errorf("requested contents should be less than 80 digits long, but got %d", length)
	}

	// Check if all characters are in the alphabet
	needsExtended := false
	for i := 0; i < length; i++ {
		if strings.IndexByte(code39Alphabet, contents[i]) < 0 {
			needsExtended = true
			break
		}
	}

	if needsExtended {
		contents = tryConvertToCode39Extended(contents)
		length = len(contents)
		if length > 80 {
			return nil, fmt.Errorf("requested contents should be less than 80 digits long, but got %d (extended mode)", length)
		}
	}

	widths := make([]int, 9)
	codeWidth := 24 + 1 + (13 * length)
	result := make([]bool, codeWidth)
	code39ToIntArray(code39AsteriskEncoding, widths)
	pos := AppendPattern(result, 0, widths, true)
	narrowWhite := []int{1}
	pos += AppendPattern(result, pos, narrowWhite, false)

	for i := 0; i < length; i++ {
		idx := strings.IndexByte(code39Alphabet, contents[i])
		code39ToIntArray(code39CharacterEncodings[idx], widths)
		pos += AppendPattern(result, pos, widths, true)
		pos += AppendPattern(result, pos, narrowWhite, false)
	}
	code39ToIntArray(code39AsteriskEncoding, widths)
	AppendPattern(result, pos, widths, true)
	return result, nil
}

func code39ToIntArray(a int, toReturn []int) {
	for i := 0; i < 9; i++ {
		if a&(1<<uint(8-i)) != 0 {
			toReturn[i] = 2
		} else {
			toReturn[i] = 1
		}
	}
}

func tryConvertToCode39Extended(contents string) string {
	var ext strings.Builder
	for i := 0; i < len(contents); i++ {
		c := contents[i]
		switch {
		case c == 0:
			ext.WriteString("%U")
		case c == ' ' || c == '-' || c == '.':
			ext.WriteByte(c)
		case c == '@':
			ext.WriteString("%V")
		case c == '`':
			ext.WriteString("%W")
		case c <= 26:
			ext.WriteByte('$')
			ext.WriteByte('A' + c - 1)
		case c < ' ':
			ext.WriteByte('%')
			ext.WriteByte('A' + c - 27)
		case c <= ',' || c == '/' || c == ':':
			ext.WriteByte('/')
			ext.WriteByte('A' + c - 33)
		case c <= '9':
			ext.WriteByte('0' + c - 48)
		case c <= '?':
			ext.WriteByte('%')
			ext.WriteByte('F' + c - 59)
		case c <= 'Z':
			ext.WriteByte('A' + c - 65)
		case c <= '_':
			ext.WriteByte('%')
			ext.WriteByte('K' + c - 91)
		case c <= 'z':
			ext.WriteByte('+')
			ext.WriteByte('A' + c - 97)
		case c <= 127:
			ext.WriteByte('%')
			ext.WriteByte('P' + c - 123)
		}
	}
	return ext.String()
}
