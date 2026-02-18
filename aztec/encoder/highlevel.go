// Package encoder implements Aztec barcode encoding.
package encoder

import (
	"fmt"

	"github.com/ericlevine/zxinggo/bitutil"
)

// Encoding modes for the Aztec high-level encoder.
const (
	modeUpper = iota
	modeLower
	modeMixed
	modeDigit
	modePunct
)

// Number of bits per code in each mode (DIGIT is 4, all others are 5).
var modeBits = [5]int{5, 5, 5, 4, 5}

// charMap maps each byte value to its code in each of the five modes.
// A value of -1 means the character cannot be encoded in that mode.
var charMap [256][5]int

func init() {
	for i := range charMap {
		for j := range charMap[i] {
			charMap[i][j] = -1
		}
	}

	// UPPER (5 bits per code):
	//   0 = FLG(n), 1 = SP, 2..27 = A..Z, 28 = LL, 29 = ML, 30 = DL, 31 = BS
	charMap[' '][modeUpper] = 1
	for c := byte('A'); c <= 'Z'; c++ {
		charMap[c][modeUpper] = int(c-'A') + 2
	}

	// LOWER (5 bits per code):
	//   0 = FLG(n), 1 = SP, 2..27 = a..z, 28 = AS, 29 = ML, 30 = DL, 31 = BS
	charMap[' '][modeLower] = 1
	for c := byte('a'); c <= 'z'; c++ {
		charMap[c][modeLower] = int(c-'a') + 2
	}

	// MIXED (5 bits per code):
	//   0 = FLG(n), 1 = SP, 2..14 = ctrl \x01..\x0D,
	//   15 = \x1B (ESC), 16..19 = \x1C..\x1F (FS/GS/RS/US),
	//   20 = @, 21 = \, 22 = ^, 23 = _, 24 = `, 25 = |, 26 = ~, 27 = \x7F (DEL),
	//   28 = PL, 29 = UL, 30 = (reserved), 31 = BS
	charMap[' '][modeMixed] = 1
	for c := byte(1); c <= 13; c++ {
		charMap[c][modeMixed] = int(c) + 1 // codes 2..14
	}
	charMap[0x1B][modeMixed] = 15
	charMap[0x1C][modeMixed] = 16
	charMap[0x1D][modeMixed] = 17
	charMap[0x1E][modeMixed] = 18
	charMap[0x1F][modeMixed] = 19
	charMap['@'][modeMixed] = 20
	charMap['\\'][modeMixed] = 21
	charMap['^'][modeMixed] = 22
	charMap['_'][modeMixed] = 23
	charMap['`'][modeMixed] = 24
	charMap['|'][modeMixed] = 25
	charMap['~'][modeMixed] = 26
	charMap[0x7F][modeMixed] = 27

	// DIGIT (4 bits per code):
	//   0 = FLG(n), 1 = SP, 2..11 = '0'..'9', 12 = ',', 13 = '.', 14 = UL, 15 = AS
	charMap[' '][modeDigit] = 1
	for c := byte('0'); c <= '9'; c++ {
		charMap[c][modeDigit] = int(c-'0') + 2
	}
	charMap[','][modeDigit] = 12
	charMap['.'][modeDigit] = 13

	// PUNCT (5 bits per code):
	//   0 = FLG(n),
	//   1 = \r, 2 = \r\n, 3 = ". ", 4 = ", ", 5 = ": ",
	//   6..29 = ! " # $ % & ' ( ) * + , - . / : ; < = > ? [ ] {
	//   30 = }, 31 = UL
	charMap['\r'][modePunct] = 1
	// Codes 2..5 are two-char sequences, handled separately.
	singlePunct := []byte{
		'!', '"', '#', '$', '%', '&', '\'', '(', ')', '*', '+', ',',
		'-', '.', '/', ':', ';', '<', '=', '>', '?', '[', ']', '{',
	}
	for idx, c := range singlePunct {
		charMap[c][modePunct] = idx + 6
	}
	charMap['}'][modePunct] = 30
}

// punctPairs maps two-character sequences to their PUNCT mode codes.
var punctPairs = map[[2]byte]int{
	{'\r', '\n'}: 2,
	{'.', ' '}:   3,
	{',', ' '}:   4,
	{':', ' '}:   5,
}

// modeSwitch describes one step of a latch/shift sequence: emit the given
// code using the bit width of intermediateMode.
type modeSwitch struct {
	intermediateMode int
	code             int
}

