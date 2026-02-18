package oned

import (
	"fmt"
	"strings"
	"unicode"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// CodabarWriter encodes Codabar barcodes.
// This is a faithful port of the Java ZXing CodaBarWriter.
type CodabarWriter struct{}

// NewCodabarWriter creates a new Codabar writer.
func NewCodabarWriter() *CodabarWriter {
	return &CodabarWriter{}
}

var codabarAltStartEndChars = [4]byte{'T', 'N', '*', 'E'}
var codabarTenLengthChars = [4]byte{'/', ':', '+', '.'}

// Encode encodes the given contents into a Codabar barcode BitMatrix.
func (w *CodabarWriter) Encode(contents string, format zxinggo.Format, width, height int, opts *zxinggo.EncodeOptions) (*bitutil.BitMatrix, error) {
	if format != zxinggo.FormatCodabar {
		return nil, fmt.Errorf("can only encode CODABAR, but got %s", format)
	}
	code, err := w.encode(contents)
	if err != nil {
		return nil, err
	}
	return RenderOneDCode(code, width, height), nil
}

func (w *CodabarWriter) encode(contents string) ([]bool, error) {
	if len(contents) < 2 {
		// Can't have a start/end guard, so tentatively add default guards
		contents = "A" + contents + "A"
	} else {
		// Verify input and calculate decoded length.
		upper := strings.ToUpper(contents)
		firstChar := upper[0]
		lastChar := upper[len(upper)-1]
		startsNormal := codabarArrayContains(codabarStartEndEncoding[:], firstChar)
		endsNormal := codabarArrayContains(codabarStartEndEncoding[:], lastChar)
		startsAlt := codabarArrayContains(codabarAltStartEndChars[:], firstChar)
		endsAlt := codabarArrayContains(codabarAltStartEndChars[:], lastChar)
		if startsNormal {
			if !endsNormal {
				return nil, fmt.Errorf("invalid start/end guards: %s", contents)
			}
			// already has valid start/end, use uppercase
			contents = string(firstChar) + contents[1:len(contents)-1] + string(lastChar)
		} else if startsAlt {
			if !endsAlt {
				return nil, fmt.Errorf("invalid start/end guards: %s", contents)
			}
			// Map alt chars to standard
			first := codabarMapAltChar(firstChar)
			last := codabarMapAltChar(lastChar)
			contents = string(first) + contents[1:len(contents)-1] + string(last)
		} else {
			if endsNormal || endsAlt {
				return nil, fmt.Errorf("invalid start/end guards: %s", contents)
			}
			// Doesn't end with guard either, so add a default
			contents = "A" + contents + "A"
		}
	}

	// The start character and the end character are decoded to 10 length each.
	resultLength := 20
	for i := 1; i < len(contents)-1; i++ {
		ch := contents[i]
		if unicode.IsDigit(rune(ch)) || ch == '-' || ch == '$' {
			resultLength += 9
		} else if codabarArrayContains(codabarTenLengthChars[:], ch) {
			resultLength += 10
		} else {
			return nil, fmt.Errorf("cannot encode: '%c'", ch)
		}
	}
	// A blank is placed between each character.
	resultLength += len(contents) - 1

	result := make([]bool, resultLength)
	position := 0
	for index := 0; index < len(contents); index++ {
		c := contents[index]
		if index == 0 || index == len(contents)-1 {
			// Map alt start/end chars
			c = codabarMapAltChar(c)
		}
		code := 0
		for i := 0; i < len(codabarAlphabet); i++ {
			if c == codabarAlphabet[i] {
				code = codabarCharacterEncodings[i]
				break
			}
		}
		color := true
		counter := 0
		bit := 0
		for bit < 7 { // A character consists of 7 elements
			result[position] = color
			position++
			if ((code >> (6 - bit)) & 1) == 0 || counter == 1 {
				color = !color
				bit++
				counter = 0
			} else {
				counter++
			}
		}
		if index < len(contents)-1 {
			result[position] = false
			position++
		}
	}
	return result, nil
}

// codabarMapAltChar maps alternate start/end characters to standard ones.
func codabarMapAltChar(c byte) byte {
	switch c {
	case 'T':
		return 'A'
	case 'N':
		return 'B'
	case '*':
		return 'C'
	case 'E':
		return 'D'
	default:
		return c
	}
}
