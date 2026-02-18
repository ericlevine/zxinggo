package decoder

import "fmt"

// DataBlock represents a block of data and error-correction codewords.
type DataBlock struct {
	NumDataCodewords int
	Codewords        []byte
}

// GetDataBlocks separates interleaved Data Matrix codewords into data and EC blocks.
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

	// EC codewords per block
	ecCodewordsPerBlock := ecBlocks.ECCodewords / totalBlocks

	result := make([]DataBlock, totalBlocks)
	blockIndex := 0
	for _, block := range ecBlocks.Blocks {
		for i := 0; i < block.Count; i++ {
			numDataCodewords := block.DataCodewords
			numBlockCodewords := numDataCodewords + ecCodewordsPerBlock
			result[blockIndex] = DataBlock{
				NumDataCodewords: numDataCodewords,
				Codewords:        make([]byte, numBlockCodewords),
			}
			blockIndex++
		}
	}

	// Data Matrix interleaving: data codewords are interleaved across blocks,
	// then EC codewords are interleaved across blocks.

	// Find the shorter and longer data block sizes
	shorterBlocksNumDataCodewords := result[0].NumDataCodewords
	longerBlocksStartAt := totalBlocks

	// Find where longer blocks start (blocks may differ by 1 data codeword)
	for i := 0; i < totalBlocks; i++ {
		if result[i].NumDataCodewords > shorterBlocksNumDataCodewords {
			longerBlocksStartAt = i
			break
		}
	}

	// De-interleave data codewords
	rawCodewordsOffset := 0
	for i := 0; i < shorterBlocksNumDataCodewords; i++ {
		for j := 0; j < totalBlocks; j++ {
			if rawCodewordsOffset >= len(rawCodewords) {
				return nil, fmt.Errorf("datamatrix/decoder: not enough raw codewords")
			}
			result[j].Codewords[i] = rawCodewords[rawCodewordsOffset]
			rawCodewordsOffset++
		}
	}

	// Handle longer blocks (extra data codeword)
	for j := longerBlocksStartAt; j < totalBlocks; j++ {
		if rawCodewordsOffset >= len(rawCodewords) {
			return nil, fmt.Errorf("datamatrix/decoder: not enough raw codewords")
		}
		result[j].Codewords[shorterBlocksNumDataCodewords] = rawCodewords[rawCodewordsOffset]
		rawCodewordsOffset++
	}

	// De-interleave EC codewords
	for i := 0; i < ecCodewordsPerBlock; i++ {
		for j := 0; j < totalBlocks; j++ {
			iOffset := result[j].NumDataCodewords + i
			if rawCodewordsOffset >= len(rawCodewords) {
				return nil, fmt.Errorf("datamatrix/decoder: not enough raw codewords")
			}
			result[j].Codewords[iOffset] = rawCodewords[rawCodewordsOffset]
			rawCodewordsOffset++
		}
	}

	if rawCodewordsOffset != len(rawCodewords) {
		return nil, fmt.Errorf("datamatrix/decoder: raw codewords count mismatch: used %d of %d", rawCodewordsOffset, len(rawCodewords))
	}

	return result, nil
}
