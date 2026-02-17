package main

import (
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/binarizer"

	// Register all format readers.
	_ "github.com/ericlevine/zxinggo/oned"
	_ "github.com/ericlevine/zxinggo/pdf417"
	_ "github.com/ericlevine/zxinggo/qrcode"
)

func main() {
	tryHarder := flag.Bool("try-harder", false, "spend more time looking for barcodes")
	pure := flag.Bool("pure", false, "hint that the image is a clean barcode render with minimal border")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: barcodescan [flags] <image-file> [image-file...]\n\n")
		fmt.Fprintf(os.Stderr, "Detect and decode barcodes in image files (PNG, JPEG, GIF).\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	exitCode := 0
	for _, path := range flag.Args() {
		results, err := scanFile(path, *tryHarder, *pure)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: error: %v\n", path, err)
			exitCode = 1
			continue
		}
		if len(results) == 0 {
			fmt.Fprintf(os.Stderr, "%s: no barcodes found\n", path)
			exitCode = 1
			continue
		}
		for _, r := range results {
			if flag.NArg() > 1 {
				fmt.Printf("%s: ", path)
			}
			fmt.Printf("[%s] %s\n", r.Format, r.Text)
		}
	}
	os.Exit(exitCode)
}

// allFormats lists every format to attempt.
var allFormats = []zxinggo.Format{
	zxinggo.FormatQRCode,
	zxinggo.FormatPDF417,
	zxinggo.FormatCode128,
	zxinggo.FormatCode39,
	zxinggo.FormatEAN13,
	zxinggo.FormatEAN8,
	zxinggo.FormatUPCA,
	zxinggo.FormatUPCE,
}

func scanFile(path string, tryHarder, pure bool) ([]*zxinggo.Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	source := zxinggo.NewImageLuminanceSource(img)
	opts := &zxinggo.DecodeOptions{
		TryHarder:   tryHarder,
		PureBarcode: pure,
	}

	// Try GlobalHistogram binarizer first (fast, works well for clean images),
	// then fall back to Hybrid binarizer (local adaptive thresholding, better
	// for photographs with uneven lighting). This mirrors the Java ZXing
	// MultiFormatReader retry strategy.
	bitmaps := []*zxinggo.BinaryBitmap{
		zxinggo.NewBinaryBitmap(binarizer.NewGlobalHistogram(source)),
		zxinggo.NewBinaryBitmap(binarizer.NewHybrid(source)),
	}

	var results []*zxinggo.Result
	seen := map[string]bool{}

	for _, bitmap := range bitmaps {
		for _, format := range allFormats {
			formatOpts := *opts
			formatOpts.PossibleFormats = []zxinggo.Format{format}

			result, err := tryDecode(bitmap, &formatOpts)
			if err != nil {
				continue
			}
			key := fmt.Sprintf("%s:%s", result.Format, result.Text)
			if seen[key] {
				continue
			}
			seen[key] = true
			results = append(results, result)
		}
	}

	return results, nil
}

// tryDecode calls zxinggo.Decode but recovers from panics that decoders may
// raise on malformed input, converting them to errors.
func tryDecode(bitmap *zxinggo.BinaryBitmap, opts *zxinggo.DecodeOptions) (result *zxinggo.Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			result = nil
			err = fmt.Errorf("decoder panic: %v", r)
		}
	}()
	return zxinggo.Decode(bitmap, opts)
}
