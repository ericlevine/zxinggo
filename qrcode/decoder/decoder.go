package decoder

import (
	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
	"github.com/ericlevine/zxinggo/internal"
	"github.com/ericlevine/zxinggo/reedsolomon"
)

// Decoder decodes QR codes.
type Decoder struct {
	rsDecoder *reedsolomon.Decoder
}

// NewDecoder creates a new QR code Decoder.
func NewDecoder() *Decoder {
	return &Decoder{
		rsDecoder: reedsolomon.NewDecoder(reedsolomon.QRCodeField256),
	}
}

// Decode decodes a BitMatrix into a DecoderResult.
func (d *Decoder) Decode(bits *bitutil.BitMatrix, characterSet string) (*internal.DecoderResult, error) {
	parser, err := NewBitMatrixParser(bits)
	if err != nil {
		return nil, err
	}

	result, err := d.decodeParser(parser, characterSet)
	if err == nil {
		return result, nil
	}

	// Try mirrored reading
	parser.Remask()
	parser.SetMirror(true)

	if _, verr := parser.ReadVersion(); verr != nil {
		return nil, err // return original error
	}
	if _, ferr := parser.ReadFormatInformation(); ferr != nil {
		return nil, err
	}

	parser.Mirror()

	result, err2 := d.decodeParser(parser, characterSet)
	if err2 != nil {
		return nil, err // return original error
	}
	return result, nil
}

func (d *Decoder) decodeParser(parser *BitMatrixParser, characterSet string) (*internal.DecoderResult, error) {
	version, err := parser.ReadVersion()
	if err != nil {
		return nil, err
	}
	formatInfo, err := parser.ReadFormatInformation()
	if err != nil {
		return nil, err
	}
	ecLevel := formatInfo.ECLevel

	codewords, err := parser.ReadCodewords()
	if err != nil {
		return nil, err
	}

	dataBlocks := GetDataBlocks(codewords, version, ecLevel)

	totalBytes := 0
	for _, db := range dataBlocks {
		totalBytes += db.NumDataCodewords
	}
	resultBytes := make([]byte, totalBytes)
	resultOffset := 0

	errorsCorrected := 0
	for _, db := range dataBlocks {
		corrected, err := d.correctErrors(db.Codewords, db.NumDataCodewords)
		if err != nil {
			return nil, err
		}
		errorsCorrected += corrected
		copy(resultBytes[resultOffset:], db.Codewords[:db.NumDataCodewords])
		resultOffset += db.NumDataCodewords
	}

	result, err := DecodeBitStream(resultBytes, version, ecLevel, characterSet)
	if err != nil {
		return nil, err
	}
	result.ErrorsCorrected = errorsCorrected
	return result, nil
}

func (d *Decoder) correctErrors(codewordBytes []byte, numDataCodewords int) (int, error) {
	numCodewords := len(codewordBytes)
	codewordsInts := make([]int, numCodewords)
	for i := 0; i < numCodewords; i++ {
		codewordsInts[i] = int(codewordBytes[i]) & 0xFF
	}
	corrected, err := d.rsDecoder.Decode(codewordsInts, numCodewords-numDataCodewords)
	if err != nil {
		return 0, zxinggo.ErrChecksum
	}
	for i := 0; i < numDataCodewords; i++ {
		codewordBytes[i] = byte(codewordsInts[i])
	}
	return corrected, nil
}
