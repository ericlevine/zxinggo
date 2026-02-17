package qrcode

import zxinggo "github.com/ericlevine/zxinggo"

func init() {
	zxinggo.RegisterReader(zxinggo.FormatQRCode, func(opts *zxinggo.DecodeOptions) zxinggo.Reader {
		return NewReader()
	})
	zxinggo.RegisterWriter(zxinggo.FormatQRCode, func() zxinggo.Writer {
		return NewWriter()
	})
}
