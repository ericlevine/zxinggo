package decoder

import "github.com/ericlevine/zxinggo/bitutil"

// DataMaskFunc is a function that returns true if a bit at position (i, j)
// should be masked.
type DataMaskFunc func(i, j int) bool

// DataMasks contains the 8 QR code data mask patterns.
var DataMasks = [8]DataMaskFunc{
	func(i, j int) bool { return (i+j)&0x01 == 0 },                   // 000
	func(i, j int) bool { return i&0x01 == 0 },                       // 001
	func(i, j int) bool { return j%3 == 0 },                          // 010
	func(i, j int) bool { return (i+j)%3 == 0 },                      // 011
	func(i, j int) bool { return ((i/2)+(j/3))&0x01 == 0 },           // 100
	func(i, j int) bool { return (i*j)%6 == 0 },                      // 101
	func(i, j int) bool { return ((i * j) % 6) < 3 },                 // 110
	func(i, j int) bool { return ((i + j + ((i * j) % 3)) & 0x01) == 0 }, // 111
}

// UnmaskBitMatrix applies data mask unmasking to a BitMatrix.
func UnmaskBitMatrix(bits *bitutil.BitMatrix, dimension int, maskIndex int) {
	mask := DataMasks[maskIndex]
	for i := 0; i < dimension; i++ {
		for j := 0; j < dimension; j++ {
			if mask(i, j) {
				bits.Flip(j, i)
			}
		}
	}
}
