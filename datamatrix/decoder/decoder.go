package decoder

import (
	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
	"github.com/ericlevine/zxinggo/reedsolomon"
)

// Decoder decodes Data Matrix ECC-200 barcodes.
type Decoder struct {
	rsDecoder *reedsolomon.Decoder
}

// NewDecoder creates a new Data Matrix Decoder.
func NewDecoder() *Decoder {
	return &Decoder{
		rsDecoder: reedsolomon.NewDecoder(reedsolomon.DataMatrixField256),
	}
}

// Decode decodes a Data Matrix bit matrix into a DecoderResult.
// The input BitMatrix should represent the full Data Matrix symbol including
// finder patterns and timing.
func (d *Decoder) Decode(bits *bitutil.BitMatrix) (*DecoderResult, error) {
	// Step 1: Read raw codewords from the bit matrix using the placement algorithm.
	rawCodewords, version, err := ReadCodewords(bits)
	if err != nil {
		return nil, err
	}

	// Step 2: Split raw codewords into data and EC blocks.
	dataBlocks, err := GetDataBlocks(rawCodewords, version)
	if err != nil {
		return nil, err
	}

	// Step 3: Error-correct each block using Reed-Solomon.
	totalDataBytes := 0
	for _, db := range dataBlocks {
		totalDataBytes += db.NumDataCodewords
	}

	resultBytes := make([]byte, totalDataBytes)
	dataBlocksOffset := 0

	for _, db := range dataBlocks {
		codewordBytes := db.Codewords
		numDataCodewords := db.NumDataCodewords

		if err := d.correctErrors(codewordBytes, numDataCodewords); err != nil {
			return nil, err
		}

		// Copy the corrected data codewords to the result
		copy(resultBytes[dataBlocksOffset:], codewordBytes[:numDataCodewords])
		dataBlocksOffset += numDataCodewords
	}

	// Step 4: Decode the data codewords into text.
	return DecodeBitStream(resultBytes)
}

// correctErrors uses Reed-Solomon error correction to fix errors in a block.
func (d *Decoder) correctErrors(codewordBytes []byte, numDataCodewords int) error {
	numCodewords := len(codewordBytes)

	// Convert to int slice for RS decoder
	codewordsInts := make([]int, numCodewords)
	for i := 0; i < numCodewords; i++ {
		codewordsInts[i] = int(codewordBytes[i]) & 0xFF
	}

	numECCodewords := numCodewords - numDataCodewords
	_, err := d.rsDecoder.Decode(codewordsInts, numECCodewords)
	if err != nil {
		return zxinggo.ErrChecksum
	}

	// Copy corrected values back
	for i := 0; i < numDataCodewords; i++ {
		codewordBytes[i] = byte(codewordsInts[i])
	}
	return nil
}
