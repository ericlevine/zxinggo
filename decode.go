package zxinggo

// DecodeOptions configures barcode decoding behavior.
type DecodeOptions struct {
	// PureBarcode hints that the image contains only the barcode with minimal
	// border and no rotation.
	PureBarcode bool

	// TryHarder enables spending more time looking for barcodes.
	TryHarder bool

	// PossibleFormats limits which formats to look for.
	PossibleFormats []Format

	// CharacterSet specifies the character set to use when decoding.
	CharacterSet string

	// AllowedLengths restricts the set of valid barcode lengths for 1D formats.
	AllowedLengths []int

	// AssumeCode39CheckDigit assumes Code 39 includes a check digit.
	AssumeCode39CheckDigit bool

	// AssumeGS1 assumes data is GS1 formatted.
	AssumeGS1 bool

	// AllowedEANExtensions restricts the allowed EAN extension lengths.
	AllowedEANExtensions []int

	// AlsoInverted enables checking for barcodes on inverted images.
	AlsoInverted bool
}

// Reader decodes barcodes from a BinaryBitmap.
type Reader interface {
	// Decode attempts to decode a barcode from the image.
	Decode(image *BinaryBitmap, opts *DecodeOptions) (*Result, error)

	// Reset resets any internal state.
	Reset()
}
