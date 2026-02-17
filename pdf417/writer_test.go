package pdf417

import (
	"testing"

	zxinggo "github.com/ericlevine/zxinggo"
)

func TestPDF417WriterBasic(t *testing.T) {
	writer := NewPDF417Writer()
	matrix, err := writer.Encode("Hello, World!", zxinggo.FormatPDF417, 400, 200, nil)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	if matrix.Width() == 0 || matrix.Height() == 0 {
		t.Fatal("expected non-empty matrix")
	}
	t.Logf("matrix size: %dx%d", matrix.Width(), matrix.Height())
}

func TestPDF417WriterNumeric(t *testing.T) {
	writer := NewPDF417Writer()
	matrix, err := writer.Encode("1234567890123456", zxinggo.FormatPDF417, 400, 200, nil)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	if matrix.Width() == 0 || matrix.Height() == 0 {
		t.Fatal("expected non-empty matrix")
	}
}

func TestPDF417WriterWrongFormat(t *testing.T) {
	writer := NewPDF417Writer()
	_, err := writer.Encode("test", zxinggo.FormatQRCode, 400, 200, nil)
	if err == nil {
		t.Error("expected error for wrong format")
	}
}

func TestPDF417WriterWithOptions(t *testing.T) {
	writer := NewPDF417Writer()
	margin := 10
	opts := &zxinggo.EncodeOptions{
		Margin:          &margin,
		ErrorCorrection: "4",
	}
	matrix, err := writer.Encode("Test with options", zxinggo.FormatPDF417, 400, 200, opts)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	if matrix.Width() == 0 || matrix.Height() == 0 {
		t.Fatal("expected non-empty matrix")
	}
}
