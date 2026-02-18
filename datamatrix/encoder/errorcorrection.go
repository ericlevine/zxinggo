// Copyright 2006 Jeremias Maerki in part, and ZXing Authors in part.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Ported from Java ZXing library.

package encoder

import (
	"fmt"

	"github.com/ericlevine/zxinggo/reedsolomon"
)

// EncodeECC200 generates Reed-Solomon ECC-200 error correction codewords and
// returns the full codeword sequence (data + EC). Data Matrix uses GF(256)
// with primitive polynomial 0x12D (already defined as reedsolomon.DataMatrixField256).
//
// For symbols with interleaved blocks, data is distributed across blocks,
// each block is independently error-corrected, and the results are interleaved
// back together.
func EncodeECC200(codewords []byte, symbolInfo *SymbolInfo) ([]byte, error) {
	if len(codewords) != symbolInfo.DataCapacity {
		return nil, fmt.Errorf("datamatrix/encoder: expected %d data codewords, got %d",
			symbolInfo.DataCapacity, len(codewords))
	}

	blockCount := symbolInfo.InterleavedBlockCount()
	ecPerBlock := symbolInfo.RSBlockError
	totalEC := blockCount * ecPerBlock

	result := make([]byte, symbolInfo.DataCapacity+totalEC)
	copy(result, codewords)

	// If there is only one block with no interleaving, encode directly.
	if blockCount == 1 {
		ec, err := generateECCBlock(codewords, ecPerBlock)
		if err != nil {
			return nil, err
		}
		copy(result[symbolInfo.DataCapacity:], ec)
		return result, nil
	}

	// Multiple interleaved blocks: de-interleave, encode each block, re-interleave.
	block1Count := symbolInfo.dataCodewordsPerBlock1Count()
	block1Data := symbolInfo.RSBlockData
	block2Data := symbolInfo.RSBlockData2
	if block2Data == 0 {
		block2Data = block1Data
	}

	// De-interleave data into blocks.
	blocks := make([][]byte, blockCount)
	for i := 0; i < blockCount; i++ {
		dataLen := block1Data
		if i >= block1Count {
			dataLen = block2Data
		}
		blocks[i] = make([]byte, dataLen)
	}

	// Data codewords are interleaved: codeword[0] -> block[0], codeword[1] -> block[1], etc.
	for i := 0; i < len(codewords); i++ {
		blockIdx := i % blockCount
		posInBlock := i / blockCount
		if posInBlock < len(blocks[blockIdx]) {
			blocks[blockIdx][posInBlock] = codewords[i]
		}
	}

	// Generate EC for each block and interleave EC codewords.
	ecBlocks := make([][]byte, blockCount)
	for i := 0; i < blockCount; i++ {
		ec, err := generateECCBlock(blocks[i], ecPerBlock)
		if err != nil {
			return nil, err
		}
		ecBlocks[i] = ec
	}

	// Interleave EC codewords into result.
	ecStart := symbolInfo.DataCapacity
	for i := 0; i < ecPerBlock; i++ {
		for j := 0; j < blockCount; j++ {
			result[ecStart] = ecBlocks[j][i]
			ecStart++
		}
	}

	return result, nil
}

// generateECCBlock generates Reed-Solomon error correction codewords for a
// single data block using GF(256) with primitive polynomial 0x12D.
func generateECCBlock(data []byte, numECCodewords int) ([]byte, error) {
	rsEncoder := reedsolomon.NewEncoder(reedsolomon.DataMatrixField256)

	// The reedsolomon.Encoder.Encode method operates on []int with space for data + EC.
	toEncode := make([]int, len(data)+numECCodewords)
	for i, b := range data {
		toEncode[i] = int(b)
	}

	rsEncoder.Encode(toEncode, numECCodewords)

	// Extract the EC codewords.
	ec := make([]byte, numECCodewords)
	for i := 0; i < numECCodewords; i++ {
		ec[i] = byte(toEncode[len(data)+i])
	}
	return ec, nil
}
