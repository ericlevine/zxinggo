package decoder

// DataBlock represents a block of data and error-correction codewords.
type DataBlock struct {
	NumDataCodewords int
	Codewords        []byte
}

// GetDataBlocks separates interleaved QR code data into original blocks.
func GetDataBlocks(rawCodewords []byte, version *Version, ecLevel ErrorCorrectionLevel) []DataBlock {
	ecBlocks := version.ECBlocksForLevel(ecLevel)

	// Count total blocks
	totalBlocks := 0
	for _, block := range ecBlocks.Blocks {
		totalBlocks += block.Count
	}

	result := make([]DataBlock, totalBlocks)
	numResultBlocks := 0
	for _, block := range ecBlocks.Blocks {
		for i := 0; i < block.Count; i++ {
			numDataCodewords := block.DataCodewords
			numBlockCodewords := ecBlocks.ECCodewordsPerBlock + numDataCodewords
			result[numResultBlocks] = DataBlock{
				NumDataCodewords: numDataCodewords,
				Codewords:        make([]byte, numBlockCodewords),
			}
			numResultBlocks++
		}
	}

	// Find where longer blocks start
	shorterBlocksTotalCodewords := len(result[0].Codewords)
	longerBlocksStartAt := len(result) - 1
	for longerBlocksStartAt >= 0 {
		if len(result[longerBlocksStartAt].Codewords) == shorterBlocksTotalCodewords {
			break
		}
		longerBlocksStartAt--
	}
	longerBlocksStartAt++

	shorterBlocksNumDataCodewords := shorterBlocksTotalCodewords - ecBlocks.ECCodewordsPerBlock

	// De-interleave: fill data codewords
	rawCodewordsOffset := 0
	for i := 0; i < shorterBlocksNumDataCodewords; i++ {
		for j := 0; j < numResultBlocks; j++ {
			result[j].Codewords[i] = rawCodewords[rawCodewordsOffset]
			rawCodewordsOffset++
		}
	}
	// Fill extra data byte in longer blocks
	for j := longerBlocksStartAt; j < numResultBlocks; j++ {
		result[j].Codewords[shorterBlocksNumDataCodewords] = rawCodewords[rawCodewordsOffset]
		rawCodewordsOffset++
	}
	// Fill EC codewords
	max := len(result[0].Codewords)
	for i := shorterBlocksNumDataCodewords; i < max; i++ {
		for j := 0; j < numResultBlocks; j++ {
			iOffset := i
			if j >= longerBlocksStartAt {
				iOffset = i + 1
			}
			result[j].Codewords[iOffset] = rawCodewords[rawCodewordsOffset]
			rawCodewordsOffset++
		}
	}

	return result
}
