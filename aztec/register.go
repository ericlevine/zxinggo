package aztec

import zxinggo "github.com/ericlevine/zxinggo"

func init() {
	zxinggo.RegisterReader(zxinggo.FormatAztec, func(opts *zxinggo.DecodeOptions) zxinggo.Reader {
		return NewReader()
	})
	zxinggo.RegisterWriter(zxinggo.FormatAztec, func() zxinggo.Writer {
		return NewWriter()
	})
}
