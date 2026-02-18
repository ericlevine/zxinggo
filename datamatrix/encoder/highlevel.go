// Copyright 2006 Jeremias Maerki in part, and ZXing Authors in part.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Ported from Java ZXing library.

package encoder

import "errors"

// Encoding mode constants for the high-level encoder.
const (
	modeASCII   = 0
	modeC40     = 1
	modeText    = 2
	modeX12     = 3
	modeEDIFACT = 4
	modeBase256 = 5
)

// Special codeword values in ASCII mode.
const (
	asciiUpperShift = 235 // shifts to upper 128 characters
	asciiPad        = 129 // padding codeword (also used for 0-length remainder)
)

// Latch codewords.
const (
	latchToC40     = 230
	latchToBase256 = 231
	latchToX12     = 238
	latchToText    = 239
	latchToEDIFACT = 240
	unlatchASCII   = 254 // unlatch from C40/Text/X12 back to ASCII
)

// EncodeHighLevel performs high-level encoding of a Data Matrix message,
// producing a slice of codewords. This implementation uses ASCII mode as the
// primary encoding with an optimization for C40 mode when it saves space
// (e.g., for uppercase-heavy text).
func EncodeHighLevel(msg string) ([]byte, error) {
	if len(msg) == 0 {
		return nil, errors.New("datamatrix/encoder: empty message")
	}

	data := []byte(msg)

	// Try ASCII-only encoding first, then see if C40 can improve it.
	asciiResult := encodeASCII(data)

	// Try C40 encoding for comparison.
	c40Result := encodeWithC40(data)

	if c40Result != nil && len(c40Result) < len(asciiResult) {
		return c40Result, nil
	}
	return asciiResult, nil
}

// encodeASCII encodes data using pure ASCII mode.
// ASCII mode rules:
//   - ASCII 0-127: codeword = value + 1
//   - digit pairs "00"-"99": codeword = pair_value + 130
//   - ASCII 128-255: Upper Shift (235) then value - 128 + 1
func encodeASCII(data []byte) []byte {
	result := make([]byte, 0, len(data))
	i := 0
	for i < len(data) {
		c := data[i]

		// Check for digit pair optimization.
		if c >= '0' && c <= '9' && i+1 < len(data) && data[i+1] >= '0' && data[i+1] <= '9' {
			// Encode digit pair into single codeword.
			pairValue := (int(c)-'0')*10 + int(data[i+1]) - '0'
			result = append(result, byte(pairValue+130))
			i += 2
			continue
		}

		if c <= 127 {
			result = append(result, c+1) // ASCII value + 1
		} else {
			// Extended ASCII: Upper Shift + (value - 128 + 1)
			result = append(result, asciiUpperShift, c-128+1)
		}
		i++
	}
	return result
}

// encodeWithC40 tries to encode data using C40 mode where beneficial,
// falling back to ASCII mode for portions that don't benefit from C40.
// C40 mode encodes 3 characters into 2 codewords, efficient for uppercase text.
//
// C40 character set:
//   Set 0 (basic): Space=3, '0'-'9'=4-13, 'A'-'Z'=14-39
//   Set 1 (shift 0): ASCII 0-31
//   Set 2 (shift 1): Special characters
//   Set 3 (shift 2): Lowercase 'a'-'z' = 0-25, plus high ASCII
func encodeWithC40(data []byte) []byte {
	result := make([]byte, 0, len(data))
	i := 0

	for i < len(data) {
		// Look ahead: count how many consecutive characters can be efficiently encoded in C40.
		c40Len := 0
		for j := i; j < len(data); j++ {
			if isBasicC40(data[j]) {
				c40Len++
			} else {
				break
			}
		}

		// C40 is efficient if we can encode at least 3 characters (3 chars -> 2 codewords vs 3 codewords in ASCII).
		// We need the latch (1 cw) + data (2 cw per 3 chars) + potential unlatch (1 cw).
		// So C40 saves space if we have >= 6 consecutive C40-friendly chars, or more precisely
		// if the C40 encoding + latch/unlatch overhead is less than ASCII.
		if c40Len >= 6 {
			// Encode this run in C40.
			result = append(result, latchToC40)
			end := i + c40Len
			var buf []int
			for j := i; j < end; j++ {
				buf = append(buf, c40Value(data[j]))
			}

			// Encode triplets.
			k := 0
			for k+3 <= len(buf) {
				v := buf[k]*1600 + buf[k+1]*40 + buf[k+2] + 1
				result = append(result, byte(v/256), byte(v%256))
				k += 3
			}

			// Handle remainder: if there are 1 or 2 values left, we need to unlatch
			// and encode them in ASCII.
			remaining := len(buf) - k
			i = end - remaining // back up to handle remainder in ASCII

			// Unlatch back to ASCII mode.
			result = append(result, unlatchASCII)
		} else {
			// Encode in ASCII mode.
			c := data[i]
			if c >= '0' && c <= '9' && i+1 < len(data) && data[i+1] >= '0' && data[i+1] <= '9' {
				pairValue := (int(c)-'0')*10 + int(data[i+1]) - '0'
				result = append(result, byte(pairValue+130))
				i += 2
				continue
			}
			if c <= 127 {
				result = append(result, c+1)
			} else {
				result = append(result, asciiUpperShift, c-128+1)
			}
			i++
		}
	}

	return result
}

// isBasicC40 returns true if the byte can be encoded as a single C40 value
// (without shift characters).
func isBasicC40(b byte) bool {
	return b == ' ' || (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z')
}

// c40Value returns the C40 value for a basic C40 character.
func c40Value(b byte) int {
	if b == ' ' {
		return 3
	}
	if b >= '0' && b <= '9' {
		return int(b-'0') + 4
	}
	if b >= 'A' && b <= 'Z' {
		return int(b-'A') + 14
	}
	// Should not reach here for basic C40 characters.
	return 0
}

// randomize253State applies the 253-state randomization algorithm used for
// padding codewords. This is required by the specification so that symbols
// with identical content but different capacities produce different pad values.
func randomize253State(codeword byte, position int) byte {
	pseudoRandom := ((149 * position) % 253) + 1
	tmp := int(codeword) + pseudoRandom
	if tmp > 254 {
		tmp -= 254
	}
	return byte(tmp)
}

// PadCodewords pads the codeword slice with the appropriate pad codewords
// to fill the symbol's data capacity.
func PadCodewords(codewords []byte, capacity int) []byte {
	if len(codewords) >= capacity {
		return codewords
	}
	result := make([]byte, capacity)
	copy(result, codewords)

	// First padding codeword is always 129 (PAD).
	if len(codewords) < capacity {
		result[len(codewords)] = asciiPad
	}

	// Subsequent padding codewords use the 253-state randomization.
	for i := len(codewords) + 1; i < capacity; i++ {
		result[i] = randomize253State(asciiPad, i+1) // position is 1-based
	}

	return result
}
