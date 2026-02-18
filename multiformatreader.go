package zxinggo

import "fmt"

// MultiFormatReader is a factory/dispatcher that selects appropriate Reader
// implementations based on format hints and tries them in sequence.
type MultiFormatReader struct {
	readers []Reader
}

// NewMultiFormatReader creates a new multi-format reader. If opts specifies
// PossibleFormats, only those formats are tried. Otherwise all formats are tried.
func NewMultiFormatReader() *MultiFormatReader {
	return &MultiFormatReader{}
}

// Decode attempts to decode a barcode from the given image using all registered
// format readers.
func (r *MultiFormatReader) Decode(image *BinaryBitmap, opts *DecodeOptions) (*Result, error) {
	if r.readers == nil {
		r.readers = buildReaders(opts)
	}
	for _, reader := range r.readers {
		result, err := reader.Decode(image, opts)
		if err == nil {
			return result, nil
		}
	}
	if opts != nil && opts.AlsoInverted {
		// Try again with inverted image — flip the cached black matrix in-place
		matrix, err := image.BlackMatrix()
		if err == nil {
			matrix.FlipAll()
			for _, reader := range r.readers {
				result, err := reader.Decode(image, opts)
				if err == nil {
					return result, nil
				}
			}
		}
	}
	return nil, ErrNotFound
}

// DecodeWithFormat attempts to decode a barcode of the given format.
func (r *MultiFormatReader) DecodeWithFormat(image *BinaryBitmap, format Format, opts *DecodeOptions) (*Result, error) {
	if opts == nil {
		opts = &DecodeOptions{}
	}
	opts.PossibleFormats = []Format{format}
	readers := buildReaders(opts)
	for _, reader := range readers {
		result, err := reader.Decode(image, opts)
		if err == nil {
			return result, nil
		}
	}
	return nil, fmt.Errorf("no barcode of format %s found: %w", format, ErrNotFound)
}

// Reset resets all internal readers.
func (r *MultiFormatReader) Reset() {
	for _, reader := range r.readers {
		reader.Reset()
	}
	r.readers = nil
}

// readerFactory is a function that creates a Reader. This is used as an
// extension point so format-specific packages can register themselves.
type readerFactory func(opts *DecodeOptions) Reader

var readerFactories = map[Format]readerFactory{}

// RegisterReader registers a reader factory for the given format. This should
// be called from an init() function in format-specific packages.
func RegisterReader(format Format, factory readerFactory) {
	readerFactories[format] = factory
}

// buildReaders creates readers based on the options.
func buildReaders(opts *DecodeOptions) []Reader {
	var readers []Reader

	if opts != nil && len(opts.PossibleFormats) > 0 {
		for _, f := range opts.PossibleFormats {
			if factory, ok := readerFactories[f]; ok {
				readers = append(readers, factory(opts))
			}
		}
	}

	if len(readers) == 0 {
		// Try all registered readers
		for _, factory := range readerFactories {
			readers = append(readers, factory(opts))
		}
	}

	return readers
}
