package oned

import zxinggo "github.com/ericlevine/zxinggo"

func init() {
	// Register all 1D readers via the multi-format 1D reader.
	oneDReaderFactory := func(opts *zxinggo.DecodeOptions) zxinggo.Reader {
		return NewMultiFormatOneDReader(opts)
	}
	zxinggo.RegisterReader(zxinggo.FormatCode128, oneDReaderFactory)
	zxinggo.RegisterReader(zxinggo.FormatCode39, oneDReaderFactory)
	zxinggo.RegisterReader(zxinggo.FormatEAN13, oneDReaderFactory)
	zxinggo.RegisterReader(zxinggo.FormatEAN8, oneDReaderFactory)
	zxinggo.RegisterReader(zxinggo.FormatUPCA, oneDReaderFactory)
	zxinggo.RegisterReader(zxinggo.FormatUPCE, oneDReaderFactory)
	zxinggo.RegisterReader(zxinggo.FormatITF, oneDReaderFactory)
	zxinggo.RegisterReader(zxinggo.FormatCodabar, oneDReaderFactory)
	zxinggo.RegisterReader(zxinggo.FormatRSS14, oneDReaderFactory)
	zxinggo.RegisterReader(zxinggo.FormatRSSExpanded, oneDReaderFactory)
	zxinggo.RegisterReader(zxinggo.FormatCode93, oneDReaderFactory)

	// Register writers
	zxinggo.RegisterWriter(zxinggo.FormatCode128, func() zxinggo.Writer { return NewCode128Writer() })
	zxinggo.RegisterWriter(zxinggo.FormatCode39, func() zxinggo.Writer { return NewCode39Writer() })
	zxinggo.RegisterWriter(zxinggo.FormatEAN13, func() zxinggo.Writer { return NewEAN13Writer() })
	zxinggo.RegisterWriter(zxinggo.FormatEAN8, func() zxinggo.Writer { return NewEAN8Writer() })
	zxinggo.RegisterWriter(zxinggo.FormatUPCA, func() zxinggo.Writer { return NewUPCAWriter() })
	zxinggo.RegisterWriter(zxinggo.FormatUPCE, func() zxinggo.Writer { return NewUPCEWriter() })
	zxinggo.RegisterWriter(zxinggo.FormatITF, func() zxinggo.Writer { return NewITFWriter() })
	zxinggo.RegisterWriter(zxinggo.FormatCodabar, func() zxinggo.Writer { return NewCodabarWriter() })
	zxinggo.RegisterWriter(zxinggo.FormatCode93, func() zxinggo.Writer { return NewCode93Writer() })
}
