package zxinggo

import "github.com/ericlevine/zxinggo/bitutil"

// EncodeOptions configures barcode encoding behavior.
type EncodeOptions struct {
	// ErrorCorrection specifies the error correction level.
	ErrorCorrection string

	// CharacterSet specifies the character set to use when encoding.
	CharacterSet string

	// Margin specifies the margin (quiet zone) in modules around the barcode.
	Margin *int

	// QRVersion forces a specific QR version (1-40).
	QRVersion int

	// QRMaskPattern forces a specific QR mask pattern (0-7).
	QRMaskPattern int

	// QRCompact enables compact QR mode.
	QRCompact bool

	// PDF417Compact enables compact PDF417 mode.
	PDF417Compact bool

	// PDF417Compaction specifies the PDF417 compaction mode.
	PDF417Compaction int

	// PDF417Dimensions specifies min/max rows/cols for PDF417.
	PDF417Dimensions *PDF417DimensionConfig

	// PDF417AutoECI enables automatic ECI selection in PDF417.
	PDF417AutoECI bool

	// GS1Format encodes in GS1 format.
	GS1Format bool

	// ForceCodeSet forces a specific code set (e.g., for Code 128).
	ForceCodeSet string

	// Code128Compact enables compact Code 128 encoding.
	Code128Compact bool
}

// PDF417DimensionConfig specifies min/max rows/cols for PDF417.
type PDF417DimensionConfig struct {
	MinRows, MaxRows int
	MinCols, MaxCols int
}

// Writer encodes data into a barcode.
type Writer interface {
	// Encode encodes the given contents into a barcode.
	Encode(contents string, format Format, width, height int, opts *EncodeOptions) (*bitutil.BitMatrix, error)
}
