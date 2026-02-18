// Package decoder implements MaxiCode decoding: bit matrix parsing, Reed-Solomon
// error correction, and character set decoding.
package decoder

import (
	"fmt"
	"strings"

	"github.com/ericlevine/zxinggo/bitutil"
	"github.com/ericlevine/zxinggo/reedsolomon"
)

// DecoderResult holds the decoded text and metadata.
type DecoderResult struct {
	Text            string
	RawBytes        []byte
	ECLevel         string
	ErrorsCorrected int
}

// interleave mode constants for correctErrors.
const (
	modeAll  = 0
	modeEven = 1
	modeOdd  = 2
)

// Decode decodes a MaxiCode from a 30x33 BitMatrix.
func Decode(bits *bitutil.BitMatrix) (*DecoderResult, error) {
	codewords := readCodewords(bits)

	rsDecoder := reedsolomon.NewDecoder(reedsolomon.MaxiCodeField64)

	errorsCorrected, err := correctErrors(rsDecoder, codewords, 0, 10, 10, modeAll)
	if err != nil {
		return nil, err
	}
	mode := int(codewords[0] & 0x0F)

	var datawords []byte
	switch mode {
	case 2, 3, 4:
		ec, err := correctErrors(rsDecoder, codewords, 20, 84, 40, modeEven)
		if err != nil {
			return nil, err
		}
		errorsCorrected += ec
		ec, err = correctErrors(rsDecoder, codewords, 20, 84, 40, modeOdd)
		if err != nil {
			return nil, err
		}
		errorsCorrected += ec
		datawords = make([]byte, 94)
	case 5:
		ec, err := correctErrors(rsDecoder, codewords, 20, 68, 56, modeEven)
		if err != nil {
			return nil, err
		}
		errorsCorrected += ec
		ec, err = correctErrors(rsDecoder, codewords, 20, 68, 56, modeOdd)
		if err != nil {
			return nil, err
		}
		errorsCorrected += ec
		datawords = make([]byte, 78)
	default:
		return nil, fmt.Errorf("maxicode: unsupported mode %d", mode)
	}

	copy(datawords[:10], codewords[:10])
	copy(datawords[10:], codewords[20:20+len(datawords)-10])

	text, err := decodeBitStream(datawords, mode)
	if err != nil {
		return nil, err
	}

	return &DecoderResult{
		Text:            text,
		RawBytes:        codewords,
		ECLevel:         fmt.Sprintf("%d", mode),
		ErrorsCorrected: errorsCorrected,
	}, nil
}

// correctErrors performs RS error correction on a subset of codewords.
// start is the offset into codewordBytes, dataCodewords+ecCodewords is the
// total block length. mode selects ALL/EVEN/ODD interleaving.
func correctErrors(rsDecoder *reedsolomon.Decoder, codewordBytes []byte,
	start, dataCodewords, ecCodewords, mode int) (int, error) {

	codewords := dataCodewords + ecCodewords
	divisor := 1
	if mode != modeAll {
		divisor = 2
	}

	codewordsInts := make([]int, codewords/divisor)
	for i := 0; i < codewords; i++ {
		if mode == modeAll || i%2 == mode-1 {
			codewordsInts[i/divisor] = int(codewordBytes[i+start]) & 0xFF
		}
	}

	errorsCorrected, err := rsDecoder.Decode(codewordsInts, ecCodewords/divisor)
	if err != nil {
		return 0, fmt.Errorf("maxicode: checksum error: %w", err)
	}

	// Copy corrected data back.
	for i := 0; i < dataCodewords; i++ {
		if mode == modeAll || i%2 == mode-1 {
			codewordBytes[i+start] = byte(codewordsInts[i/divisor])
		}
	}
	return errorsCorrected, nil
}

// --- BitMatrixParser ---

