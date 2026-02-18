package zxinggo

// MultipleBarcodeReader can decode multiple barcodes from a single image.
type MultipleBarcodeReader interface {
	// DecodeMultiple attempts to decode all barcodes in the image.
	DecodeMultiple(image *BinaryBitmap, opts *DecodeOptions) ([]*Result, error)
}
