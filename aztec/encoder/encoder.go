package encoder

import (
	"fmt"

	"github.com/ericlevine/zxinggo/bitutil"
	"github.com/ericlevine/zxinggo/reedsolomon"
)

// AztecCode holds the result of encoding data into an Aztec barcode.
type AztecCode struct {
	Matrix    *bitutil.BitMatrix
	Compact   bool
	Size      int
	Layers    int
	CodeWords int
}

// WORD_SIZE[layers] gives the codeword size for that layer count.
// Index 0 is for the mode message (4 bits). Indices 1-32 for data layers.
var wordSizeTable = [33]int{
	4, 6, 6, 8, 8, 8, 8, 8, 8, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10,
	12, 12, 12, 12, 12, 12, 12, 12, 12, 12,
}

// gfForWordSize returns the Galois Field for the given codeword bit width.
func gfForWordSize(ws int) *reedsolomon.GenericGF {
	switch ws {
	case 4:
		return reedsolomon.AztecParam
	case 6:
		return reedsolomon.AztecData6
	case 8:
		return reedsolomon.AztecData8
	case 10:
		return reedsolomon.AztecData10
	case 12:
		return reedsolomon.AztecData12
	default:
		panic(fmt.Sprintf("aztec: unsupported word size %d", ws))
	}
}

