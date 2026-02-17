package decoder

import (
	"fmt"
	"math/bits"

	"github.com/ericlevine/zxinggo/bitutil"
)

// ECB represents a single error-correction block specification.
type ECB struct {
	Count         int
	DataCodewords int
}

// ECBlocks represents a set of error-correction blocks for one EC level.
type ECBlocks struct {
	ECCodewordsPerBlock int
	Blocks              []ECB
}

// NumBlocks returns the total number of blocks.
func (ecb *ECBlocks) NumBlocks() int {
	total := 0
	for _, b := range ecb.Blocks {
		total += b.Count
	}
	return total
}

// TotalECCodewords returns the total number of error-correction codewords.
func (ecb *ECBlocks) TotalECCodewords() int {
	return ecb.ECCodewordsPerBlock * ecb.NumBlocks()
}

// Version represents a QR code version (1-40).
type Version struct {
	Number                  int
	AlignmentPatternCenters []int
	ECBlocksArray           [4]ECBlocks // L, M, Q, H
	TotalCodewords          int
}

// DimensionForVersion returns the module dimension for this version.
func (v *Version) DimensionForVersion() int {
	return 17 + 4*v.Number
}

// ECBlocksForLevel returns the ECBlocks for the given error correction level.
func (v *Version) ECBlocksForLevel(ecLevel ErrorCorrectionLevel) *ECBlocks {
	return &v.ECBlocksArray[ecLevel.Ordinal()]
}

// BuildFunctionPattern builds a BitMatrix indicating function pattern modules.
func (v *Version) BuildFunctionPattern() *bitutil.BitMatrix {
	dimension := v.DimensionForVersion()
	bm := bitutil.NewBitMatrix(dimension)

	// Top left finder pattern + separator + format
	bm.SetRegion(0, 0, 9, 9)
	// Top right finder pattern + separator + format
	bm.SetRegion(dimension-8, 0, 8, 9)
	// Bottom left finder pattern + separator + format
	bm.SetRegion(0, dimension-8, 9, 8)

	// Alignment patterns
	max := len(v.AlignmentPatternCenters)
	for x := 0; x < max; x++ {
		i := v.AlignmentPatternCenters[x] - 2
		for y := 0; y < max; y++ {
			if (x != 0 || (y != 0 && y != max-1)) && (x != max-1 || y != 0) {
				bm.SetRegion(v.AlignmentPatternCenters[y]-2, i, 5, 5)
			}
		}
	}

	// Vertical timing pattern
	bm.SetRegion(6, 9, 1, dimension-17)
	// Horizontal timing pattern
	bm.SetRegion(9, 6, dimension-17, 1)

	if v.Number > 6 {
		// Version info, top right
		bm.SetRegion(dimension-11, 0, 3, 6)
		// Version info, bottom left
		bm.SetRegion(0, dimension-11, 6, 3)
	}

	return bm
}

// versionDecodeInfo maps version bits to versions 7+.
var versionDecodeInfo = []int{
	0x07C94, 0x085BC, 0x09A99, 0x0A4D3, 0x0BBF6,
	0x0C762, 0x0D847, 0x0E60D, 0x0F928, 0x10B78,
	0x1145D, 0x12A17, 0x13532, 0x149A6, 0x15683,
	0x168C9, 0x177EC, 0x18EC4, 0x191E1, 0x1AFAB,
	0x1B08E, 0x1CC1A, 0x1D33F, 0x1ED75, 0x1F250,
	0x209D5, 0x216F0, 0x228BA, 0x2379F, 0x24B0B,
	0x2542E, 0x26A64, 0x27541, 0x28C69,
}

// GetVersionForNumber returns the Version for the given version number (1-40).
func GetVersionForNumber(number int) (*Version, error) {
	if number < 1 || number > 40 {
		return nil, errInvalidVersion
	}
	return &versions[number-1], nil
}

