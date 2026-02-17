package oned

import (
	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// MultiFormatOneDReader attempts to decode 1D barcodes by trying multiple
// format-specific readers in sequence.
type MultiFormatOneDReader struct {
	readers []RowDecoder
}

// NewMultiFormatOneDReader creates a new multi-format reader configured by opts.
func NewMultiFormatOneDReader(opts *zxinggo.DecodeOptions) *MultiFormatOneDReader {
	var readers []RowDecoder

	if opts != nil && len(opts.PossibleFormats) > 0 {
		formats := make(map[zxinggo.Format]bool)
		for _, f := range opts.PossibleFormats {
			formats[f] = true
		}
		if formats[zxinggo.FormatEAN13] || formats[zxinggo.FormatUPCA] ||
			formats[zxinggo.FormatEAN8] || formats[zxinggo.FormatUPCE] {
			readers = append(readers, NewEAN13Reader(), NewEAN8Reader(), NewUPCAReader(), NewUPCEReader())
		}
		if formats[zxinggo.FormatCode39] {
			useCheckDigit := opts.AssumeCode39CheckDigit
			readers = append(readers, NewCode39ReaderWithCheckDigit(useCheckDigit, false))
		}
		if formats[zxinggo.FormatCode128] {
			readers = append(readers, NewCode128Reader())
		}
	}

	if len(readers) == 0 {
		readers = []RowDecoder{
			NewEAN13Reader(),
			NewEAN8Reader(),
			NewUPCAReader(),
			NewUPCEReader(),
			NewCode39Reader(),
			NewCode128Reader(),
		}
	}

	return &MultiFormatOneDReader{readers: readers}
}

// DecodeRow tries each reader in sequence until one succeeds.
func (r *MultiFormatOneDReader) DecodeRow(rowNumber int, row *bitutil.BitArray, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	for _, reader := range r.readers {
		result, err := reader.DecodeRow(rowNumber, row, opts)
		if err == nil {
			return result, nil
		}
	}
	return nil, zxinggo.ErrNotFound
}

// Decode decodes a 1D barcode from the given image.
func (r *MultiFormatOneDReader) Decode(image *zxinggo.BinaryBitmap, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	return DecodeOneD(image, r, opts)
}

// Reset is a no-op for 1D readers.
func (r *MultiFormatOneDReader) Reset() {}
