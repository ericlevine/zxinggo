package pdf417

import (
	"fmt"
	"math"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
	"github.com/ericlevine/zxinggo/pdf417/encoder"
)

const (
	defaultWhiteSpace            = 30
	defaultErrorCorrectionLevel  = 2
)

// PDF417Writer encodes PDF417 barcodes.
type PDF417Writer struct{}

// NewPDF417Writer creates a new PDF417 writer.
func NewPDF417Writer() *PDF417Writer {
	return &PDF417Writer{}
}

// Encode encodes the given contents into a PDF417 barcode BitMatrix.
func (w *PDF417Writer) Encode(contents string, format zxinggo.Format, width, height int, opts *zxinggo.EncodeOptions) (*bitutil.BitMatrix, error) {
	if format != zxinggo.FormatPDF417 {
		return nil, fmt.Errorf("can only encode PDF_417, but got %s", format)
	}

	enc := encoder.NewPDF417Encoder()
	margin := defaultWhiteSpace
	errorCorrectionLevel := defaultErrorCorrectionLevel

	if opts != nil {
		if opts.PDF417Compact {
			enc.SetCompact(true)
		}
		if opts.PDF417Compaction > 0 {
			enc.SetCompaction(encoder.Compaction(opts.PDF417Compaction))
		}
		if opts.PDF417Dimensions != nil {
			enc.SetDimensions(
				opts.PDF417Dimensions.MaxCols,
				opts.PDF417Dimensions.MinCols,
				opts.PDF417Dimensions.MaxRows,
				opts.PDF417Dimensions.MinRows,
			)
		}
		if opts.Margin != nil {
			margin = *opts.Margin
		}
		if opts.ErrorCorrection != "" {
			var ecl int
			if _, err := fmt.Sscanf(opts.ErrorCorrection, "%d", &ecl); err == nil {
				errorCorrectionLevel = ecl
			}
		}
	}

	if err := enc.GenerateBarcodeLogic(contents, errorCorrectionLevel); err != nil {
		return nil, err
	}

	aspectRatio := 4
	originalScale := enc.BarcodeMatrix().ScaledMatrix(1, aspectRatio)
	rotated := false
	if (height > width) != (len(originalScale[0]) < len(originalScale)) {
		originalScale = rotateArray(originalScale)
		rotated = true
	}

	scaleX := width / len(originalScale[0])
	scaleY := height / len(originalScale)
	scale := int(math.Min(float64(scaleX), float64(scaleY)))

	if scale > 1 {
		scaledMatrix := enc.BarcodeMatrix().ScaledMatrix(scale, scale*aspectRatio)
		if rotated {
			scaledMatrix = rotateArray(scaledMatrix)
		}
		return bitMatrixFromByteArray(scaledMatrix, margin), nil
	}
	return bitMatrixFromByteArray(originalScale, margin), nil
}

func bitMatrixFromByteArray(input [][]byte, margin int) *bitutil.BitMatrix {
	outputWidth := len(input[0]) + 2*margin
	outputHeight := len(input) + 2*margin
	output := bitutil.NewBitMatrixWithSize(outputWidth, outputHeight)

	for y := 0; y < len(input); y++ {
		yOutput := outputHeight - margin - 1 - y
		for x := 0; x < len(input[0]); x++ {
			if input[y][x] == 1 {
				output.Set(x+margin, yOutput)
			}
		}
	}
	return output
}

func rotateArray(bitarray [][]byte) [][]byte {
	rows := len(bitarray)
	cols := len(bitarray[0])
	temp := make([][]byte, cols)
	for i := range temp {
		temp[i] = make([]byte, rows)
	}
	for ii := 0; ii < rows; ii++ {
		inverseii := rows - ii - 1
		for jj := 0; jj < cols; jj++ {
			temp[jj][inverseii] = bitarray[ii][jj]
		}
	}
	return temp
}