// GetProvisionalVersionForDimension returns the Version for a QR code of the given dimension.
func GetProvisionalVersionForDimension(dimension int) (*Version, error) {
	if dimension%4 != 1 {
		return nil, fmt.Errorf("qrcode/decoder: invalid dimension %d", dimension)
	}
	return GetVersionForNumber((dimension - 17) / 4)
}

// DecodeVersionInformation decodes version information bits.
func DecodeVersionInformation(versionBits int) *Version {
	bestDifference := 32
	bestVersion := 0
	for i, target := range versionDecodeInfo {
		if target == versionBits {
			v := &versions[i+6]
			return v
		}
		bitsDiff := bits.OnesCount(uint(versionBits ^ target))
		if bitsDiff < bestDifference {
			bestVersion = i + 7
			bestDifference = bitsDiff
		}
	}
	if bestDifference <= 3 {
		v := &versions[bestVersion-1]
		return v
	}
	return nil
}

func newVersion(number int, align []int, l, m, q, h ECBlocks) Version {
	v := Version{
		Number:                  number,
		AlignmentPatternCenters: align,
		ECBlocksArray:           [4]ECBlocks{l, m, q, h},
	}
	total := 0
	ecCodewords := l.ECCodewordsPerBlock
	for _, block := range l.Blocks {
		total += block.Count * (block.DataCodewords + ecCodewords)
	}
	v.TotalCodewords = total
	return v
}

func eb(ecCW int, blocks ...ECB) ECBlocks {
	return ECBlocks{ECCodewordsPerBlock: ecCW, Blocks: blocks}
}

func b(count, dataCodewords int) ECB {
	return ECB{Count: count, DataCodewords: dataCodewords}
}

