package qrcode

import (
	"fmt"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
	"github.com/ericlevine/zxinggo/qrcode/decoder"
	"github.com/ericlevine/zxinggo/qrcode/encoder"
)

const defaultQuietZoneSize = 4

// Writer encodes QR codes.
type Writer struct{}

// NewWriter creates a new QR code Writer.
func NewWriter() *Writer {
	return &Writer{}
}

// Encode encodes the given contents into a QR code BitMatrix.
func (w *Writer) Encode(contents string, format zxinggo.Format, width, height int, opts *zxinggo.EncodeOptions) (*bitutil.BitMatrix, error) {
	if contents == "" {
		return nil, fmt.Errorf("found empty contents")
	}
	if format != zxinggo.FormatQRCode {
		return nil, fmt.Errorf("can only encode QR_CODE, but got %s", format)
	}
	if width < 0 || height < 0 {
		return nil, fmt.Errorf("requested dimensions are too small: %dx%d", width, height)
	}

	ecLevel := decoder.ECLevelL
	quietZone := defaultQuietZoneSize
	qrVersion := 0
	maskPattern := -1

	if opts != nil {
		if opts.ErrorCorrection != "" {
			switch opts.ErrorCorrection {
			case "L":
				ecLevel = decoder.ECLevelL
			case "M":
				ecLevel = decoder.ECLevelM
			case "Q":
				ecLevel = decoder.ECLevelQ
			case "H":
				ecLevel = decoder.ECLevelH
			default:
				return nil, fmt.Errorf("unknown error correction level: %s", opts.ErrorCorrection)
			}
		}
		if opts.Margin != nil {
			quietZone = *opts.Margin
		}
		if opts.QRVersion > 0 {
			qrVersion = opts.QRVersion
		}
		if opts.QRMaskPattern >= 0 && opts.QRMaskPattern <= 7 {
			maskPattern = opts.QRMaskPattern
		}
	}

	code, err := encoder.Encode(contents, ecLevel, qrVersion, maskPattern)
	if err != nil {
		return nil, err
	}
	return encoder.RenderResult(code, width, height, quietZone), nil
}
