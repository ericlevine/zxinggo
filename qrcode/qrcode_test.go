package qrcode

import (
	"testing"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/qrcode/decoder"
	"github.com/ericlevine/zxinggo/qrcode/encoder"
)

func TestRoundTripNumeric(t *testing.T) {
	testRoundTrip(t, "1234567890", decoder.ECLevelM)
}

func TestRoundTripAlphanumeric(t *testing.T) {
	testRoundTrip(t, "HELLO WORLD", decoder.ECLevelL)
}

func TestRoundTripByte(t *testing.T) {
	testRoundTrip(t, "Hello, World! This is a test.", decoder.ECLevelQ)
}

func TestRoundTripHighEC(t *testing.T) {
	testRoundTrip(t, "TEST123", decoder.ECLevelH)
}

func TestRoundTripAllECLevels(t *testing.T) {
	content := "Testing all EC levels"
	levels := []decoder.ErrorCorrectionLevel{
		decoder.ECLevelL, decoder.ECLevelM, decoder.ECLevelQ, decoder.ECLevelH,
	}
	for _, ecLevel := range levels {
		t.Run(ecLevel.String(), func(t *testing.T) {
			testRoundTrip(t, content, ecLevel)
		})
	}
}

func TestWriterEncode(t *testing.T) {
	w := NewWriter()
	result, err := w.Encode("Hello", zxinggo.FormatQRCode, 100, 100, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if result.Width() == 0 || result.Height() == 0 {
		t.Fatalf("empty result matrix")
	}
	if result.Width() < 100 || result.Height() < 100 {
		t.Fatalf("result too small: %dx%d", result.Width(), result.Height())
	}
}

func TestWriterEncodeWithOptions(t *testing.T) {
	w := NewWriter()
	margin := 2
	opts := &zxinggo.EncodeOptions{
		ErrorCorrection: "H",
		Margin:          &margin,
	}
	result, err := w.Encode("Test", zxinggo.FormatQRCode, 200, 200, opts)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if result.Width() < 200 || result.Height() < 200 {
		t.Fatalf("result too small: %dx%d", result.Width(), result.Height())
	}
}

func TestWriterWrongFormat(t *testing.T) {
	w := NewWriter()
	_, err := w.Encode("Hello", zxinggo.FormatCode128, 100, 100, nil)
	if err == nil {
		t.Fatal("expected error for wrong format")
	}
}

func TestWriterEmptyContents(t *testing.T) {
	w := NewWriter()
	_, err := w.Encode("", zxinggo.FormatQRCode, 100, 100, nil)
	if err == nil {
		t.Fatal("expected error for empty contents")
	}
}

func testRoundTrip(t *testing.T, content string, ecLevel decoder.ErrorCorrectionLevel) {
	t.Helper()

	// Encode
	code, err := encoder.Encode(content, ecLevel, 0, -1)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if code.Matrix == nil {
		t.Fatal("encoded matrix is nil")
	}

	// Convert ByteMatrix to BitMatrix for decoding
	bits := code.ToBitMatrix()

	// Decode
	dec := decoder.NewDecoder()
	result, err := dec.Decode(bits, "")
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if result.Text != content {
		t.Errorf("round-trip mismatch: got %q, want %q", result.Text, content)
	}
}
