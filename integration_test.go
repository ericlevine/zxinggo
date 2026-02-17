package zxinggo_test

import (
	"testing"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/binarizer"

	// Import format packages to trigger init() registration.
	_ "github.com/ericlevine/zxinggo/oned"
	_ "github.com/ericlevine/zxinggo/pdf417"
	_ "github.com/ericlevine/zxinggo/qrcode"
)

func encodeAndDecode(t *testing.T, content string, format zxinggo.Format, width, height int) string {
	t.Helper()

	// Encode
	matrix, err := zxinggo.Encode(content, format, width, height, nil)
	if err != nil {
		t.Fatalf("Encode(%s, %s) failed: %v", content, format, err)
	}
	if matrix.Width() == 0 || matrix.Height() == 0 {
		t.Fatalf("encoded matrix is empty")
	}

	// Convert to image
	img := zxinggo.BitMatrixToImage(matrix)

	// Create binary bitmap via binarizer pipeline
	source := zxinggo.NewGrayImageLuminanceSource(img)
	bin := binarizer.NewGlobalHistogram(source)
	bitmap := zxinggo.NewBinaryBitmap(bin)

	// Decode - use PureBarcode since we're decoding from a clean render
	opts := &zxinggo.DecodeOptions{
		PossibleFormats: []zxinggo.Format{format},
		PureBarcode:     true,
	}
	result, err := zxinggo.Decode(bitmap, opts)
	if err != nil {
		t.Fatalf("Decode(%s) failed: %v", format, err)
	}

	return result.Text
}

func TestRoundTripQRCode(t *testing.T) {
	content := "Hello, World!"
	decoded := encodeAndDecode(t, content, zxinggo.FormatQRCode, 400, 400)
	if decoded != content {
		t.Errorf("QR round-trip: got %q, want %q", decoded, content)
	}
}

func TestRoundTripQRCodeNumeric(t *testing.T) {
	content := "1234567890"
	decoded := encodeAndDecode(t, content, zxinggo.FormatQRCode, 200, 200)
	if decoded != content {
		t.Errorf("QR numeric round-trip: got %q, want %q", decoded, content)
	}
}

func TestRoundTripCode128(t *testing.T) {
	content := "Hello123"
	decoded := encodeAndDecode(t, content, zxinggo.FormatCode128, 300, 100)
	if decoded != content {
		t.Errorf("Code128 round-trip: got %q, want %q", decoded, content)
	}
}

func TestRoundTripCode39(t *testing.T) {
	content := "HELLO"
	decoded := encodeAndDecode(t, content, zxinggo.FormatCode39, 300, 100)
	if decoded != content {
		t.Errorf("Code39 round-trip: got %q, want %q", decoded, content)
	}
}

func TestRoundTripEAN13(t *testing.T) {
	content := "5901234123457"
	decoded := encodeAndDecode(t, content, zxinggo.FormatEAN13, 500, 100)
	if decoded != content {
		t.Errorf("EAN-13 round-trip: got %q, want %q", decoded, content)
	}
}

func TestRoundTripEAN8(t *testing.T) {
	content := "96385074"
	decoded := encodeAndDecode(t, content, zxinggo.FormatEAN8, 300, 100)
	if decoded != content {
		t.Errorf("EAN-8 round-trip: got %q, want %q", decoded, content)
	}
}

func TestRoundTripUPCA(t *testing.T) {
	content := "012345678905"
	// UPC-A is encoded as EAN-13 with leading 0, so the decoder returns the
	// full 13-digit EAN-13 string "0012345678905".
	decoded := encodeAndDecode(t, content, zxinggo.FormatUPCA, 500, 100)
	expected := "0" + content // "0012345678905"
	if decoded != expected {
		t.Errorf("UPC-A round-trip: got %q, want %q", decoded, expected)
	}
}

func TestRoundTripUPCE(t *testing.T) {
	content := "01234565"
	decoded := encodeAndDecode(t, content, zxinggo.FormatUPCE, 400, 100)
	if decoded != content {
		t.Errorf("UPC-E round-trip: got %q, want %q", decoded, content)
	}
}

func TestEncodeTopLevelAPI(t *testing.T) {
	// Test that the top-level Encode works for all writable formats
	formats := []struct {
		format  zxinggo.Format
		content string
		width   int
		height  int
	}{
		{zxinggo.FormatQRCode, "Test", 200, 200},
		{zxinggo.FormatPDF417, "Test", 400, 200},
		{zxinggo.FormatCode128, "Test", 300, 100},
		{zxinggo.FormatCode39, "TEST", 300, 100},
		{zxinggo.FormatEAN13, "5901234123457", 300, 100},
		{zxinggo.FormatEAN8, "96385074", 300, 100},
		{zxinggo.FormatUPCA, "012345678905", 300, 100},
		{zxinggo.FormatUPCE, "01234565", 300, 100},
	}
	for _, tc := range formats {
		t.Run(tc.format.String(), func(t *testing.T) {
			matrix, err := zxinggo.Encode(tc.content, tc.format, tc.width, tc.height, nil)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}
			if matrix.Width() == 0 || matrix.Height() == 0 {
				t.Fatal("empty result")
			}
		})
	}
}

func TestImageLuminanceSource(t *testing.T) {
	// Encode a QR code, convert to image, verify luminance source properties
	matrix, err := zxinggo.Encode("test", zxinggo.FormatQRCode, 100, 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	img := zxinggo.BitMatrixToImage(matrix)
	source := zxinggo.NewGrayImageLuminanceSource(img)

	if source.Width() != img.Bounds().Dx() {
		t.Errorf("width: got %d, want %d", source.Width(), img.Bounds().Dx())
	}
	if source.Height() != img.Bounds().Dy() {
		t.Errorf("height: got %d, want %d", source.Height(), img.Bounds().Dy())
	}

	lum := source.Matrix()
	if len(lum) != source.Width()*source.Height() {
		t.Errorf("matrix length: got %d, want %d", len(lum), source.Width()*source.Height())
	}

	row := source.Row(0, nil)
	if len(row) != source.Width() {
		t.Errorf("row length: got %d, want %d", len(row), source.Width())
	}
}