// bitnr maps (y, x) coordinates in the 33x30 MaxiCode grid to bit numbers.
// Values >= 0 are bit positions (bit/6 = codeword index, 5-bit%6 = bit within codeword).
// Values < 0 are special: -1 = fixed 0, -2 = fixed 0, -3 = fixed 0 (all unused/fixed bits).
var bitnr = [33][30]int{
	{121, 120, 127, 126, 133, 132, 139, 138, 145, 144, 151, 150, 157, 156, 163, 162, 169, 168, 175, 174, 181, 180, 187, 186, 193, 192, 199, 198, -2, -2},
	{123, 122, 129, 128, 135, 134, 141, 140, 147, 146, 153, 152, 159, 158, 165, 164, 171, 170, 177, 176, 183, 182, 189, 188, 195, 194, 201, 200, 816, -3},
	{125, 124, 131, 130, 137, 136, 143, 142, 149, 148, 155, 154, 161, 160, 167, 166, 173, 172, 179, 178, 185, 184, 191, 190, 197, 196, 203, 202, 818, 817},
	{283, 282, 277, 276, 271, 270, 265, 264, 259, 258, 253, 252, 247, 246, 241, 240, 235, 234, 229, 228, 223, 222, 217, 216, 211, 210, 205, 204, 819, -3},
	{285, 284, 279, 278, 273, 272, 267, 266, 261, 260, 255, 254, 249, 248, 243, 242, 237, 236, 231, 230, 225, 224, 219, 218, 213, 212, 207, 206, 821, 820},
	{287, 286, 281, 280, 275, 274, 269, 268, 263, 262, 257, 256, 251, 250, 245, 244, 239, 238, 233, 232, 227, 226, 221, 220, 215, 214, 209, 208, 822, -3},
	{289, 288, 295, 294, 301, 300, 307, 306, 313, 312, 319, 318, 325, 324, 331, 330, 337, 336, 343, 342, 349, 348, 355, 354, 361, 360, 367, 366, 824, 823},
	{291, 290, 297, 296, 303, 302, 309, 308, 315, 314, 321, 320, 327, 326, 333, 332, 339, 338, 345, 344, 351, 350, 357, 356, 363, 362, 369, 368, 825, -3},
	{293, 292, 299, 298, 305, 304, 311, 310, 317, 316, 323, 322, 329, 328, 335, 334, 341, 340, 347, 346, 353, 352, 359, 358, 365, 364, 371, 370, 827, 826},
	{409, 408, 403, 402, 397, 396, 391, 390, 79, 78, -2, -2, 13, 12, 37, 36, 2, -1, 44, 43, 109, 108, 385, 384, 379, 378, 373, 372, 828, -3},
	{411, 410, 405, 404, 399, 398, 393, 392, 81, 80, 40, -2, 15, 14, 39, 38, 3, -1, -1, 45, 111, 110, 387, 386, 381, 380, 375, 374, 830, 829},
	{413, 412, 407, 406, 401, 400, 395, 394, 83, 82, 41, -3, -3, -3, -3, -3, 5, 4, 47, 46, 113, 112, 389, 388, 383, 382, 377, 376, 831, -3},
	{415, 414, 421, 420, 427, 426, 103, 102, 55, 54, 16, -3, -3, -3, -3, -3, -3, -3, 20, 19, 85, 84, 433, 432, 439, 438, 445, 444, 833, 832},
	{417, 416, 423, 422, 429, 428, 105, 104, 57, 56, -3, -3, -3, -3, -3, -3, -3, -3, 22, 21, 87, 86, 435, 434, 441, 440, 447, 446, 834, -3},
	{419, 418, 425, 424, 431, 430, 107, 106, 59, 58, -3, -3, -3, -3, -3, -3, -3, -3, -3, 23, 89, 88, 437, 436, 443, 442, 449, 448, 836, 835},
	{481, 480, 475, 474, 469, 468, 48, -2, 30, -3, -3, -3, -3, -3, -3, -3, -3, -3, -3, 0, 53, 52, 463, 462, 457, 456, 451, 450, 837, -3},
	{483, 482, 477, 476, 471, 470, 49, -1, -2, -3, -3, -3, -3, -3, -3, -3, -3, -3, -3, -3, -2, -1, 465, 464, 459, 458, 453, 452, 839, 838},
	{485, 484, 479, 478, 473, 472, 51, 50, 31, -3, -3, -3, -3, -3, -3, -3, -3, -3, -3, 1, -2, 42, 467, 466, 461, 460, 455, 454, 840, -3},
	{487, 486, 493, 492, 499, 498, 97, 96, 61, 60, -3, -3, -3, -3, -3, -3, -3, -3, -3, 26, 91, 90, 505, 504, 511, 510, 517, 516, 842, 841},
	{489, 488, 495, 494, 501, 500, 99, 98, 63, 62, -3, -3, -3, -3, -3, -3, -3, -3, 28, 27, 93, 92, 507, 506, 513, 512, 519, 518, 843, -3},
	{491, 490, 497, 496, 503, 502, 101, 100, 65, 64, 17, -3, -3, -3, -3, -3, -3, -3, 18, 29, 95, 94, 509, 508, 515, 514, 521, 520, 845, 844},
	{559, 558, 553, 552, 547, 546, 541, 540, 73, 72, 32, -3, -3, -3, -3, -3, -3, 10, 67, 66, 115, 114, 535, 534, 529, 528, 523, 522, 846, -3},
	{561, 560, 555, 554, 549, 548, 543, 542, 75, 74, -2, -1, 7, 6, 35, 34, 11, -2, 69, 68, 117, 116, 537, 536, 531, 530, 525, 524, 848, 847},
	{563, 562, 557, 556, 551, 550, 545, 544, 77, 76, -2, 33, 9, 8, 25, 24, -1, -2, 71, 70, 119, 118, 539, 538, 533, 532, 527, 526, 849, -3},
	{565, 564, 571, 570, 577, 576, 583, 582, 589, 588, 595, 594, 601, 600, 607, 606, 613, 612, 619, 618, 625, 624, 631, 630, 637, 636, 643, 642, 851, 850},
	{567, 566, 573, 572, 579, 578, 585, 584, 591, 590, 597, 596, 603, 602, 609, 608, 615, 614, 621, 620, 627, 626, 633, 632, 639, 638, 645, 644, 852, -3},
	{569, 568, 575, 574, 581, 580, 587, 586, 593, 592, 599, 598, 605, 604, 611, 610, 617, 616, 623, 622, 629, 628, 635, 634, 641, 640, 647, 646, 854, 853},
	{727, 726, 721, 720, 715, 714, 709, 708, 703, 702, 697, 696, 691, 690, 685, 684, 679, 678, 673, 672, 667, 666, 661, 660, 655, 654, 649, 648, 855, -3},
	{729, 728, 723, 722, 717, 716, 711, 710, 705, 704, 699, 698, 693, 692, 687, 686, 681, 680, 675, 674, 669, 668, 663, 662, 657, 656, 651, 650, 857, 856},
	{731, 730, 725, 724, 719, 718, 713, 712, 707, 706, 701, 700, 695, 694, 689, 688, 683, 682, 677, 676, 671, 670, 665, 664, 659, 658, 653, 652, 858, -3},
	{733, 732, 739, 738, 745, 744, 751, 750, 757, 756, 763, 762, 769, 768, 775, 774, 781, 780, 787, 786, 793, 792, 799, 798, 805, 804, 811, 810, 860, 859},
	{735, 734, 741, 740, 747, 746, 753, 752, 759, 758, 765, 764, 771, 770, 777, 776, 783, 782, 789, 788, 795, 794, 801, 800, 807, 806, 813, 812, 861, -3},
	{737, 736, 743, 742, 749, 748, 755, 754, 761, 760, 767, 766, 773, 772, 779, 778, 785, 784, 791, 790, 797, 796, 803, 802, 809, 808, 815, 814, 863, 862},
}

