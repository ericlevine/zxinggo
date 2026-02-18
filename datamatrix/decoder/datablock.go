package decoder

import "fmt"

// DataBlock represents a block of data and error-correction codewords.
type DataBlock struct {
	NumDataCodewords int
	Codewords        []byte
}

// GetDataBlocks separates interleaved Data Matrix codewords into data and EC blocks.
// This is a faithful port of the Java ZXing DataBlock.getDataBlocks method.
// Data Matrix interleaves codewords across blocks: first all data codewords are
// interleaved, then all EC codewords are interleaved.
func GetDataBlocks(rawCodewords []byte, version *Version) ([]DataBlock, error) {
	ecBlocks := version.GetECBlocks()

	// Count total blocks
	totalBlocks := 0
	for _, block := range ecBlocks.Blocks {
		totalBlocks += block.Count
	}

	if totalBlocks == 0 {
		return nil, fmt.Errorf("datamatrix/decoder: no EC blocks defined")
	}

	// ECCodewords is the number of EC codewords per block
	ecCodewordsPerBlock := ecBlocks.ECCodewords

	// Now establish DataBlocks of the appropriate size and number of data codewords
	result := make([]DataBlock, totalBlocks)
	numResultBlocks := 0
	for _, block := range ecBlocks.Blocks {
		for i := 0; i < block.Count; i++ {
			numDataCodewords := block.DataCodewords
			numBlockCodewords := ecCodewordsPerBlock + numDataCodewords
			result[numResultBlocks] = DataBlock{
				NumDataCodewords: numDataCodewords,
				Codewords:        make([]byte, numBlockCodewords),
			}
			numResultBlocks++
		}
	}

	// All blocks have the same amount of data, except that the last n
	// (where n may be 0) have 1 less byte. Figure out where these start.
	// There is only one case where there is a difference for Data Matrix for size 144
	longerBlocksTotalCodewords := len(result[0].Codewords)

	longerBlocksNumDataCodewords := longerBlocksTotalCodewords - ecCodewordsPerBlock
	shorterBlocksNumDataCodewords := longerBlocksNumDataCodewords - 1

	// The last elements of result may be 1 element shorter for 144 matrix
	// first fill out as many elements as all of them have minus 1
	rawCodewordsOffset := 0
	for i := 0; i < shorterBlocksNumDataCodewords; i++ {
		for j := 0; j < numResultBlocks; j++ {
			if rawCodewordsOffset >= len(rawCodewords) {
				return nil, fmt.Errorf("datamatrix/decoder: not enough raw codewords")
			}
			result[j].Codewords[i] = rawCodewords[rawCodewordsOffset]
			rawCodewordsOffset++
		}
	}

	// Fill out the last data block in the longer ones
	specialVersion := version.VersionNumber() == 24
	numLongerBlocks := numResultBlocks
	if specialVersion {
		numLongerBlocks = 8
	}
	for j := 0; j < numLongerBlocks; j++ {
		if rawCodewordsOffset >= len(rawCodewords) {
			return nil, fmt.Errorf("datamatrix/decoder: not enough raw codewords")
		}
		result[j].Codewords[longerBlocksNumDataCodewords-1] = rawCodewords[rawCodewordsOffset]
		rawCodewordsOffset++
	}

	// Now add in error correction blocks
	max := len(result[0].Codewords)
	for i := longerBlocksNumDataCodewords; i < max; i++ {
		for j := 0; j < numResultBlocks; j++ {
			jOffset := j
			iOffset := i
			if specialVersion {
				jOffset = (j + 8) % numResultBlocks
				if jOffset > 7 {
					iOffset = i - 1
				}
			}
			if rawCodewordsOffset >= len(rawCodewords) {
				return nil, fmt.Errorf("datamatrix/decoder: not enough raw codewords")
			}
			result[jOffset].Codewords[iOffset] = rawCodewords[rawCodewordsOffset]
			rawCodewordsOffset++
		}
	}

	if rawCodewordsOffset != len(rawCodewords) {
		return nil, fmt.Errorf("datamatrix/decoder: raw codewords count mismatch: used %d of %d", rawCodewordsOffset, len(rawCodewords))
	}

	return result, nil
}
