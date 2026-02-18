package datamatrix

import zxinggo "github.com/ericlevine/zxinggo"

func init() {
	zxinggo.RegisterReader(zxinggo.FormatDataMatrix, func(opts *zxinggo.DecodeOptions) zxinggo.Reader {
		return NewReader()
	})
	zxinggo.RegisterWriter(zxinggo.FormatDataMatrix, func() zxinggo.Writer {
		return NewWriter()
	})
}
