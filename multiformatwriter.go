package zxinggo

import (
	"fmt"

	"github.com/ericlevine/zxinggo/bitutil"
)

// MultiFormatWriter is a factory/dispatcher that selects the appropriate Writer
// implementation based on the requested format.
type MultiFormatWriter struct{}

// NewMultiFormatWriter creates a new multi-format writer.
func NewMultiFormatWriter() *MultiFormatWriter {
	return &MultiFormatWriter{}
}

// writerFactory is a function that creates a Writer.
type writerFactory func() Writer

var writerFactories = map[Format]writerFactory{}

// RegisterWriter registers a writer factory for the given format.
func RegisterWriter(format Format, factory writerFactory) {
	writerFactories[format] = factory
}

// Encode encodes the given contents into a barcode of the specified format.
func (w *MultiFormatWriter) Encode(contents string, format Format, width, height int, opts *EncodeOptions) (*bitutil.BitMatrix, error) {
	factory, ok := writerFactories[format]
	if !ok {
		return nil, fmt.Errorf("no writer registered for format %s: %w", format, ErrWriter)
	}
	writer := factory()
	return writer.Encode(contents, format, width, height, opts)
}

// Encode is a top-level convenience function that encodes the given contents
// into a barcode of the specified format.
func Encode(contents string, format Format, width, height int, opts *EncodeOptions) (*bitutil.BitMatrix, error) {
	w := NewMultiFormatWriter()
	return w.Encode(contents, format, width, height, opts)
}

// Decode is a top-level convenience function that decodes a barcode from the
// given BinaryBitmap.
func Decode(image *BinaryBitmap, opts *DecodeOptions) (*Result, error) {
	r := NewMultiFormatReader()
	return r.Decode(image, opts)
}
