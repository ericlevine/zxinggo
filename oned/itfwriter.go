package oned

import (
	"fmt"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// ITFWriter encodes ITF (Interleaved 2 of 5) barcodes.
type ITFWriter struct{}

// NewITFWriter creates a new ITF writer.
func NewITFWriter() *ITFWriter {
	return &ITFWriter{}
}

// Encode encodes the given contents into an ITF barcode BitMatrix.
func (w *ITFWriter) Encode(contents string, format zxinggo.Format, width, height int, opts *zxinggo.EncodeOptions) (*bitutil.BitMatrix, error) {
	if format != zxinggo.FormatITF {
		return nil, fmt.Errorf("can only encode ITF, but got %s", format)
	}
	if err := CheckNumeric(contents); err != nil {
		return nil, err
	}
	if len(contents)%2 != 0 {
		return nil, fmt.Errorf("ITF requires an even number of digits, got %d", len(contents))
	}
	code, err := w.encode(contents)
	if err != nil {
		return nil, err
	}
	return RenderOneDCode(code, width, height), nil
}

func (w *ITFWriter) encode(contents string) ([]bool, error) {
	length := len(contents)
	// Each digit pair encodes to 5+5 = 10 bars/spaces with width 1 or 3.
	// Start pattern: 1+1+1+1 = 4. End pattern: 3+1+1 = 5 (wide-narrow-narrow).
	// A digit pair has total width: sum of (narrow/wide) for bars + sum for spaces.
	// Each pair: 5 narrow/wide bars interleaved with 5 narrow/wide spaces.
	// Width = sum of widths for both digits.
	digitPairWidth := 0
	for _, w := range itfPatterns[0] {
		digitPairWidth += w
	}
	digitPairWidth *= 2 // bars + spaces (worst case for sizing doesn't matter, we compute exactly)

	// Actually compute total width precisely:
	// Start: 4 (1+1+1+1), End: 5 (3+1+1)
	// Each pair: for each of 5 positions, bar width from digit1 + space width from digit2
	totalWidth := 4 + 5 // start + end
	for i := 0; i < length; i += 2 {
		d1 := contents[i] - '0'
		d2 := contents[i+1] - '0'
		for j := 0; j < 5; j++ {
			totalWidth += itfPatterns[d1][j] + itfPatterns[d2][j]
		}
	}

	result := make([]bool, totalWidth)
	pos := 0

	// Start pattern: narrow bar, narrow space, narrow bar, narrow space
	startPattern := []int{1, 1, 1, 1}
	pos += AppendPattern(result, pos, startPattern, true)

	for i := 0; i < length; i += 2 {
		d1 := contents[i] - '0'
		d2 := contents[i+1] - '0'
		encoding := make([]int, 10)
		for j := 0; j < 5; j++ {
			encoding[2*j] = itfPatterns[d1][j]
			encoding[2*j+1] = itfPatterns[d2][j]
		}
		pos += AppendPattern(result, pos, encoding, true)
	}

	// End pattern: wide bar, narrow space, narrow bar
	endPattern := []int{3, 1, 1}
	AppendPattern(result, pos, endPattern, true)

	return result, nil
}
