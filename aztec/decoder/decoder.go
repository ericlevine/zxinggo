// Package decoder implements the Aztec barcode decoder.
//
// It takes a BitMatrix (the sampled grid from the detector) along with
// structural parameters (compact mode, layer count, data-block count)
// and produces the decoded text.
//
// The algorithm follows the ZXing Java reference implementation:
//  1. Extract raw bits from the concentric data layers.
//  2. Correct errors using Reed-Solomon over the appropriate Galois Field.
//  3. Extract the data bits from the corrected codewords.
//  4. Decode the resulting bit stream using the Aztec 5-mode encoding tables.
package decoder

import (
	"strings"
	"unicode/utf8"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
	"github.com/ericlevine/zxinggo/reedsolomon"
)

// ---------------------------------------------------------------------------
// Result types
// ---------------------------------------------------------------------------

// AztecDetectorResult carries the output of the Aztec detector that the
// decoder needs: the sampled bit matrix, the corner/center result points,
// and the structural parameters read from the mode message.
type AztecDetectorResult struct {
	Bits         *bitutil.BitMatrix
	Points       []zxinggo.ResultPoint
	Compact      bool
	NbDataBlocks int
	NbLayers     int
}

// DecoderResult holds the final decoded text and raw bytes.
type DecoderResult struct {
	Text     string
	RawBytes []byte
}

// ---------------------------------------------------------------------------
// Encoding-mode constants
// ---------------------------------------------------------------------------

const (
	modeUpper = iota
	modeLower
	modeMixed
	modeDigit
	modePunct
)

// Character tables -- indexed by the codeword value inside each mode.
var upperTable = [32]rune{
	0, ' ', 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M',
	'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z', 0, 0, 0, 0,
}

var lowerTable = [32]rune{
	0, ' ', 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm',
	'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', 0, 0, 0, 0,
}

var mixedTable = [32]rune{
	0, ' ', '\x01', '\x02', '\x03', '\x04', '\x05', '\x06', '\x07', '\b', '\t', '\n',
	'\x0b', '\f', '\r', '\x1b', '\x1c', '\x1d', '\x1e', '\x1f',
	'@', '\\', '^', '_', '`', '|', '~', '\x7f', 0, 0, 0, 0,
}

// punctTable maps codeword values to strings. Matches Java ZXing PUNCT_TABLE.
// Index 0 = FLG(n) handled specially. Index 31 = CTRL_UL handled specially.
var punctTable = [32]string{
	"", "\r", "\r\n", ". ", ", ", ": ", "!", "\"", "#", "$", "%", "&", "'", "(", ")",
	"*", "+", ",", "-", ".", "/", ":", ";", "<", "=", ">", "?", "[", "]", "{", "}", "",
}

// ---------------------------------------------------------------------------
// Public entry point
// ---------------------------------------------------------------------------

// Decode decodes an Aztec symbol described by the given detector result.
func Decode(detectorResult *AztecDetectorResult) (*DecoderResult, error) {
	rawbits := extractBits(detectorResult)

	correctedBits, err := correctBits(detectorResult, rawbits)
	if err != nil {
		return nil, err
	}

	text, rawBytes, err := getEncodedData(correctedBits)
	if err != nil {
		return nil, err
	}

	return &DecoderResult{
		Text:     text,
		RawBytes: rawBytes,
	}, nil
}

// ---------------------------------------------------------------------------
// Reed-Solomon error correction
// ---------------------------------------------------------------------------

// codewordSize returns the number of bits per codeword for the symbol.
func codewordSize(nbLayers int) int {
	if nbLayers <= 2 {
		return 6
	}
	if nbLayers <= 8 {
		return 8
	}
	if nbLayers <= 22 {
		return 10
	}
	return 12
}

func totalBitsInLayer(layers int, compact bool) int {
	base := 112
	if compact {
		base = 88
	}
	return (base + 16*layers) * layers
}

