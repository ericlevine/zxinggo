package oned

import (
	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// MultiFormatOneDReader attempts to decode 1D barcodes by trying multiple
// format-specific readers in sequence.
type MultiFormatOneDReader struct {
	readers          []RowDecoder
	possibleFormats  map[zxinggo.Format]bool
}

// NewMultiFormatOneDReader creates a new multi-format reader configured by opts.
func NewMultiFormatOneDReader(opts *zxinggo.DecodeOptions) *MultiFormatOneDReader {
	var readers []RowDecoder
	var possibleFormats map[zxinggo.Format]bool

	if opts != nil && len(opts.PossibleFormats) > 0 {
		possibleFormats = make(map[zxinggo.Format]bool)
		for _, f := range opts.PossibleFormats {
			possibleFormats[f] = true
		}
		// UPC/EAN readers: match Java's MultiFormatUPCEANReader else-if logic.
		// EAN-13 covers UPC-A, so only add UPCAReader if EAN-13 is not requested.
		if possibleFormats[zxinggo.FormatEAN13] {
			readers = append(readers, NewEAN13Reader())
		} else if possibleFormats[zxinggo.FormatUPCA] {
			readers = append(readers, NewUPCAReader())
		}
		if possibleFormats[zxinggo.FormatEAN8] {
			readers = append(readers, NewEAN8Reader())
		}
		if possibleFormats[zxinggo.FormatUPCE] {
			readers = append(readers, NewUPCEReader())
		}
		if possibleFormats[zxinggo.FormatCode39] {
			useCheckDigit := opts.AssumeCode39CheckDigit
			readers = append(readers, NewCode39ReaderWithCheckDigit(useCheckDigit, false))
		}
		if possibleFormats[zxinggo.FormatCode128] {
			readers = append(readers, NewCode128Reader())
		}
		if possibleFormats[zxinggo.FormatITF] {
			readers = append(readers, NewITFReader())
		}
		if possibleFormats[zxinggo.FormatCodabar] {
			readers = append(readers, NewCodabarReader())
		}
		if possibleFormats[zxinggo.FormatRSS14] {
			readers = append(readers, NewRSS14Reader())
		}
		if possibleFormats[zxinggo.FormatRSSExpanded] {
			readers = append(readers, NewRSSExpandedReader())
		}
	}

	if len(readers) == 0 {
		// Default: EAN-13 covers UPC-A, so no separate UPCAReader needed.
		readers = []RowDecoder{
			NewEAN13Reader(),
			NewEAN8Reader(),
			NewUPCEReader(),
			NewCode39Reader(),
			NewCode128Reader(),
			NewITFReader(),
			NewCodabarReader(),
			NewRSS14Reader(),
			NewRSSExpandedReader(),
		}
	}

	return &MultiFormatOneDReader{readers: readers, possibleFormats: possibleFormats}
}

// DecodeRow tries each reader in sequence until one succeeds.
// Includes Java-compatible EAN-13 → UPC-A conversion when UPC-A was requested.
func (r *MultiFormatOneDReader) DecodeRow(rowNumber int, row *bitutil.BitArray, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	for _, reader := range r.readers {
		result, err := reader.DecodeRow(rowNumber, row, opts)
		if err == nil {
			return r.maybeConvertEAN13ToUPCA(result), nil
		}
	}
	return nil, zxinggo.ErrNotFound
}

// maybeConvertEAN13ToUPCA converts an EAN-13 result starting with '0' to UPC-A
// if UPC-A was requested. Matches Java MultiFormatUPCEANReader behavior.
func (r *MultiFormatOneDReader) maybeConvertEAN13ToUPCA(result *zxinggo.Result) *zxinggo.Result {
	if result.Format != zxinggo.FormatEAN13 || len(result.Text) == 0 || result.Text[0] != '0' {
		return result
	}
	// Convert if UPC-A was requested, or if no format filter was set (default readers)
	if r.possibleFormats == nil || r.possibleFormats[zxinggo.FormatUPCA] {
		upcaResult := zxinggo.NewResult(result.Text[1:], nil, result.Points, zxinggo.FormatUPCA)
		for k, v := range result.Metadata {
			upcaResult.PutMetadata(k, v)
		}
		return upcaResult
	}
	return result
}

// Decode decodes a 1D barcode from the given image.
// Like Java's OneDReader.decode(), if TryHarder is set and the initial scan
// fails, it tries again with the image rotated 90 degrees counterclockwise.
func (r *MultiFormatOneDReader) Decode(image *zxinggo.BinaryBitmap, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	result, err := DecodeOneD(image, r, opts)
	if err == nil {
		return result, nil
	}
	tryHarder := opts != nil && opts.TryHarder
	if !tryHarder {
		return nil, err
	}
	// Try with rotated image (90 degrees CCW)
	rotated := image.RotateCounterClockwise()
	if rotated == nil {
		return nil, err
	}
	result, err2 := DecodeOneD(rotated, r, opts)
	if err2 != nil {
		return nil, err
	}
	// Record that we found it rotated 90 degrees CCW / 270 degrees CW
	orientation := 270
	if existing, ok := result.Metadata[zxinggo.MetadataOrientation]; ok {
		if existingInt, ok := existing.(int); ok {
			orientation = (orientation + existingInt) % 360
		}
	}
	result.PutMetadata(zxinggo.MetadataOrientation, orientation)
	// Adjust result points: for a CCW rotation, (x,y) in rotated image
	// maps to (rotatedHeight - 1 - y, x) in the original image
	if result.Points != nil {
		rotatedHeight := rotated.Height()
		for i, p := range result.Points {
			result.Points[i] = zxinggo.ResultPoint{
				X: float64(rotatedHeight) - p.Y - 1,
				Y: p.X,
			}
		}
	}
	return result, nil
}

// Reset is a no-op for 1D readers.
func (r *MultiFormatOneDReader) Reset() {}
