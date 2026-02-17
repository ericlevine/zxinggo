package pdf417

import zxinggo "github.com/ericlevine/zxinggo"

func init() {
	zxinggo.RegisterReader(zxinggo.FormatPDF417, func(opts *zxinggo.DecodeOptions) zxinggo.Reader {
		return NewPDF417Reader()
	})
	zxinggo.RegisterWriter(zxinggo.FormatPDF417, func() zxinggo.Writer {
		return NewPDF417Writer()
	})
}
