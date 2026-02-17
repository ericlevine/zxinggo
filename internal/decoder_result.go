// Package internal provides shared result types used across barcode format packages.
package internal

// DecoderResult encapsulates the result of decoding a matrix of bits.
type DecoderResult struct {
	RawBytes                      []byte
	NumBits                       int
	Text                          string
	ByteSegments                  [][]byte
	ECLevel                       string
	ErrorsCorrected               int
	Erasures                      int
	Other                         interface{}
	StructuredAppendParity        int
	StructuredAppendSequenceNumber int
	SymbologyModifier             int
}

// NewDecoderResult creates a DecoderResult with the basic fields.
func NewDecoderResult(rawBytes []byte, text string, byteSegments [][]byte, ecLevel string) *DecoderResult {
	numBits := 0
	if rawBytes != nil {
		numBits = 8 * len(rawBytes)
	}
	return &DecoderResult{
		RawBytes:                       rawBytes,
		NumBits:                        numBits,
		Text:                           text,
		ByteSegments:                   byteSegments,
		ECLevel:                        ecLevel,
		StructuredAppendParity:         -1,
		StructuredAppendSequenceNumber: -1,
	}
}

// NewDecoderResultFull creates a DecoderResult with structured append info.
func NewDecoderResultFull(rawBytes []byte, text string, byteSegments [][]byte,
	ecLevel string, saSequence, saParity, symbologyModifier int) *DecoderResult {
	numBits := 0
	if rawBytes != nil {
		numBits = 8 * len(rawBytes)
	}
	return &DecoderResult{
		RawBytes:                       rawBytes,
		NumBits:                        numBits,
		Text:                           text,
		ByteSegments:                   byteSegments,
		ECLevel:                        ecLevel,
		StructuredAppendParity:         saParity,
		StructuredAppendSequenceNumber: saSequence,
		SymbologyModifier:              symbologyModifier,
	}
}

// HasStructuredAppend returns true if this result has structured append info.
func (d *DecoderResult) HasStructuredAppend() bool {
	return d.StructuredAppendParity >= 0 && d.StructuredAppendSequenceNumber >= 0
}
