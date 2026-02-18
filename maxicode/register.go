package maxicode

import zxinggo "github.com/ericlevine/zxinggo"

func init() {
	zxinggo.RegisterReader(zxinggo.FormatMaxiCode, func(opts *zxinggo.DecodeOptions) zxinggo.Reader {
		return NewReader()
	})
}