// Encode encodes the given data into an Aztec barcode symbol.
func Encode(data []byte, minECCPercent int, userSpecifiedLayers int) (*AztecCode, error) {
	// 1. High-level encode the data into a bit stream.
	bits, err := highLevelEncode(data)
	if err != nil {
		return nil, err
	}

	// 2. Choose symbol size.
	eccBits := bits.Size()*minECCPercent/100 + 11
	totalSizeBits := bits.Size() + eccBits

	var compact bool
	var layers int
	var totalBitsInLayer int
	var wordSize int
	var stuffedBits *bitutil.BitArray

	if userSpecifiedLayers != 0 {
		compact = userSpecifiedLayers < 0
		layers = userSpecifiedLayers
		if compact {
			layers = -layers
		}
		maxLayers := 32
		if compact {
			maxLayers = 4
		}
		if layers < 1 || layers > maxLayers {
			return nil, fmt.Errorf("aztec: illegal layer value %d", userSpecifiedLayers)
		}
		totalBitsInLayer = totalBitsInLayerFn(layers, compact)
		wordSize = wordSizeTable[layers]
		usableBits := totalBitsInLayer - (totalBitsInLayer % wordSize)
		stuffedBits = stuffBits(bits, wordSize)
		if stuffedBits.Size()+eccBits > usableBits {
			return nil, fmt.Errorf("aztec: data too large for user specified layer")
		}
		if compact && stuffedBits.Size() > wordSize*64 {
			return nil, fmt.Errorf("aztec: data too large for user specified layer")
		}
	} else {
		// Auto: try Compact1-4, then Normal4-32.
		// (Normal1-3 are skipped because Compact(i+1) is the same size but has more data.)
		found := false
		for i := 0; i <= 32; i++ {
			compact = i <= 3
			if compact {
				layers = i + 1
			} else {
				layers = i
			}
			totalBitsInLayer = totalBitsInLayerFn(layers, compact)
			if totalSizeBits > totalBitsInLayer {
				continue
			}
			if stuffedBits == nil || wordSize != wordSizeTable[layers] {
				wordSize = wordSizeTable[layers]
				stuffedBits = stuffBits(bits, wordSize)
			}
			usableBits := totalBitsInLayer - (totalBitsInLayer % wordSize)
			if compact && stuffedBits.Size() > wordSize*64 {
				continue
			}
			if stuffedBits.Size()+eccBits <= usableBits {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("aztec: data too large for any Aztec symbol")
		}
	}

	// 3. Generate check words (RS encode data + ECC, return as bit stream with padding).
	messageBits := generateCheckWords(stuffedBits, totalBitsInLayer, wordSize)

	// 4. Generate mode message.
	messageSizeInWords := stuffedBits.Size() / wordSize
	modeMessage := generateModeMessage(compact, layers, messageSizeInWords)

	// 5. Allocate symbol and build alignment map.
	baseMatrixSize := layers*4 + 11
	if !compact {
		baseMatrixSize = layers*4 + 14
	}
	alignmentMap := make([]int, baseMatrixSize)
	var matrixSize int

	if compact {
		matrixSize = baseMatrixSize
		for i := 0; i < baseMatrixSize; i++ {
			alignmentMap[i] = i
		}
	} else {
		matrixSize = baseMatrixSize + 1 + 2*((baseMatrixSize/2-1)/15)
		origCenter := baseMatrixSize / 2
		center := matrixSize / 2
		for i := 0; i < origCenter; i++ {
			newOffset := i + i/15
			alignmentMap[origCenter-i-1] = center - newOffset - 1
			alignmentMap[origCenter+i] = center + newOffset + 1
		}
	}

	matrix := bitutil.NewBitMatrix(matrixSize)

	// 6. Draw data bits.
	rowOffset := 0
	for i := 0; i < layers; i++ {
		rowSize := (layers-i)*4 + 9
		if !compact {
			rowSize = (layers-i)*4 + 12
		}
		for j := 0; j < rowSize; j++ {
			columnOffset := j * 2
			for k := 0; k < 2; k++ {
				if messageBits.Get(rowOffset + columnOffset + k) {
					matrix.Set(alignmentMap[i*2+k], alignmentMap[i*2+j])
				}
				if messageBits.Get(rowOffset + rowSize*2 + columnOffset + k) {
					matrix.Set(alignmentMap[i*2+j], alignmentMap[baseMatrixSize-1-i*2-k])
				}
				if messageBits.Get(rowOffset + rowSize*4 + columnOffset + k) {
					matrix.Set(alignmentMap[baseMatrixSize-1-i*2-k], alignmentMap[baseMatrixSize-1-i*2-j])
				}
				if messageBits.Get(rowOffset + rowSize*6 + columnOffset + k) {
					matrix.Set(alignmentMap[baseMatrixSize-1-i*2-j], alignmentMap[i*2+k])
				}
			}
		}
		rowOffset += rowSize * 8
	}

	// 7. Draw mode message.
	drawModeMessage(matrix, compact, matrixSize, modeMessage)

	// 8. Draw alignment marks.
	if compact {
		drawBullsEye(matrix, matrixSize/2, 5)
	} else {
		drawBullsEye(matrix, matrixSize/2, 7)
		for i, j := 0, 0; i < baseMatrixSize/2-1; i, j = i+15, j+16 {
			for k := (matrixSize / 2) & 1; k < matrixSize; k += 2 {
				matrix.Set(matrixSize/2-j, k)
				matrix.Set(matrixSize/2+j, k)
				matrix.Set(k, matrixSize/2-j)
				matrix.Set(k, matrixSize/2+j)
			}
		}
	}

	return &AztecCode{
		Matrix:    matrix,
		Compact:   compact,
		Size:      matrixSize,
		Layers:    layers,
		CodeWords: messageSizeInWords,
	}, nil
}

func totalBitsInLayerFn(layers int, compact bool) int {
	base := 112
	if compact {
		base = 88
	}
	return (base + 16*layers) * layers
}

// stuffBits processes the data bit stream, inserting stuff bits to prevent
// all-zero or all-one codewords. Matches the Java ZXing Encoder.stuffBits.
func stuffBits(bits *bitutil.BitArray, wordSize int) *bitutil.BitArray {
	out := bitutil.NewBitArray(0)
	n := bits.Size()
	mask := (1 << uint(wordSize)) - 2 // all bits except LSB

	for i := 0; i < n; i += wordSize {
		word := 0
		for j := 0; j < wordSize; j++ {
			if i+j >= n || bits.Get(i+j) {
				word |= 1 << uint(wordSize-1-j)
			}
		}
		if (word & mask) == mask {
			// Upper bits are all 1 -> stuff: write upper bits (LSB=0), back up 1
			out.AppendBits(uint32(word&mask), wordSize)
			i-- // net effect with loop increment: advance wordSize-1 bits
		} else if (word & mask) == 0 {
			// Upper bits are all 0 -> stuff: write with LSB=1, back up 1
			out.AppendBits(uint32(word|1), wordSize)
			i--
		} else {
			out.AppendBits(uint32(word), wordSize)
		}
	}
	return out
}

// generateCheckWords applies Reed-Solomon encoding to the stuffed bits,
// producing a bit stream of exactly totalBits length (with leading padding).
func generateCheckWords(stuffedBits *bitutil.BitArray, totalBits, wordSize int) *bitutil.BitArray {
	messageSizeInWords := stuffedBits.Size() / wordSize
	totalWords := totalBits / wordSize

	messageWords := bitsToWords(stuffedBits, wordSize, totalWords)

	rs := reedsolomon.NewEncoder(gfForWordSize(wordSize))
	rs.Encode(messageWords, totalWords-messageSizeInWords)

	startPad := totalBits % wordSize
	out := bitutil.NewBitArray(0)
	out.AppendBits(0, startPad)
	for _, w := range messageWords {
		out.AppendBits(uint32(w), wordSize)
	}
	return out
}

func bitsToWords(stuffedBits *bitutil.BitArray, wordSize, totalWords int) []int {
	message := make([]int, totalWords)
	n := stuffedBits.Size() / wordSize
	for i := 0; i < n; i++ {
		value := 0
		for j := 0; j < wordSize; j++ {
			if stuffedBits.Get(i*wordSize + j) {
				value |= 1 << uint(wordSize-1-j)
			}
		}
		message[i] = value
	}
	return message
}

// generateModeMessage creates the mode message bits.
func generateModeMessage(compact bool, layers, messageSizeInWords int) *bitutil.BitArray {
	modeMessage := bitutil.NewBitArray(0)
	if compact {
		modeMessage.AppendBits(uint32(layers-1), 2)
		modeMessage.AppendBits(uint32(messageSizeInWords-1), 6)
		return generateCheckWords(modeMessage, 28, 4)
	}
	modeMessage.AppendBits(uint32(layers-1), 5)
	modeMessage.AppendBits(uint32(messageSizeInWords-1), 11)
	return generateCheckWords(modeMessage, 40, 4)
}

// drawBullsEye draws the concentric finder rings and orientation marks.
func drawBullsEye(matrix *bitutil.BitMatrix, center, size int) {
	for i := 0; i < size; i += 2 {
		for j := center - i; j <= center+i; j++ {
			matrix.Set(j, center-i)
			matrix.Set(j, center+i)
			matrix.Set(center-i, j)
			matrix.Set(center+i, j)
		}
	}
	// Orientation marks
	matrix.Set(center-size, center-size)
	matrix.Set(center-size+1, center-size)
	matrix.Set(center-size, center-size+1)
	matrix.Set(center+size, center-size)
	matrix.Set(center+size, center-size+1)
	matrix.Set(center+size, center+size-1)
}

// drawModeMessage places the mode message bits around the bullseye.
func drawModeMessage(matrix *bitutil.BitMatrix, compact bool, matrixSize int, modeMessage *bitutil.BitArray) {
	center := matrixSize / 2
	if compact {
		for i := 0; i < 7; i++ {
			offset := center - 3 + i
			if modeMessage.Get(i) {
				matrix.Set(offset, center-5)
			}
			if modeMessage.Get(i + 7) {
				matrix.Set(center+5, offset)
			}
			if modeMessage.Get(20 - i) {
				matrix.Set(offset, center+5)
			}
			if modeMessage.Get(27 - i) {
				matrix.Set(center-5, offset)
			}
		}
	} else {
		for i := 0; i < 10; i++ {
			offset := center - 5 + i + i/5
			if modeMessage.Get(i) {
				matrix.Set(offset, center-7)
			}
			if modeMessage.Get(i + 10) {
				matrix.Set(center+7, offset)
			}
			if modeMessage.Get(29 - i) {
				matrix.Set(offset, center+7)
			}
			if modeMessage.Get(39 - i) {
				matrix.Set(center-7, offset)
			}
		}
	}
}
