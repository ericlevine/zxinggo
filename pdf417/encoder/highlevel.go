// Copyright 2006 Jeremias Maerki in part, and ZXing Authors in part.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Ported from Java ZXing library.

package encoder

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// Compaction mode constants
const (
	textCompaction    = 0
	byteCompaction    = 1
	numericCompaction = 2
)

// Text compaction submode constants
const (
	submodeAlpha       = 0
	submodeLower       = 1
	submodeMixed       = 2
	submodePunctuation = 3
)

// Mode latch and shift constants
const (
	latchToText       = 900
	latchToBytePadded = 901
	latchToNumeric    = 902
	shiftToByte       = 913
	latchToByte       = 924
	eciUserDefined    = 925
	eciGeneralPurpose = 926
	eciCharset        = 927
)

// Compaction represents possible PDF417 barcode compaction types.
type Compaction int

const (
	// CompactionAuto selects compaction mode automatically.
	CompactionAuto Compaction = iota
	// CompactionText forces text compaction mode.
	CompactionText
	// CompactionByte forces byte compaction mode.
	CompactionByte
	// CompactionNumeric forces numeric compaction mode.
	CompactionNumeric
)

// textMixedRaw is the raw code table for text compaction Mixed sub-mode.
var textMixedRaw = []byte{
	48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 38, 13, 9, 44, 58,
	35, 45, 46, 36, 47, 43, 37, 42, 61, 94, 0, 32, 0, 0, 0,
}

// textPunctuationRaw is the raw code table for text compaction Punctuation sub-mode.
var textPunctuationRaw = []byte{
	59, 60, 62, 64, 91, 92, 93, 95, 96, 126, 33, 13, 9, 44, 58,
	10, 45, 46, 36, 47, 34, 124, 42, 40, 41, 63, 123, 125, 39, 0,
}

// mixed is the inverse lookup table for the mixed sub-mode.
var mixed [128]int

// punctuation is the inverse lookup table for the punctuation sub-mode.
var punctuation [128]int

func init() {
	// Construct inverse lookups
	for i := range mixed {
		mixed[i] = -1
	}
	for i, b := range textMixedRaw {
		if b > 0 {
			mixed[b] = i
		}
	}
	for i := range punctuation {
		punctuation[i] = -1
	}
	for i, b := range textPunctuationRaw {
		if b > 0 {
			punctuation[b] = i
		}
	}
}

// EncodeHighLevel performs high-level encoding of a PDF417 message using the
// algorithm described in annex P of ISO/IEC 15438:2001(E).
// This is a simplified port that does not support ECI or custom charsets.
func EncodeHighLevel(msg string, compaction Compaction) (string, error) {
	if len(msg) == 0 {
		return "", errors.New("empty message not allowed")
	}

	if compaction == CompactionText {
		for i, ch := range msg {
			if ch > 127 {
				return "", fmt.Errorf("non-encodable character detected: %c (Unicode: %d) at position #%d", ch, ch, i)
			}
		}
	}

	// For non-text compaction without ECI, check ISO-8859-1 range
	if compaction == CompactionAuto || compaction == CompactionByte {
		for i, ch := range msg {
			if ch > 255 {
				return "", fmt.Errorf("non-encodable character detected: %c (Unicode: %d) at position #%d", ch, ch, i)
			}
		}
	}

	var sb strings.Builder
	sb.Grow(len(msg))

	msgLen := len(msg)
	p := 0
	textSubMode := submodeAlpha

	switch compaction {
	case CompactionText:
		encodeText(msg, p, msgLen, &sb, textSubMode)

	case CompactionByte:
		msgBytes := []byte(msg)
		encodeBinary(msgBytes, p, len(msgBytes), byteCompaction, &sb)

	case CompactionNumeric:
		sb.WriteRune(rune(latchToNumeric))
		encodeNumeric(msg, p, msgLen, &sb)

	default: // CompactionAuto
		encodingMode := textCompaction // Default mode, see 4.4.2.1
		for p < msgLen {
			n := determineConsecutiveDigitCount(msg, p)
			if n >= 13 {
				sb.WriteRune(rune(latchToNumeric))
				encodingMode = numericCompaction
				textSubMode = submodeAlpha // Reset after latch
				encodeNumeric(msg, p, n, &sb)
				p += n
			} else {
				t := determineConsecutiveTextCount(msg, p)
				if t >= 5 || n == msgLen {
					if encodingMode != textCompaction {
						sb.WriteRune(rune(latchToText))
						encodingMode = textCompaction
						textSubMode = submodeAlpha // start with submode alpha after latch
					}
					textSubMode = encodeText(msg, p, t, &sb, textSubMode)
					p += t
				} else {
					b := determineConsecutiveBinaryCount(msg, p)
					if b == 0 {
						b = 1
					}
					bytesData := []byte(msg[p : p+b])
					if len(bytesData) == 1 && encodingMode == textCompaction {
						// Switch for one byte (instead of latch)
						encodeBinary(bytesData, 0, 1, textCompaction, &sb)
					} else {
						// Mode latch performed by encodeBinary()
						encodeBinary(bytesData, 0, len(bytesData), encodingMode, &sb)
						encodingMode = byteCompaction
						textSubMode = submodeAlpha // Reset after latch
					}
					p += b
				}
			}
		}
	}

	return sb.String(), nil
}

