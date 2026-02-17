package decoder

import "errors"

var (
	errInvalidECLevel = errors.New("qrcode/decoder: invalid error correction level")
	errInvalidMode    = errors.New("qrcode/decoder: invalid mode")
	errInvalidVersion = errors.New("qrcode/decoder: invalid version number")
)
