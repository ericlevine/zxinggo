package oned

import (
	"fmt"
	"strings"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// CodabarWriter encodes Codabar barcodes.
type CodabarWriter struct{}

// NewCodabarWriter creates a new Codabar writer.
func NewCodabarWriter() *CodabarWriter {
	return &CodabarWriter{}
}

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
		// Codabar requires at least start + stop; if not present, wrap with A/B
		contents = "A" + contents + "B"
	} else {
		upper := strings.ToUpper(contents)
		first := upper[0]
		last := upper[len(upper)-1]
		if !isCodabarStartEnd(first) || !isCodabarStartEnd(last) {
			contents = "A" + contents + "B"
		} else {
			contents = string(first) + contents[1:len(contents)-1] + string(last)
		}
	}

	// Validate all characters
	for i := 1; i < len(contents)-1; i++ {
		if strings.IndexByte(codabarAlphabet, contents[i]) < 0 {
			return nil, fmt.Errorf("invalid character in Codabar contents: %c", contents[i])
		}
	}

	// Compute total width: each character is 7 elements + 1 inter-char gap, minus
	// the last gap.
	totalWidth := 0
	for i := 0; i < len(contents); i++ {
		idx := strings.IndexByte(codabarAlphabet, contents[i])
		if idx < 0 {
			return nil, fmt.Errorf("invalid character in Codabar contents: %c", contents[i])
		}
		for _, w := range codabarCharacterEncodings[idx] {
			totalWidth += w
		}
	}
	totalWidth += len(contents) - 1 // inter-character gaps

	result := make([]bool, totalWidth)
	pos := 0
	for i := 0; i < len(contents); i++ {
		idx := strings.IndexByte(codabarAlphabet, contents[i])
		pos += AppendPattern(result, pos, codabarCharacterEncodings[idx][:], true)
		if i < len(contents)-1 {
			// inter-character gap (1 module white space)
			pos++ // already false (white)
		}
	}

	return result, nil
}