// encodeText encodes parts of the message using Text Compaction as described
// in ISO/IEC 15438:2001(E), chapter 4.4.2.
func encodeText(msg string, startpos, count int, sb *strings.Builder, initialSubmode int) int {
	var tmp strings.Builder
	tmp.Grow(count)
	submode := initialSubmode
	idx := 0

	for {
		ch := msg[startpos+idx]
		switch submode {
		case submodeAlpha:
			if isAlphaUpper(ch) {
				if ch == ' ' {
					tmp.WriteRune(26) // space
				} else {
					tmp.WriteRune(rune(ch - 65))
				}
			} else {
				if isAlphaLower(ch) {
					submode = submodeLower
					tmp.WriteRune(27) // ll
					continue
				} else if isMixed(ch) {
					submode = submodeMixed
					tmp.WriteRune(28) // ml
					continue
				} else {
					tmp.WriteRune(29) // ps
					tmp.WriteRune(rune(punctuation[ch]))
				}
			}

		case submodeLower:
			if isAlphaLower(ch) {
				if ch == ' ' {
					tmp.WriteRune(26) // space
				} else {
					tmp.WriteRune(rune(ch - 97))
				}
			} else {
				if isAlphaUpper(ch) {
					tmp.WriteRune(27) // as
					tmp.WriteRune(rune(ch - 65))
				} else if isMixed(ch) {
					submode = submodeMixed
					tmp.WriteRune(28) // ml
					continue
				} else {
					tmp.WriteRune(29) // ps
					tmp.WriteRune(rune(punctuation[ch]))
				}
			}

		case submodeMixed:
			if isMixed(ch) {
				tmp.WriteRune(rune(mixed[ch]))
			} else {
				if isAlphaUpper(ch) {
					submode = submodeAlpha
					tmp.WriteRune(28) // al
					continue
				} else if isAlphaLower(ch) {
					submode = submodeLower
					tmp.WriteRune(27) // ll
					continue
				} else {
					if startpos+idx+1 < count && isPunctuation(msg[startpos+idx+1]) {
						submode = submodePunctuation
						tmp.WriteRune(25) // pl
						continue
					}
					tmp.WriteRune(29) // ps
					tmp.WriteRune(rune(punctuation[ch]))
				}
			}

		default: // submodePunctuation
			if isPunctuation(ch) {
				tmp.WriteRune(rune(punctuation[ch]))
			} else {
				submode = submodeAlpha
				tmp.WriteRune(29) // al
				continue
			}
		}
		idx++
		if idx >= count {
			break
		}
	}

	tmpStr := tmp.String()
	tmpRunes := []rune(tmpStr)
	h := rune(0)
	tLen := len(tmpRunes)
	for i := 0; i < tLen; i++ {
		odd := (i % 2) != 0
		if odd {
			h = h*30 + tmpRunes[i]
			sb.WriteRune(h)
		} else {
			h = tmpRunes[i]
		}
	}
	if (tLen % 2) != 0 {
		sb.WriteRune(h*30 + 29) // ps
	}
	return submode
}

