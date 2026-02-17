package zxinggo

import "errors"

var (
	// ErrNotFound is returned when a barcode is not found in the image.
	ErrNotFound = errors.New("barcode not found")

	// ErrChecksum is returned when a barcode's checksum does not match.
	ErrChecksum = errors.New("checksum error")

	// ErrFormat is returned when a barcode cannot be decoded due to format issues.
	ErrFormat = errors.New("format error")

	// ErrWriter is returned when a barcode cannot be encoded.
	ErrWriter = errors.New("writer error")
)
