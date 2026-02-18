# zxinggo

A pure Go port of the [ZXing](https://github.com/zxing/zxing) ("Zebra Crossing") barcode reading and writing library.

## Supported Formats

| Format | Read | Write |
|--------|------|-------|
| QR Code | Yes | Yes |
| PDF417 | Yes | Yes |
| Data Matrix | Yes | Yes |
| Aztec | Yes | Yes |
| Code 128 | Yes | Yes |
| Code 39 | Yes | Yes |
| EAN-13 | Yes | Yes |
| EAN-8 | Yes | Yes |
| UPC-A | Yes | Yes |
| UPC-E | Yes | Yes |
| ITF | Yes | Yes |
| Codabar | Yes | Yes |
| RSS-14 (GS1 DataBar) | Yes | - |
| RSS Expanded | Yes | - |
| MaxiCode | Yes | - |

## Installation

```
go get github.com/ericlevine/zxinggo
```

## Usage

### Decoding a barcode from an image

```go
package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/binarizer"

	// Register the formats you want to decode.
	_ "github.com/ericlevine/zxinggo/qrcode"
	_ "github.com/ericlevine/zxinggo/oned"
)

func main() {
	f, _ := os.Open("barcode.png")
	defer f.Close()
	img, _, _ := image.Decode(f)

	source := zxinggo.NewImageLuminanceSource(img)
	bitmap := zxinggo.NewBinaryBitmap(binarizer.NewHybrid(source))

	result, err := zxinggo.Decode(bitmap, nil)
	if err != nil {
		fmt.Println("No barcode found:", err)
		return
	}
	fmt.Printf("[%s] %s\n", result.Format, result.Text)
}
```

### Encoding a barcode

```go
package main

import (
	"fmt"
	"image"
	"image/png"
	"os"

	zxinggo "github.com/ericlevine/zxinggo"

	_ "github.com/ericlevine/zxinggo/qrcode"
)

func main() {
	matrix, err := zxinggo.Encode("Hello, World!", zxinggo.FormatQRCode, 256, 256, nil)
	if err != nil {
		fmt.Println("Encode error:", err)
		return
	}

	// Convert BitMatrix to image
	w, h := matrix.Width(), matrix.Height()
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if matrix.Get(x, y) {
				img.Pix[y*img.Stride+x] = 0 // black
			} else {
				img.Pix[y*img.Stride+x] = 255 // white
			}
		}
	}

	f, _ := os.Create("qrcode.png")
	defer f.Close()
	png.Encode(f, img)
}
```

### Decoding with options

```go
opts := &zxinggo.DecodeOptions{
	TryHarder:       true,
	PossibleFormats: []zxinggo.Format{zxinggo.FormatQRCode, zxinggo.FormatEAN13},
}
result, err := zxinggo.Decode(bitmap, opts)
```

## CLI Tool

The `barcodescan` command-line tool decodes barcodes from image files:

```
go install github.com/ericlevine/zxinggo/cmd/barcodescan@latest

barcodescan photo.jpg
# [QR_CODE] https://example.com

barcodescan --try-harder receipt.png
# [EAN_13] 4006381333931
```

## Architecture

Format packages register themselves via `init()` using blank imports. Only import the formats you need:

```go
import (
	_ "github.com/ericlevine/zxinggo/qrcode"     // QR Code
	_ "github.com/ericlevine/zxinggo/datamatrix"  // Data Matrix
	_ "github.com/ericlevine/zxinggo/aztec"       // Aztec
	_ "github.com/ericlevine/zxinggo/pdf417"      // PDF417
	_ "github.com/ericlevine/zxinggo/oned"        // All 1D formats
	_ "github.com/ericlevine/zxinggo/maxicode"    // MaxiCode
)
```

## Testing

The test suite includes the full ZXing blackbox image test corpus (1,200+ test images across all formats):

```
go test ./...
```

## License

Apache License 2.0 - same as the original [ZXing project](https://github.com/zxing/zxing).

This project is a derivative work of ZXing, Copyright 2007 ZXing authors.