// getLatchSequence returns the sequence of codes to latch from one mode to
// another. Each entry specifies the current mode and the code to emit.
func getLatchSequence(from, to int) []modeSwitch {
	if from == to {
		return nil
	}
	switch from {
	case modeUpper:
		switch to {
		case modeLower:
			return []modeSwitch{{modeUpper, 28}} // LL
		case modeMixed:
			return []modeSwitch{{modeUpper, 29}} // ML
		case modeDigit:
			return []modeSwitch{{modeUpper, 30}} // DL
		case modePunct:
			return []modeSwitch{{modeUpper, 29}, {modeMixed, 28}} // ML, PL
		}
	case modeLower:
		switch to {
		case modeUpper:
			return []modeSwitch{{modeLower, 29}, {modeMixed, 29}} // ML, UL
		case modeMixed:
			return []modeSwitch{{modeLower, 29}} // ML
		case modeDigit:
			return []modeSwitch{{modeLower, 30}} // DL
		case modePunct:
			return []modeSwitch{{modeLower, 29}, {modeMixed, 28}} // ML, PL
		}
	case modeMixed:
		switch to {
		case modeUpper:
			return []modeSwitch{{modeMixed, 29}} // UL
		case modeLower:
			return []modeSwitch{{modeMixed, 29}, {modeUpper, 28}} // UL, LL
		case modeDigit:
			return []modeSwitch{{modeMixed, 29}, {modeUpper, 30}} // UL, DL
		case modePunct:
			return []modeSwitch{{modeMixed, 28}} // PL
		}
	case modeDigit:
		switch to {
		case modeUpper:
			return []modeSwitch{{modeDigit, 14}} // UL
		case modeLower:
			return []modeSwitch{{modeDigit, 14}, {modeUpper, 28}} // UL, LL
		case modeMixed:
			return []modeSwitch{{modeDigit, 14}, {modeUpper, 29}} // UL, ML
		case modePunct:
			return []modeSwitch{{modeDigit, 14}, {modeUpper, 29}, {modeMixed, 28}} // UL, ML, PL
		}
	case modePunct:
		switch to {
		case modeUpper:
			return []modeSwitch{{modePunct, 31}} // UL
		case modeLower:
			return []modeSwitch{{modePunct, 31}, {modeUpper, 28}} // UL, LL
		case modeMixed:
			return []modeSwitch{{modePunct, 31}, {modeUpper, 29}} // UL, ML
		case modeDigit:
			return []modeSwitch{{modePunct, 31}, {modeUpper, 30}} // UL, DL
		}
	}
	return nil
}

// highLevelEncode encodes data bytes into a BitArray using the Aztec
// high-level encoding scheme. It uses a greedy strategy starting in UPPER
// mode.
func highLevelEncode(data []byte) (*bitutil.BitArray, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("aztec: empty input")
	}

	result := bitutil.NewBitArray(0)
	curMode := modeUpper

	i := 0
	for i < len(data) {
		// Check for two-character PUNCT pairs.
		if i+1 < len(data) {
			pair := [2]byte{data[i], data[i+1]}
			if pCode, ok := punctPairs[pair]; ok {
				if curMode == modePunct {
					result.AppendBits(uint32(pCode), modeBits[modePunct])
				} else {
					// Latch to PUNCT, emit the pair, then the next
					// iteration will latch back out if needed.
					seq := getLatchSequence(curMode, modePunct)
					for _, sw := range seq {
						result.AppendBits(uint32(sw.code), modeBits[sw.intermediateMode])
					}
					curMode = modePunct
					result.AppendBits(uint32(pCode), modeBits[modePunct])
				}
				i += 2
				continue
			}
		}

		b := data[i]

		// If encodable in the current mode, emit directly.
		if charMap[b][curMode] != -1 {
			result.AppendBits(uint32(charMap[b][curMode]), modeBits[curMode])
			i++
			continue
		}

		// Find the best mode for this character.
		newMode := findBestMode(b, curMode)
		if newMode == -1 {
			// No character mode can encode this byte; use binary shift.
			// Binary shift is available from UPPER, LOWER, and MIXED (code 31).
			// It is not available from DIGIT or PUNCT, so latch out first.
			if curMode == modeDigit {
				result.AppendBits(14, modeBits[modeDigit]) // UL
				curMode = modeUpper
			} else if curMode == modePunct {
				result.AppendBits(31, modeBits[modePunct]) // UL
				curMode = modeUpper
			}
			i = emitBinaryShift(result, data, i, curMode)
			continue
		}

		// Decide whether to use a shift or a latch.
		if canShift(curMode, newMode) && shouldShift(data, i, curMode) {
			emitShiftCode(result, curMode, newMode)
			result.AppendBits(uint32(charMap[b][newMode]), modeBits[newMode])
			// curMode remains unchanged after a shift.
		} else {
			seq := getLatchSequence(curMode, newMode)
			for _, sw := range seq {
				result.AppendBits(uint32(sw.code), modeBits[sw.intermediateMode])
			}
			curMode = newMode
			result.AppendBits(uint32(charMap[b][curMode]), modeBits[curMode])
		}
		i++
	}

	return result, nil
}

