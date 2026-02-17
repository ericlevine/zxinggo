package oned

import (
	"fmt"

	"github.com/ericlevine/zxinggo/bitutil"
)

// OneDEncoder encodes contents into a boolean pattern for a 1D barcode.
type OneDEncoder interface {
	// Encode encodes the contents into a boolean array representing bars.
	Encode(contents string) ([]bool, error)
}

const defaultOneDMargin = 10 // quiet zone in modules

// RenderOneDCode renders a 1D barcode pattern as a BitMatrix with quiet zones.
func RenderOneDCode(code []bool, width, height int) *bitutil.BitMatrix {
	inputWidth := len(code)
	fullWidth := inputWidth + 2*defaultOneDMargin
	if width < fullWidth {
		width = fullWidth
	}
	if height < 1 {
		height = 1
	}

	outputWidth := width
	outputHeight := height

	multiple := outputWidth / fullWidth
	if multiple < 1 {
		multiple = 1
	}
	leftPadding := (outputWidth - (inputWidth * multiple)) / 2

	output := bitutil.NewBitMatrixWithSize(outputWidth, outputHeight)
	for inputX := 0; inputX < inputWidth; inputX++ {
		if code[inputX] {
			outputX := leftPadding + inputX*multiple
			for x := outputX; x < outputX+multiple && x < outputWidth; x++ {
				for y := 0; y < outputHeight; y++ {
					output.Set(x, y)
				}
			}
		}
	}
	return output
}

// AppendPattern appends a pattern of bars/spaces to a boolean array.
// If startColor is true, the first element is a bar (black); otherwise space (white).
// Returns the total width appended.
func AppendPattern(target []bool, pos int, pattern []int, startColor bool) int {
	color := startColor
	numAdded := 0
	for _, p := range pattern {
		for j := 0; j < p; j++ {
			target[pos] = color
			pos++
			numAdded++
		}
		color = !color
	}
	return numAdded
}

// CheckNumeric validates that a string contains only digits.
func CheckNumeric(s string) error {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return fmt.Errorf("contents contain non-digit character: %c", s[i])
		}
	}
	return nil
}
