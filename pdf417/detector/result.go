// Package detector implements PDF417 barcode detection in binary images.
package detector

import (
	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// PDF417DetectorResult encapsulates the results of detecting one or more
// PDF417 barcodes in an image.
type PDF417DetectorResult struct {
	Bits     *bitutil.BitMatrix
	Points   [][]*zxinggo.ResultPoint
	Rotation int
}
