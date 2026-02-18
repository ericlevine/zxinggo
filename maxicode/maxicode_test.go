package maxicode

import (
	"testing"

	"github.com/ericlevine/zxinggo/bitutil"
	"github.com/ericlevine/zxinggo/maxicode/decoder"
	"github.com/ericlevine/zxinggo/reedsolomon"
)

// TestDecoderMode4 builds a BitMatrix from hand-crafted codewords and decodes it.
func TestDecoderMode4(t *testing.T) {
	// Mode 4: unstructured data.
	// Layout: codewords[0..9] primary data, codewords[10..19] primary ECC (10 words),
	// codewords[20..103] secondary data (84 words), codewords[104..143] secondary ECC (40 words).
	// After decode: datawords[0..9] = codewords[0..9], datawords[10..93] = codewords[20..103].
	// getMessage(datawords, 1, 93) for mode 4 reads datawords[1]..datawords[93].
	//
	// Set A character values: \r=0, A=1, B=2, C=3, ... PAD=33.
	// We put A,B,C at datawords[1..3] (= codewords[1..3] in primary block),
	// and PAD everywhere else.

	codewords := make([]byte, 144)
	codewords[0] = 4 // mode 4
	codewords[1] = 1 // A
	codewords[2] = 2 // B
	codewords[3] = 3 // C
	for i := 4; i < 10; i++ {
		codewords[i] = 33 // PAD
	}
	for i := 20; i < 104; i++ {
		codewords[i] = 33 // PAD
	}

	enc := reedsolomon.NewEncoder(reedsolomon.MaxiCodeField64)

	// Encode primary block: 10 data + 10 ECC = 20 total
	primary := make([]int, 20)
	for i := 0; i < 10; i++ {
		primary[i] = int(codewords[i])
	}
	enc.Encode(primary, 10)
	for i := 0; i < 10; i++ {
		codewords[10+i] = byte(primary[10+i])
	}

	// Encode secondary block: even/odd interleaved, 84 data + 40 ECC = 124 total.
	// Even positions (i%2==0): 42 data + 20 ECC
	// Odd positions (i%2==1): 42 data + 20 ECC
	evenBlock := make([]int, 62) // 42 data + 20 ECC
	oddBlock := make([]int, 62)
	for i := 0; i < 84; i++ {
		if i%2 == 0 {
			evenBlock[i/2] = int(codewords[20+i])
		} else {
			oddBlock[i/2] = int(codewords[20+i])
		}
	}
	enc.Encode(evenBlock, 20)
	enc.Encode(oddBlock, 20)

	// Write ECC back interleaved into codewords[104..143]
	for i := 0; i < 40; i++ {
		if i%2 == 0 {
			codewords[104+i] = byte(evenBlock[42+i/2])
		} else {
			codewords[104+i] = byte(oddBlock[42+i/2])
		}
	}

	// Build 30x33 BitMatrix from codewords using BITNR table
	bits := buildBitMatrix(codewords)

	// Decode
	dr, err := decoder.Decode(bits)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if dr.Text != "ABC" {
		t.Errorf("got %q, want %q", dr.Text, "ABC")
	}
	if dr.ECLevel != "4" {
		t.Errorf("ec level: got %q, want %q", dr.ECLevel, "4")
	}
}

// TestDecoderMode4Longer tests a longer mode 4 message spanning into the secondary block.
func TestDecoderMode4Longer(t *testing.T) {
	codewords := make([]byte, 144)
	codewords[0] = 4 // mode 4

	// "HELLO WORLD" in Set A: H=8, E=5, L=12, L=12, O=15, ' '=32, W=23, O=15, R=18, L=12, D=4
	msg := []byte{8, 5, 12, 12, 15, 32, 23, 15, 18, 12, 4}

	// datawords[1..9] = codewords[1..9]
	// datawords[10..93] = codewords[20..103]
	// Write first 9 chars to codewords[1..9], rest to codewords[20..]
	for i := 0; i < len(msg) && i < 9; i++ {
		codewords[1+i] = msg[i]
	}
	if len(msg) > 9 {
		for i := 9; i < len(msg); i++ {
			codewords[20+(i-9)] = msg[i]
		}
	}
	// PAD the rest of primary data
	// msg fills codewords[1..9], remaining primary data already 0 but needs PAD=33
	// Actually codewords[1..9] are filled with msg[0..8] = H,E,L,L,O,' ',W,O,R
	// codewords[20..21] = msg[9..10] = L,D
	for i := 22; i < 104; i++ {
		codewords[i] = 33 // PAD
	}

	enc := reedsolomon.NewEncoder(reedsolomon.MaxiCodeField64)

	// Primary ECC
	primary := make([]int, 20)
	for i := 0; i < 10; i++ {
		primary[i] = int(codewords[i])
	}
	enc.Encode(primary, 10)
	for i := 0; i < 10; i++ {
		codewords[10+i] = byte(primary[10+i])
	}

	// Secondary ECC (even/odd interleaved)
	evenBlock := make([]int, 62)
	oddBlock := make([]int, 62)
	for i := 0; i < 84; i++ {
		if i%2 == 0 {
			evenBlock[i/2] = int(codewords[20+i])
		} else {
			oddBlock[i/2] = int(codewords[20+i])
		}
	}
	enc.Encode(evenBlock, 20)
	enc.Encode(oddBlock, 20)
	for i := 0; i < 40; i++ {
		if i%2 == 0 {
			codewords[104+i] = byte(evenBlock[42+i/2])
		} else {
			codewords[104+i] = byte(oddBlock[42+i/2])
		}
	}

	bits := buildBitMatrix(codewords)
	dr, err := decoder.Decode(bits)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if dr.Text != "HELLO WORLD" {
		t.Errorf("got %q, want %q", dr.Text, "HELLO WORLD")
	}
}

// TestExtractPureBits tests the pure bit extraction from a rendered image.
func TestExtractPureBits(t *testing.T) {
	imgW, imgH := 90, 99 // 3x scale
	img := bitutil.NewBitMatrixWithSize(imgW, imgH)

	for y := 0; y < 33; y++ {
		for x := 0; x < 30; x++ {
			if (x+y)%2 == 0 {
				for dy := 0; dy < 3; dy++ {
					for dx := 0; dx < 3; dx++ {
						ix := x*3 + dx
						iy := y*3 + dy
						if y%2 == 1 {
							ix += 1
						}
						if ix < imgW && iy < imgH {
							img.Set(ix, iy)
						}
					}
				}
			}
		}
	}

	bits, err := extractPureBits(img)
	if err != nil {
		t.Fatalf("extractPureBits error: %v", err)
	}
	if bits.Width() != 30 || bits.Height() != 33 {
		t.Errorf("dimensions: got %dx%d, want 30x33", bits.Width(), bits.Height())
	}
}

// BITNR table for test encoding.
var testBitnr = [33][30]int{
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

// buildBitMatrix creates a 30x33 BitMatrix from 144 codewords using the BITNR mapping.
func buildBitMatrix(codewords []byte) *bitutil.BitMatrix {
	bits := bitutil.NewBitMatrixWithSize(30, 33)
	for y := 0; y < 33; y++ {
		for x := 0; x < 30; x++ {
			bit := testBitnr[y][x]
			if bit >= 0 {
				cwIdx := bit / 6
				bitIdx := 5 - bit%6
				if (codewords[cwIdx] & (1 << uint(bitIdx))) != 0 {
					bits.Set(x, y)
				}
			}
		}
	}
	return bits
}