// correctBits applies Reed-Solomon error correction to the raw bit stream
// and unstuffs the data codewords. Matches Java ZXing Decoder.correctBits.
func correctBits(ddata *AztecDetectorResult, rawbits []bool) ([]bool, error) {
	nbLayers := ddata.NbLayers
	nbDataBlocks := ddata.NbDataBlocks

	cwSize := codewordSize(nbLayers)
	numCodewords := len(rawbits) / cwSize

	if nbDataBlocks > numCodewords {
		return nil, zxinggo.ErrFormat
	}

	offset := len(rawbits) % cwSize
	numDataCodewords := nbDataBlocks
	numECCodewords := numCodewords - numDataCodewords

	// Convert raw bits into codeword integers (MSB first, starting after offset).
	dataWords := make([]int, numCodewords)
	for i := 0; i < numCodewords; i++ {
		w := 0
		for j := 0; j < cwSize; j++ {
			w <<= 1
			if rawbits[offset+i*cwSize+j] {
				w |= 1
			}
		}
		dataWords[i] = w
	}

	// Reed-Solomon decode.
	var gf *reedsolomon.GenericGF
	switch cwSize {
	case 6:
		gf = reedsolomon.AztecData6
	case 8:
		gf = reedsolomon.AztecData8
	case 10:
		gf = reedsolomon.AztecData10
	case 12:
		gf = reedsolomon.AztecData12
	default:
		return nil, zxinggo.ErrFormat
	}

	rsDecoder := reedsolomon.NewDecoder(gf)
	_, err := rsDecoder.Decode(dataWords, numECCodewords)
	if err != nil {
		return nil, zxinggo.ErrChecksum
	}

	// Unstuff the corrected data codewords.
	// A codeword of all-zeros or all-ones is illegal (should not appear after stuffing).
	// A codeword of value 1 (0...01) means cwSize-1 zero bits.
	// A codeword of value mask-1 (1...10) means cwSize-1 one bits.
	// All other codewords contribute all cwSize bits unchanged.
	mask := (1 << uint(cwSize)) - 1
	stuffedCount := 0
	for i := 0; i < numDataCodewords; i++ {
		w := dataWords[i]
		if w == 0 || w == mask {
			return nil, zxinggo.ErrFormat
		}
		if w == 1 || w == mask-1 {
			stuffedCount++
		}
	}

	correctedBits := make([]bool, numDataCodewords*cwSize-stuffedCount)
	idx := 0
	for i := 0; i < numDataCodewords; i++ {
		w := dataWords[i]
		if w == 1 || w == mask-1 {
			// Stuffed codeword: output cwSize-1 identical bits.
			fill := w > 1 // true for mask-1 (all ones)
			for j := 0; j < cwSize-1; j++ {
				correctedBits[idx] = fill
				idx++
			}
		} else {
			// Normal codeword: output all cwSize bits.
			for bit := cwSize - 1; bit >= 0; bit-- {
				correctedBits[idx] = (w & (1 << uint(bit))) != 0
				idx++
			}
		}
	}

	return correctedBits, nil
}

// ---------------------------------------------------------------------------
// Bit stream decoding (Aztec multi-mode encoding)
// ---------------------------------------------------------------------------

// getEncodedData decodes the corrected data-bit stream into text using the
// Aztec five-mode encoding scheme.
func getEncodedData(correctedBits []bool) (string, []byte, error) {
	endIndex := len(correctedBits)
	currentMode := modeUpper
	index := 0

	var result strings.Builder
	var rawBytes []byte

	for index < endIndex {
		if currentMode == modeDigit {
			index, currentMode = decodeDigit(&result, correctedBits, index, endIndex)
		} else {
			index, currentMode = decodeNonDigit(&result, correctedBits, index, endIndex, currentMode)
		}
		if index < 0 {
			return "", nil, zxinggo.ErrFormat
		}
	}

	text := result.String()
	if utf8.ValidString(text) {
		rawBytes = []byte(text)
	}

	return text, rawBytes, nil
}

// readCode reads bitsToRead bits starting at index from the corrected bit
// stream and returns the integer value (MSB first) together with the new index.
func readCode(correctedBits []bool, index, bitsToRead, endIndex int) (int, int) {
	if index+bitsToRead > endIndex {
		return -1, endIndex
	}
	code := 0
	for i := index; i < index+bitsToRead; i++ {
		code <<= 1
		if correctedBits[i] {
			code |= 1
		}
	}
	return code, index + bitsToRead
}