// readCodewords reads 144 codewords (6 bits each) from a 30x33 MaxiCode BitMatrix.
func readCodewords(matrix *bitutil.BitMatrix) []byte {
	result := make([]byte, 144)
	height := matrix.Height()
	width := matrix.Width()
	for y := 0; y < height; y++ {
		row := bitnr[y]
		for x := 0; x < width; x++ {
			bit := row[x]
			if bit >= 0 && matrix.Get(x, y) {
				result[bit/6] |= byte(1 << uint(5-bit%6))
			}
		}
	}
	return result
}

// --- DecodedBitStreamParser ---

// Special control characters used in MaxiCode character sets.
const (
	shiftA     = '\uFFF0'
	shiftB     = '\uFFF1'
	shiftC     = '\uFFF2'
	shiftD     = '\uFFF3'
	shiftE     = '\uFFF4'
	twoShiftA  = '\uFFF5'
	threeShiftA = '\uFFF6'
	latchA     = '\uFFF7'
	latchB     = '\uFFF8'
	lockChar   = '\uFFF9'
	eciChar    = '\uFFFA'
	nsChar     = '\uFFFB'
	padChar    = '\uFFFC'
	fsChar     = '\u001C'
	gsChar     = '\u001D'
	rsChar     = '\u001E'
)

