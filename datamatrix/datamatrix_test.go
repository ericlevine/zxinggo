package datamatrix

import (
	"testing"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/binarizer"
	"github.com/ericlevine/zxinggo/bitutil"
)

func TestDataMatrixRoundTrip(t *testing.T) {
	tests := []string{
		"Hello",
		"Test123",
		"1234567890",
		"ABCDEF",
		"Hello, World!",
	}

	writer := NewWriter()
	reader := NewReader()

	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			matrix, err := writer.Encode(tc, zxinggo.FormatDataMatrix, 0, 0, nil)
			if err != nil {
				t.Fatalf("encode error: %v", err)
			}

			source := newBitMatrixLuminanceSource(matrix)
			bitmap := zxinggo.NewBinaryBitmap(binarizer.NewGlobalHistogram(source))

			opts := &zxinggo.DecodeOptions{PureBarcode: true}
			result, err := reader.Decode(bitmap, opts)
			if err != nil {
				t.Fatalf("decode error for %q: %v", tc, err)
			}
			if result.Text != tc {
				t.Errorf("round-trip mismatch: got %q, want %q", result.Text, tc)
			}
			if result.Format != zxinggo.FormatDataMatrix {
				t.Errorf("format mismatch: got %v, want %v", result.Format, zxinggo.FormatDataMatrix)
			}
		})
	}
}

func TestDataMatrixWriterFormatValidation(t *testing.T) {
	_, err := NewWriter().Encode("TEST", zxinggo.FormatQRCode, 200, 200, nil)
	if err == nil {
		t.Error("expected error for wrong format on DataMatrixWriter")
	}
}

// bitMatrixLuminanceSource wraps a BitMatrix as a LuminanceSource for testing.
type bitMatrixLuminanceSource struct {
	matrix *bitutil.BitMatrix
}

func newBitMatrixLuminanceSource(m *bitutil.BitMatrix) *bitMatrixLuminanceSource {
	return &bitMatrixLuminanceSource{matrix: m}
}

func (s *bitMatrixLuminanceSource) Width() int  { return s.matrix.Width() }
func (s *bitMatrixLuminanceSource) Height() int { return s.matrix.Height() }

func (s *bitMatrixLuminanceSource) Row(y int, row []byte) []byte {
	w := s.matrix.Width()
	if len(row) < w {
		row = make([]byte, w)
	}
	for x := 0; x < w; x++ {
		if s.matrix.Get(x, y) {
			row[x] = 0 // black
		} else {
			row[x] = 255 // white
		}
	}
	return row
}

func (s *bitMatrixLuminanceSource) Matrix() []byte {
	w := s.matrix.Width()
	h := s.matrix.Height()
	result := make([]byte, w*h)
	for y := 0; y < h; y++ {
		offset := y * w
		for x := 0; x < w; x++ {
			if s.matrix.Get(x, y) {
				result[offset+x] = 0
			} else {
				result[offset+x] = 255
			}
		}
	}
	return result
}