// decodeNonDigit handles UPPER, LOWER, MIXED and PUNCT modes (all 5-bit).
func decodeNonDigit(result *strings.Builder, bits []bool, index, endIndex, mode int) (int, int) {
	code, newIndex := readCode(bits, index, 5, endIndex)
	if code < 0 {
		return endIndex, mode
	}
	index = newIndex

	// FLG(n) is code 0 in every non-digit mode.
	if code == 0 {
		return handleFLG(result, bits, index, endIndex, mode)
	}

	switch mode {
	case modeUpper:
		switch {
		case code >= 1 && code <= 27:
			result.WriteRune(upperTable[code])
		case code == 28:
			return index, modeLower
		case code == 29:
			return index, modeMixed
		case code == 30:
			return index, modeDigit
		case code == 31:
			return handleBinaryShift(result, bits, index, endIndex, mode)
		}

	case modeLower:
		switch {
		case code >= 1 && code <= 27:
			result.WriteRune(lowerTable[code])
		case code == 28:
			return decodeOneCharShift(result, bits, index, endIndex, modeLower, modeUpper)
		case code == 29:
			return index, modeMixed
		case code == 30:
			return index, modeDigit
		case code == 31:
			return handleBinaryShift(result, bits, index, endIndex, mode)
		}

	case modeMixed:
		switch {
		case code >= 1 && code <= 27:
			result.WriteRune(mixedTable[code])
		case code == 28:
			return index, modePunct
		case code == 29:
			return index, modeUpper
		case code == 30:
			return decodeOneCharShift(result, bits, index, endIndex, modeMixed, modePunct)
		case code == 31:
			return handleBinaryShift(result, bits, index, endIndex, mode)
		}

	case modePunct:
		switch {
		case code >= 1 && code <= 30:
			result.WriteString(punctTable[code])
		case code == 31:
			return index, modeUpper
		}
	}

	return index, mode
}

// decodeDigit handles DIGIT mode (4-bit codewords).
func decodeDigit(result *strings.Builder, bits []bool, index, endIndex int) (int, int) {
	code, newIndex := readCode(bits, index, 4, endIndex)
	if code < 0 {
		return endIndex, modeDigit
	}
	index = newIndex

	switch {
	case code == 0:
		return handleFLG(result, bits, index, endIndex, modeDigit)
	case code == 1:
		return decodeOneCharShift(result, bits, index, endIndex, modeDigit, modePunct)
	case code >= 2 && code <= 11:
		result.WriteByte(byte('0' + code - 2))
	case code == 12:
		result.WriteByte(',')
	case code == 13:
		result.WriteByte('.')
	case code == 14:
		return index, modeUpper
	case code == 15:
		return decodeOneCharShift(result, bits, index, endIndex, modeDigit, modeUpper)
	}

	return index, modeDigit
}

// decodeOneCharShift reads exactly one character in the target mode and
// returns to the originating mode.
func decodeOneCharShift(result *strings.Builder, bits []bool, index, endIndex, returnMode, shiftMode int) (int, int) {
	if shiftMode == modeDigit {
		code, newIndex := readCode(bits, index, 4, endIndex)
		if code < 0 {
			return endIndex, returnMode
		}
		index = newIndex
		switch {
		case code >= 2 && code <= 11:
			result.WriteByte(byte('0' + code - 2))
		case code == 12:
			result.WriteByte(',')
		case code == 13:
			result.WriteByte('.')
		}
		return index, returnMode
	}

	code, newIndex := readCode(bits, index, 5, endIndex)
	if code < 0 {
		return endIndex, returnMode
	}
	index = newIndex

	switch shiftMode {
	case modeUpper:
		if code >= 1 && code <= 27 {
			result.WriteRune(upperTable[code])
		}
	case modeLower:
		if code >= 1 && code <= 27 {
			result.WriteRune(lowerTable[code])
		}
	case modeMixed:
		if code >= 1 && code <= 27 {
			result.WriteRune(mixedTable[code])
		}
	case modePunct:
		if code >= 1 && code <= 30 {
			result.WriteString(punctTable[code])
		}
	}

	return index, returnMode
}