// Byte indices for structured data extraction (modes 2 & 3).
var countryBytes = []byte{53, 54, 43, 44, 45, 46, 47, 48, 37, 38}
var serviceClassBytes = []byte{55, 56, 57, 58, 59, 60, 49, 50, 51, 52}
var postcode2LengthBytes = []byte{39, 40, 41, 42, 31, 32}
var postcode2Bytes = []byte{33, 34, 35, 36, 25, 26, 27, 28, 29, 30, 19,
	20, 21, 22, 23, 24, 13, 14, 15, 16, 17, 18, 7, 8, 9, 10, 11, 12, 1, 2}
var postcode3Bytes = [][]byte{
	{39, 40, 41, 42, 31, 32},
	{33, 34, 35, 36, 25, 26},
	{27, 28, 29, 30, 19, 20},
	{21, 22, 23, 24, 13, 14},
	{15, 16, 17, 18, 7, 8},
	{9, 10, 11, 12, 1, 2},
}

// The 5 MaxiCode character sets. Each string has 64 entries indexed by 6-bit codeword value.
var sets = [5]string{
	// Set A
	"\rABCDEFGHIJKLMNOPQRSTUVWXYZ" + string(eciChar) + string(fsChar) + string(gsChar) + string(rsChar) + string(nsChar) + " " + string(padChar) +
		"\"#$%&'()*+,-./0123456789:" + string(shiftB) + string(shiftC) + string(shiftD) + string(shiftE) + string(latchB),
	// Set B
	"`abcdefghijklmnopqrstuvwxyz" + string(eciChar) + string(fsChar) + string(gsChar) + string(rsChar) + string(nsChar) + "{" + string(padChar) +
		"}~\u007F;<=>?[\\]^_ ,./:@!|" + string(padChar) + string(twoShiftA) + string(threeShiftA) + string(padChar) +
		string(shiftA) + string(shiftC) + string(shiftD) + string(shiftE) + string(latchA),
	// Set C
	"\u00C0\u00C1\u00C2\u00C3\u00C4\u00C5\u00C6\u00C7\u00C8\u00C9\u00CA\u00CB\u00CC\u00CD\u00CE\u00CF\u00D0\u00D1\u00D2\u00D3\u00D4\u00D5\u00D6\u00D7\u00D8\u00D9\u00DA" +
		string(eciChar) + string(fsChar) + string(gsChar) + string(rsChar) + string(nsChar) +
		"\u00DB\u00DC\u00DD\u00DE\u00DF\u00AA\u00AC\u00B1\u00B2\u00B3\u00B5\u00B9\u00BA\u00BC\u00BD\u00BE\u0080\u0081\u0082\u0083\u0084\u0085\u0086\u0087\u0088\u0089" +
		string(latchA) + " " + string(lockChar) + string(shiftD) + string(shiftE) + string(latchB),
	// Set D
	"\u00E0\u00E1\u00E2\u00E3\u00E4\u00E5\u00E6\u00E7\u00E8\u00E9\u00EA\u00EB\u00EC\u00ED\u00EE\u00EF\u00F0\u00F1\u00F2\u00F3\u00F4\u00F5\u00F6\u00F7\u00F8\u00F9\u00FA" +
		string(eciChar) + string(fsChar) + string(gsChar) + string(rsChar) + string(nsChar) +
		"\u00FB\u00FC\u00FD\u00FE\u00FF\u00A1\u00A8\u00AB\u00AF\u00B0\u00B4\u00B7\u00B8\u00BB\u00BF\u008A\u008B\u008C\u008D\u008E\u008F\u0090\u0091\u0092\u0093\u0094" +
		string(latchA) + " " + string(shiftC) + string(lockChar) + string(shiftE) + string(latchB),
	// Set E
	"\u0000\u0001\u0002\u0003\u0004\u0005\u0006\u0007\u0008\u0009\n\u000B\u000C\r\u000E\u000F\u0010\u0011\u0012\u0013\u0014\u0015\u0016\u0017\u0018\u0019\u001A" +
		string(eciChar) + string(padChar) + string(padChar) + "\u001B" + string(nsChar) + string(fsChar) + string(gsChar) + string(rsChar) +
		"\u001F\u009F\u00A0\u00A2\u00A3\u00A4\u00A5\u00A6\u00A7\u00A9\u00AD\u00AE\u00B6\u0095\u0096\u0097\u0098\u0099\u009A\u009B\u009C\u009D\u009E" +
		string(latchA) + " " + string(shiftC) + string(shiftD) + string(lockChar) + string(latchB),
}

