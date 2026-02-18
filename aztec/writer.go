package aztec

import (
	"fmt"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/aztec/encoder"
	"github.com/ericlevine/zxinggo/bitutil"
)

// Writer encodes Aztec barcodes.
type Writer struct{}

// NewWriter creates a new Aztec Writer.
func NewWriter() *Writer {
	return &Writer{}
}

// Encode encodes the given contents into an Aztec BitMatrix.
func (w *Writer) Encode(contents string, format zxinggo.Format, width, height int, opts *zxinggo.EncodeOptions) (*bitutil.BitMatrix, error) {
	if contents == "" {
		return nil, fmt.Errorf("found empty contents")
	}
	if format != zxinggo.FormatAztec {
		return nil, fmt.Errorf("can only encode AZTEC, but got %s", format)
	}

	minECCPercent := 33
	code, err := encoder.Encode([]byte(contents), minECCPercent, 0)
	if err != nil {
		return nil, err
	}

	return renderMatrix(code.Matrix, width, height), nil
}

// renderMatrix scales the encoded Aztec symbol to fit the requested
// width and height, preserving the module aspect ratio.
func renderMatrix(code *bitutil.BitMatrix, width, height int) *bitutil.BitMatrix {
	inputWidth := code.Width()
	inputHeight := code.Height()

	// Add a 1-module quiet zone on each side.
	qz := 1
	outputWidth := inputWidth + 2*qz
	outputHeight := inputHeight + 2*qz

	if width < outputWidth {
		width = outputWidth
	}
	if height < outputHeight {
		height = outputHeight
	}

	multiple := width / outputWidth
	if h := height / outputHeight; h < multiple {
		multiple = h
	}
	if multiple < 1 {
		multiple = 1
	}

	leftPadding := (width - inputWidth*multiple) / 2
	topPadding := (height - inputHeight*multiple) / 2

	result := bitutil.NewBitMatrixWithSize(width, height)
	for inputY := 0; inputY < inputHeight; inputY++ {
		outputY := topPadding + inputY*multiple
		for inputX := 0; inputX < inputWidth; inputX++ {
			if code.Get(inputX, inputY) {
				outputX := leftPadding + inputX*multiple
				for y := 0; y < multiple; y++ {
					for x := 0; x < multiple; x++ {
						result.Set(outputX+x, outputY+y)
					}
				}
			}
		}
	}
	return result
}

// Compile-time check.
var _ zxinggo.Writer = (*Writer)(nil)