// versions contains all 40 QR code versions.
var versions = [40]Version{
	newVersion(1, nil, eb(7, b(1, 19)), eb(10, b(1, 16)), eb(13, b(1, 13)), eb(17, b(1, 9))),
	newVersion(2, []int{6, 18}, eb(10, b(1, 34)), eb(16, b(1, 28)), eb(22, b(1, 22)), eb(28, b(1, 16))),
	newVersion(3, []int{6, 22}, eb(15, b(1, 55)), eb(26, b(1, 44)), eb(18, b(2, 17)), eb(22, b(2, 13))),
	newVersion(4, []int{6, 26}, eb(20, b(1, 80)), eb(18, b(2, 32)), eb(26, b(2, 24)), eb(16, b(4, 9))),
	newVersion(5, []int{6, 30}, eb(26, b(1, 108)), eb(24, b(2, 43)), eb(18, b(2, 15), b(2, 16)), eb(22, b(2, 11), b(2, 12))),
	newVersion(6, []int{6, 34}, eb(18, b(2, 68)), eb(16, b(4, 27)), eb(24, b(4, 19)), eb(28, b(4, 15))),
	newVersion(7, []int{6, 22, 38}, eb(20, b(2, 78)), eb(18, b(4, 31)), eb(18, b(2, 14), b(4, 15)), eb(26, b(4, 13), b(1, 14))),
	newVersion(8, []int{6, 24, 42}, eb(24, b(2, 97)), eb(22, b(2, 38), b(2, 39)), eb(22, b(4, 18), b(2, 19)), eb(26, b(4, 14), b(2, 15))),
	newVersion(9, []int{6, 26, 46}, eb(30, b(2, 116)), eb(22, b(3, 36), b(2, 37)), eb(20, b(4, 16), b(4, 17)), eb(24, b(4, 12), b(4, 13))),
	newVersion(10, []int{6, 28, 50}, eb(18, b(2, 68), b(2, 69)), eb(26, b(4, 43), b(1, 44)), eb(24, b(6, 19), b(2, 20)), eb(28, b(6, 15), b(2, 16))),
	newVersion(11, []int{6, 30, 54}, eb(20, b(4, 81)), eb(30, b(1, 50), b(4, 51)), eb(28, b(4, 22), b(4, 23)), eb(24, b(3, 12), b(8, 13))),
	newVersion(12, []int{6, 32, 58}, eb(24, b(2, 92), b(2, 93)), eb(22, b(6, 36), b(2, 37)), eb(26, b(4, 20), b(6, 21)), eb(28, b(7, 14), b(4, 15))),
	newVersion(13, []int{6, 34, 62}, eb(26, b(4, 107)), eb(22, b(8, 37), b(1, 38)), eb(24, b(8, 20), b(4, 21)), eb(22, b(12, 11), b(4, 12))),
	newVersion(14, []int{6, 26, 46, 66}, eb(30, b(3, 115), b(1, 116)), eb(24, b(4, 40), b(5, 41)), eb(20, b(11, 16), b(5, 17)), eb(24, b(11, 12), b(5, 13))),
	newVersion(15, []int{6, 26, 48, 70}, eb(22, b(5, 87), b(1, 88)), eb(24, b(5, 41), b(5, 42)), eb(30, b(5, 24), b(7, 25)), eb(24, b(11, 12), b(7, 13))),
	newVersion(16, []int{6, 26, 50, 74}, eb(24, b(5, 98), b(1, 99)), eb(28, b(7, 45), b(3, 46)), eb(24, b(15, 19), b(2, 20)), eb(30, b(3, 15), b(13, 16))),
	newVersion(17, []int{6, 30, 54, 78}, eb(28, b(1, 107), b(5, 108)), eb(28, b(10, 46), b(1, 47)), eb(28, b(1, 22), b(15, 23)), eb(28, b(2, 14), b(17, 15))),
	newVersion(18, []int{6, 30, 56, 82}, eb(30, b(5, 120), b(1, 121)), eb(26, b(9, 43), b(4, 44)), eb(28, b(17, 22), b(1, 23)), eb(28, b(2, 14), b(19, 15))),
	newVersion(19, []int{6, 30, 58, 86}, eb(28, b(3, 113), b(4, 114)), eb(26, b(3, 44), b(11, 45)), eb(26, b(17, 21), b(4, 22)), eb(26, b(9, 13), b(16, 14))),
	newVersion(20, []int{6, 34, 62, 90}, eb(28, b(3, 107), b(5, 108)), eb(26, b(3, 41), b(13, 42)), eb(30, b(15, 24), b(5, 25)), eb(28, b(15, 15), b(10, 16))),
	newVersion(21, []int{6, 28, 50, 72, 94}, eb(28, b(4, 116), b(4, 117)), eb(26, b(17, 42)), eb(28, b(17, 22), b(6, 23)), eb(30, b(19, 16), b(6, 17))),
	newVersion(22, []int{6, 26, 50, 74, 98}, eb(28, b(2, 111), b(7, 112)), eb(28, b(17, 46)), eb(30, b(7, 24), b(16, 25)), eb(24, b(34, 13))),
	newVersion(23, []int{6, 30, 54, 78, 102}, eb(30, b(4, 121), b(5, 122)), eb(28, b(4, 47), b(14, 48)), eb(30, b(11, 24), b(14, 25)), eb(30, b(16, 15), b(14, 16))),
	newVersion(24, []int{6, 28, 54, 80, 106}, eb(30, b(6, 117), b(4, 118)), eb(28, b(6, 45), b(14, 46)), eb(30, b(11, 24), b(16, 25)), eb(30, b(30, 16), b(2, 17))),
	newVersion(25, []int{6, 32, 58, 84, 110}, eb(26, b(8, 106), b(4, 107)), eb(28, b(8, 47), b(13, 48)), eb(30, b(7, 24), b(22, 25)), eb(30, b(22, 15), b(13, 16))),
	newVersion(26, []int{6, 30, 58, 86, 114}, eb(28, b(10, 114), b(2, 115)), eb(28, b(19, 46), b(4, 47)), eb(28, b(28, 22), b(6, 23)), eb(30, b(33, 16), b(4, 17))),
	newVersion(27, []int{6, 34, 62, 90, 118}, eb(30, b(8, 122), b(4, 123)), eb(28, b(22, 45), b(3, 46)), eb(30, b(8, 23), b(26, 24)), eb(30, b(12, 15), b(28, 16))),
	newVersion(28, []int{6, 26, 50, 74, 98, 122}, eb(30, b(3, 117), b(10, 118)), eb(28, b(3, 45), b(23, 46)), eb(30, b(4, 24), b(31, 25)), eb(30, b(11, 15), b(31, 16))),
	newVersion(29, []int{6, 30, 54, 78, 102, 126}, eb(30, b(7, 116), b(7, 117)), eb(28, b(21, 45), b(7, 46)), eb(30, b(1, 23), b(37, 24)), eb(30, b(19, 15), b(26, 16))),
	newVersion(30, []int{6, 26, 52, 78, 104, 130}, eb(30, b(5, 115), b(10, 116)), eb(28, b(19, 47), b(10, 48)), eb(30, b(15, 24), b(25, 25)), eb(30, b(23, 15), b(25, 16))),
	newVersion(31, []int{6, 30, 56, 82, 108, 134}, eb(30, b(13, 115), b(3, 116)), eb(28, b(2, 46), b(29, 47)), eb(30, b(42, 24), b(1, 25)), eb(30, b(23, 15), b(28, 16))),
	newVersion(32, []int{6, 34, 60, 86, 112, 138}, eb(30, b(17, 115)), eb(28, b(10, 46), b(23, 47)), eb(30, b(10, 24), b(35, 25)), eb(30, b(19, 15), b(35, 16))),
	newVersion(33, []int{6, 30, 58, 86, 114, 142}, eb(30, b(17, 115), b(1, 116)), eb(28, b(14, 46), b(21, 47)), eb(30, b(29, 24), b(19, 25)), eb(30, b(11, 15), b(46, 16))),
	newVersion(34, []int{6, 34, 62, 90, 118, 146}, eb(30, b(13, 115), b(6, 116)), eb(28, b(14, 46), b(23, 47)), eb(30, b(44, 24), b(7, 25)), eb(30, b(59, 16), b(1, 17))),
	newVersion(35, []int{6, 30, 54, 78, 102, 126, 150}, eb(30, b(12, 121), b(7, 122)), eb(28, b(12, 47), b(26, 48)), eb(30, b(39, 24), b(14, 25)), eb(30, b(22, 15), b(41, 16))),
	newVersion(36, []int{6, 24, 50, 76, 102, 128, 154}, eb(30, b(6, 121), b(14, 122)), eb(28, b(6, 47), b(34, 48)), eb(30, b(46, 24), b(10, 25)), eb(30, b(2, 15), b(64, 16))),
	newVersion(37, []int{6, 28, 54, 80, 106, 132, 158}, eb(30, b(17, 122), b(4, 123)), eb(28, b(29, 46), b(14, 47)), eb(30, b(49, 24), b(10, 25)), eb(30, b(24, 15), b(46, 16))),
	newVersion(38, []int{6, 32, 58, 84, 110, 136, 162}, eb(30, b(4, 122), b(18, 123)), eb(28, b(13, 46), b(32, 47)), eb(30, b(48, 24), b(14, 25)), eb(30, b(42, 15), b(32, 16))),
	newVersion(39, []int{6, 26, 54, 82, 110, 138, 166}, eb(30, b(20, 117), b(4, 118)), eb(28, b(40, 47), b(7, 48)), eb(30, b(43, 24), b(22, 25)), eb(30, b(10, 15), b(67, 16))),
	newVersion(40, []int{6, 30, 58, 86, 114, 142, 170}, eb(30, b(19, 118), b(6, 119)), eb(28, b(18, 47), b(31, 48)), eb(30, b(34, 24), b(34, 25)), eb(30, b(20, 15), b(61, 16))),
}
