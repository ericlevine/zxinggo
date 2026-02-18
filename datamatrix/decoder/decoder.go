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
	dataBlocksCount := len(dataBlocks)
	totalErrorsCorrected := 0

	for j := 0; j < dataBlocksCount; j++ {
		codewordBytes := dataBlocks[j].Codewords
		numDataCodewords := dataBlocks[j].NumDataCodewords

		corrected, err := d.correctErrors(codewordBytes, numDataCodewords)
		if err != nil {
			return nil, err
		}
		totalErrorsCorrected += corrected

		// De-interlace data blocks: block j's i-th codeword goes to
		// position i*dataBlocksCount+j in the result.
		for i := 0; i < numDataCodewords; i++ {
			resultBytes[i*dataBlocksCount+j] = codewordBytes[i]
		}
	}

	// Step 4: Decode the data codewords into text.
	dr, err := DecodeBitStream(resultBytes)
	if err != nil {
		return nil, err
	}
	dr.ErrorsCorrected = totalErrorsCorrected
	dr.SymbologyModifier = 1
	return dr, nil
}

// correctErrors uses Reed-Solomon error correction to fix errors in a block.
func (d *Decoder) correctErrors(codewordBytes []byte, numDataCodewords int) (int, error) {
	numCodewords := len(codewordBytes)

	// Convert to int slice for RS decoder
	codewordsInts := make([]int, numCodewords)
	for i := 0; i < numCodewords; i++ {
		codewordsInts[i] = int(codewordBytes[i]) & 0xFF
	}

	numECCodewords := numCodewords - numDataCodewords
	errorsCorrected, err := d.rsDecoder.Decode(codewordsInts, numECCodewords)
	if err != nil {
		return 0, zxinggo.ErrChecksum
	}

	// Copy corrected values back
	for i := 0; i < numDataCodewords; i++ {
		codewordBytes[i] = byte(codewordsInts[i])
	}
	return errorsCorrected, nil
}