// findBestMode returns the best mode to encode byte b when currently in
// curMode, or -1 if no character mode can encode it (binary shift required).
func findBestMode(b byte, curMode int) int {
	if charMap[b][curMode] != -1 {
		return curMode
	}
	// Preference order: try modes requiring fewer latch codes first.
	preferenceOrders := [5][]int{
		{modeLower, modeMixed, modeDigit, modePunct}, // from UPPER
		{modeDigit, modeMixed, modeUpper, modePunct}, // from LOWER
		{modeUpper, modePunct, modeLower, modeDigit}, // from MIXED
		{modeUpper, modeLower, modeMixed, modePunct},  // from DIGIT
		{modeUpper, modeLower, modeMixed, modeDigit},  // from PUNCT
	}
	for _, m := range preferenceOrders[curMode] {
		if charMap[b][m] != -1 {
			return m
		}
	}
	return -1
}

// canShift returns whether a single-character shift from curMode to newMode
// is available. Aztec defines only two shift types:
//   - AS (Alpha Shift to UPPER) from LOWER (code 28) and DIGIT (code 15).
func canShift(curMode, newMode int) bool {
	if newMode != modeUpper {
		return false
	}
	return curMode == modeLower || curMode == modeDigit
}

// shouldShift returns true if a shift should be preferred over a latch.
// A shift is better when the character at pos is an isolated excursion and
// the next character can be encoded in curMode.
func shouldShift(data []byte, pos int, curMode int) bool {
	if pos+1 >= len(data) {
		return true
	}
	return charMap[data[pos+1]][curMode] != -1
}

// emitShiftCode writes the appropriate shift code.
func emitShiftCode(bits *bitutil.BitArray, curMode, _ int) {
	switch curMode {
	case modeLower:
		bits.AppendBits(28, modeBits[modeLower]) // AS
	case modeDigit:
		bits.AppendBits(15, modeBits[modeDigit]) // AS
	}
}

// emitBinaryShift encodes a run of bytes using the Binary Shift mechanism.
// It returns the index of the first byte after the binary-shifted region.
//
// Format: BS code (31, 5 bits) followed by a length field and raw bytes.
//   - Length 1..31:  5-bit field containing the length.
//   - Length 32..2078: 5-bit zero field followed by 11-bit (length - 31).
func emitBinaryShift(bits *bitutil.BitArray, data []byte, pos int, curMode int) int {
	start := pos

	// Gather consecutive bytes that are not in any character mode.
	for pos < len(data) && !inAnyMode(data[pos]) {
		pos++
	}
	if pos == start {
		// The byte IS in a character mode (perhaps a different one) but we
		// were called because findBestMode returned -1. This shouldn't happen,
		// but encode one byte as binary to make progress.
		pos = start + 1
	}
	count := pos - start
	if count > 2078 {
		count = 2078
		pos = start + count
	}

	// Emit BS code.
	bits.AppendBits(31, modeBits[curMode])

	// Emit length.
	if count <= 31 {
		bits.AppendBits(uint32(count), 5)
	} else {
		bits.AppendBits(0, 5)
		bits.AppendBits(uint32(count-31), 11)
	}

	// Emit raw bytes.
	for j := start; j < start+count; j++ {
		bits.AppendBits(uint32(data[j]), 8)
	}
	return pos
}

// inAnyMode returns true if b can be encoded in at least one character mode.
func inAnyMode(b byte) bool {
	for m := 0; m < 5; m++ {
		if charMap[b][m] != -1 {
			return true
		}
	}
	return false
}