// encodeBinary encodes parts of the message using Byte Compaction as described
// in ISO/IEC 15438:2001(E), chapter 4.4.3.
func encodeBinary(bytes []byte, startpos, count, startmode int, sb *strings.Builder) {
	if count == 1 && startmode == textCompaction {
		sb.WriteRune(rune(shiftToByte))
	} else {
		if (count % 6) == 0 {
			sb.WriteRune(rune(latchToByte))
		} else {
			sb.WriteRune(rune(latchToBytePadded))
		}
	}

	idx := startpos
	// Encode sixpacks
	if count >= 6 {
		chars := make([]rune, 5)
		for (startpos+count-idx) >= 6 {
			var t int64
			for i := 0; i < 6; i++ {
				t <<= 8
				t += int64(bytes[idx+i]) & 0xff
			}
			for i := 0; i < 5; i++ {
				chars[i] = rune(t % 900)
				t /= 900
			}
			for i := len(chars) - 1; i >= 0; i-- {
				sb.WriteRune(chars[i])
			}
			idx += 6
		}
	}
	// Encode rest (remaining n<5 bytes if any)
	for i := idx; i < startpos+count; i++ {
		ch := int(bytes[i]) & 0xff
		sb.WriteRune(rune(ch))
	}
}

// encodeNumeric encodes parts of the message using Numeric Compaction.
func encodeNumeric(msg string, startpos, count int, sb *strings.Builder) {
	idx := 0
	var tmp strings.Builder
	tmp.Grow(count/3 + 1)
	num900 := big.NewInt(900)
	num0 := big.NewInt(0)
	for idx < count {
		tmp.Reset()
		length := 44
		if count-idx < 44 {
			length = count - idx
		}
		part := "1" + msg[startpos+idx:startpos+idx+length]
		bigint := new(big.Int)
		bigint.SetString(part, 10)

		tmpRunes := make([]rune, 0, length/3+1)
		mod := new(big.Int)
		for {
			bigint.DivMod(bigint, num900, mod)
			tmpRunes = append(tmpRunes, rune(mod.Int64()))
			if bigint.Cmp(num0) == 0 {
				break
			}
		}

		// Reverse and append
		for i := len(tmpRunes) - 1; i >= 0; i-- {
			sb.WriteRune(tmpRunes[i])
		}
		idx += length
	}
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isAlphaUpper(ch byte) bool {
	return ch == ' ' || (ch >= 'A' && ch <= 'Z')
}

func isAlphaLower(ch byte) bool {
	return ch == ' ' || (ch >= 'a' && ch <= 'z')
}

func isMixed(ch byte) bool {
	return mixed[ch] != -1
}

func isPunctuation(ch byte) bool {
	return punctuation[ch] != -1
}

func isText(ch byte) bool {
	return ch == '\t' || ch == '\n' || ch == '\r' || (ch >= 32 && ch <= 126)
}

// determineConsecutiveDigitCount determines the number of consecutive
// characters that are encodable using numeric compaction.
func determineConsecutiveDigitCount(msg string, startpos int) int {
	count := 0
	msgLen := len(msg)
	idx := startpos
	if idx < msgLen {
		for idx < msgLen && isDigit(msg[idx]) {
			count++
			idx++
		}
	}
	return count
}

// determineConsecutiveTextCount determines the number of consecutive
// characters that are encodable using text compaction.
func determineConsecutiveTextCount(msg string, startpos int) int {
	msgLen := len(msg)
	idx := startpos
	for idx < msgLen {
		numericCount := 0
		for numericCount < 13 && idx < msgLen && isDigit(msg[idx]) {
			numericCount++
			idx++
		}
		if numericCount >= 13 {
			return idx - startpos - numericCount
		}
		if numericCount > 0 {
			// Heuristic: All text-encodable chars or digits are binary encodable
			continue
		}

		// Check if character is encodable
		if !isText(msg[idx]) {
			break
		}
		idx++
	}
	return idx - startpos
}

// determineConsecutiveBinaryCount determines the number of consecutive
// characters that are encodable using binary compaction.
func determineConsecutiveBinaryCount(msg string, startpos int) int {
	msgLen := len(msg)
	idx := startpos
	for idx < msgLen {
		numericCount := 0

		i := idx
		for numericCount < 13 && isDigit(msg[i]) {
			numericCount++
			i = idx + numericCount
			if i >= msgLen {
				break
			}
		}
		if numericCount >= 13 {
			return idx - startpos
		}
		idx++
	}
	return idx - startpos
}
