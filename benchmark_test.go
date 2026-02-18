package zxinggo_test

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"testing"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/binarizer"

	_ "github.com/ericlevine/zxinggo/aztec"
	_ "github.com/ericlevine/zxinggo/datamatrix"
	_ "github.com/ericlevine/zxinggo/oned"
	_ "github.com/ericlevine/zxinggo/pdf417"
	_ "github.com/ericlevine/zxinggo/qrcode"
)

func loadTestImage(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		panic("failed to open image: " + err.Error())
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		panic("failed to decode image: " + err.Error())
	}
	return img
}

var decodeTests = []struct {
	name   string
	path   string
	format zxinggo.Format
}{
	{"QRCode", "testdata/blackbox/qrcode-1/1.png", zxinggo.FormatQRCode},
	{"DataMatrix", "testdata/blackbox/datamatrix-1/0123456789.png", zxinggo.FormatDataMatrix},
	{"PDF417", "testdata/blackbox/pdf417-1/01.png", zxinggo.FormatPDF417},
	{"Aztec", "testdata/blackbox/aztec-1/abc-37x37.png", zxinggo.FormatAztec},
	{"Code128", "testdata/blackbox/code128-1/1.png", zxinggo.FormatCode128},
	{"EAN13", "testdata/blackbox/ean13-1/1.png", zxinggo.FormatEAN13},
}

var encodeTests = []struct {
	name    string
	content string
	format  zxinggo.Format
	width   int
	height  int
}{
	{"QRCode", "Hello, World! This is a QR code benchmark test.", zxinggo.FormatQRCode, 400, 400},
	{"DataMatrix", "Hello DataMatrix", zxinggo.FormatDataMatrix, 0, 0},
	{"PDF417", "Hello PDF417 Benchmark Test Data", zxinggo.FormatPDF417, 0, 0},
	{"Aztec", "Hello Aztec Code", zxinggo.FormatAztec, 0, 0},
	{"Code128", "Hello123", zxinggo.FormatCode128, 300, 100},
	{"EAN13", "5901234123457", zxinggo.FormatEAN13, 300, 100},
}

func BenchmarkDecode(b *testing.B) {
	for _, tc := range decodeTests {
		b.Run(tc.name, func(b *testing.B) {
			img := loadTestImage(tc.path)
			opts := &zxinggo.DecodeOptions{
				PossibleFormats: []zxinggo.Format{tc.format},
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Create fresh binarizer/bitmap each iteration since HybridBinarizer caches
				source := zxinggo.NewImageLuminanceSource(img)
				bitmap := zxinggo.NewBinaryBitmap(binarizer.NewHybrid(source))
				_, err := zxinggo.Decode(bitmap, opts)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkEncode(b *testing.B) {
	for _, tc := range encodeTests {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := zxinggo.Encode(tc.content, tc.format, tc.width, tc.height, nil)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