// decodeBitStream decodes the data bytes into text according to the mode.
func decodeBitStream(bytes []byte, mode int) (string, error) {
	var result strings.Builder
	result.Grow(144)

	switch mode {
	case 2, 3:
		var postcode string
		if mode == 2 {
			pc := getInt(bytes, postcode2Bytes)
			ps2Length := getInt(bytes, postcode2LengthBytes)
			if ps2Length > 10 {
				return "", fmt.Errorf("maxicode: invalid postcode length %d", ps2Length)
			}
			postcode = fmt.Sprintf("%0*d", ps2Length, pc)
		} else {
			postcode = getPostCode3(bytes)
		}
		country := fmt.Sprintf("%03d", getInt(bytes, countryBytes))
		service := fmt.Sprintf("%03d", getInt(bytes, serviceClassBytes))
		msg := getMessage(bytes, 10, 84)
		prefix := string(rsChar) + "01" + string(gsChar)
		if strings.HasPrefix(msg, "[)>"+prefix) && len(msg) >= 9 {
			// Insert structured data at position 9 (after [)>RS01GS + 2-char format type)
			result.WriteString(msg[:9])
			result.WriteString(postcode + string(gsChar) + country + string(gsChar) + service + string(gsChar))
			result.WriteString(msg[9:])
		} else {
			result.WriteString(postcode + string(gsChar) + country + string(gsChar) + service + string(gsChar))
			result.WriteString(msg)
		}
	case 4:
		result.WriteString(getMessage(bytes, 1, 93))
	case 5:
		result.WriteString(getMessage(bytes, 1, 77))
	}
	return result.String(), nil
}

// getBit returns bit value (0 or 1) at the given 1-based bit position in bytes.
func getBit(bit int, bytes []byte) int {
	bit--
	if (bytes[bit/6] & (1 << uint(5-bit%6))) == 0 {
		return 0
	}
	return 1
}

// getInt extracts a multi-bit integer from bytes using the given bit position array.
func getInt(bytes []byte, x []byte) int {
	val := 0
	for i := 0; i < len(x); i++ {
		val += getBit(int(x[i]), bytes) << uint(len(x)-i-1)
	}
	return val
}

// getPostCode3 extracts a 6-character alphanumeric postcode (mode 3).
func getPostCode3(bytes []byte) string {
	var sb strings.Builder
	sb.Grow(len(postcode3Bytes))
	for _, p3bytes := range postcode3Bytes {
		idx := getInt(bytes, p3bytes)
		r := []rune(sets[0])
		if idx < len(r) {
			sb.WriteRune(r[idx])
		}
	}
	return sb.String()
}

// getMessage decodes a sequence of codeword bytes using the MaxiCode character set state machine.
func getMessage(bytes []byte, start, length int) string {
	var sb strings.Builder
	shift := -1
	set := 0
	lastset := 0

	setRunes := [5][]rune{
		[]rune(sets[0]),
		[]rune(sets[1]),
		[]rune(sets[2]),
		[]rune(sets[3]),
		[]rune(sets[4]),
	}

	for i := start; i < start+length; i++ {
		idx := int(bytes[i])
		if idx >= len(setRunes[set]) {
			continue
		}
		c := setRunes[set][idx]
		switch c {
		case latchA:
			set = 0
			shift = -1
		case latchB:
			set = 1
			shift = -1
		case shiftA, shiftB, shiftC, shiftD, shiftE:
			lastset = set
			set = int(c - shiftA)
			shift = 1
		case twoShiftA:
			lastset = set
			set = 0
			shift = 2
		case threeShiftA:
			lastset = set
			set = 0
			shift = 3
		case nsChar:
			// Numeric shift: next 5 bytes encode a 9-digit number.
			if i+5 < start+length {
				nsval := (int(bytes[i+1]) << 24) + (int(bytes[i+2]) << 18) +
					(int(bytes[i+3]) << 12) + (int(bytes[i+4]) << 6) + int(bytes[i+5])
				sb.WriteString(fmt.Sprintf("%09d", nsval))
				i += 5
			}
		case lockChar:
			shift = -1
		default:
			sb.WriteRune(c)
		}
		// Java uses post-decrement: if (shift-- == 0) — checks BEFORE decrementing.
		if shift == 0 {
			set = lastset
		}
		shift--
	}
	// Strip trailing PAD characters.
	result := sb.String()
	for strings.HasSuffix(result, string(padChar)) {
		result = result[:len(result)-len(string(padChar))]
	}
	return result
}