// handleFLG processes the FLG(n) function.
func handleFLG(result *strings.Builder, bits []bool, index, endIndex, mode int) (int, int) {
	n, newIndex := readCode(bits, index, 3, endIndex)
	if n < 0 {
		return endIndex, mode
	}
	index = newIndex

	switch {
	case n == 0:
		result.WriteByte(0x1D) // FNC1 -> GS
	case n >= 1 && n <= 4:
		// ECI: read n 4-bit digit codes
		for i := 0; i < n; i++ {
			_, index = readCode(bits, index, 4, endIndex)
		}
	case n == 7:
		// Reserved, technically invalid
	}

	return index, mode
}

// handleBinaryShift reads a binary-shift length and then that many raw bytes.
func handleBinaryShift(result *strings.Builder, bits []bool, index, endIndex, mode int) (int, int) {
	length, newIndex := readCode(bits, index, 5, endIndex)
	if length < 0 {
		return endIndex, mode
	}
	index = newIndex

	if length == 0 {
		extra, newIndex2 := readCode(bits, index, 11, endIndex)
		if extra < 0 {
			return endIndex, mode
		}
		index = newIndex2
		length = extra + 31
	}

	for i := 0; i < length; i++ {
		ch, newIdx := readCode(bits, index, 8, endIndex)
		if ch < 0 {
			return endIndex, mode
		}
		index = newIdx
		result.WriteByte(byte(ch))
	}

	return index, mode
}

// ---------------------------------------------------------------------------
// Bit extraction from the Aztec symbol matrix
// ---------------------------------------------------------------------------

// extractBits reads all data modules from the symbol matrix in the correct
// order. Matches Java ZXing Decoder.extractBits exactly.
//
// Layers are read from outermost (i=0, largest rowSize) to innermost.
// Each layer has 4 sides, each side has rowSize 2-module positions.
func extractBits(ddata *AztecDetectorResult) []bool {
	compact := ddata.Compact
	layers := ddata.NbLayers
	matrix := ddata.Bits

	baseMatrixSize := layers*4 + 11
	if !compact {
		baseMatrixSize = layers*4 + 14
	}

	// Build alignment map (same construction as encoder).
	alignmentMap := make([]int, baseMatrixSize)
	if compact {
		for i := 0; i < baseMatrixSize; i++ {
			alignmentMap[i] = i
		}
	} else {
		matrixSize := baseMatrixSize + 1 + 2*((baseMatrixSize/2-1)/15)
		origCenter := baseMatrixSize / 2
		center := matrixSize / 2
		for i := 0; i < origCenter; i++ {
			newOffset := i + i/15
			alignmentMap[origCenter-i-1] = center - newOffset - 1
			alignmentMap[origCenter+i] = center + newOffset + 1
		}
	}

	totalBits := totalBitsInLayer(layers, compact)
	rawbits := make([]bool, totalBits)

	rowOffset := 0
	for i := 0; i < layers; i++ {
		rowSize := (layers-i)*4 + 9
		if !compact {
			rowSize = (layers-i)*4 + 12
		}
		low := i * 2
		high := baseMatrixSize - 1 - low

		for j := 0; j < rowSize; j++ {
			columnOffset := j * 2
			for k := 0; k < 2; k++ {
				// left column
				rawbits[rowOffset+columnOffset+k] =
					readModule(matrix, alignmentMap, low+k, low+j)
				// bottom row
				rawbits[rowOffset+2*rowSize+columnOffset+k] =
					readModule(matrix, alignmentMap, low+j, high-k)
				// right column
				rawbits[rowOffset+4*rowSize+columnOffset+k] =
					readModule(matrix, alignmentMap, high-k, high-j)
				// top row
				rawbits[rowOffset+6*rowSize+columnOffset+k] =
					readModule(matrix, alignmentMap, high-j, low+k)
			}
		}
		rowOffset += rowSize * 8
	}

	return rawbits
}

// readModule reads a single module from the matrix using the alignment map.
// The x,y args are abstract coordinates; alignmentMap maps them to real coords.
// In BitMatrix, Get(x, y) expects x=column, y=row.
func readModule(matrix *bitutil.BitMatrix, alignmentMap []int, x, y int) bool {
	if x < 0 || x >= len(alignmentMap) || y < 0 || y >= len(alignmentMap) {
		return false
	}
	mx := alignmentMap[x]
	my := alignmentMap[y]
	if mx < 0 || mx >= matrix.Width() || my < 0 || my >= matrix.Height() {
		return false
	}
	return matrix.Get(mx, my)
}
