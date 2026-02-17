// Package bitutil provides bit manipulation utilities for barcode processing.
package bitutil

import (
	"math/bits"
	"strings"
)

const loadFactor = 0.75

// BitArray is a simple, fast array of bits represented compactly by an array
// of uint32 values internally.
type BitArray struct {
	bits []uint32
	size int
}

// NewBitArray creates a new BitArray with the given size.
func NewBitArray(size int) *BitArray {
	if size <= 0 {
		return &BitArray{}
	}
	return &BitArray{
		bits: makeArray(size),
		size: size,
	}
}

// NewBitArrayFromBits creates a BitArray from existing data (for testing).
func NewBitArrayFromBits(b []uint32, size int) *BitArray {
	return &BitArray{bits: b, size: size}
}

// Size returns the number of bits in the array.
func (ba *BitArray) Size() int {
	return ba.size
}

// SizeInBytes returns the number of bytes needed to hold the bits.
func (ba *BitArray) SizeInBytes() int {
	return (ba.size + 7) / 8
}

func (ba *BitArray) ensureCapacity(newSize int) {
	if newSize > len(ba.bits)*32 {
		newBits := makeArray(int(float64(newSize) / loadFactor))
		copy(newBits, ba.bits)
		ba.bits = newBits
	}
}

// Get returns true if bit i is set.
func (ba *BitArray) Get(i int) bool {
	return (ba.bits[i/32] & (1 << uint(i&0x1F))) != 0
}

// Set sets bit i.
func (ba *BitArray) Set(i int) {
	ba.bits[i/32] |= 1 << uint(i&0x1F)
}

// Flip flips bit i.
func (ba *BitArray) Flip(i int) {
	ba.bits[i/32] ^= 1 << uint(i&0x1F)
}

// GetNextSet returns the index of the first set bit starting from the given
// index, or size if none are set.
func (ba *BitArray) GetNextSet(from int) int {
	if from >= ba.size {
		return ba.size
	}
	bitsOffset := from / 32
	currentBits := ba.bits[bitsOffset]
	// mask off lesser bits
	currentBits &= ^uint32(0) << uint(from&0x1F)
	for currentBits == 0 {
		bitsOffset++
		if bitsOffset == len(ba.bits) {
			return ba.size
		}
		currentBits = ba.bits[bitsOffset]
	}
	result := bitsOffset*32 + bits.TrailingZeros32(currentBits)
	if result > ba.size {
		return ba.size
	}
	return result
}

// GetNextUnset returns the index of the first unset bit starting from the
// given index, or size if none are unset.
func (ba *BitArray) GetNextUnset(from int) int {
	if from >= ba.size {
		return ba.size
	}
	bitsOffset := from / 32
	currentBits := ^ba.bits[bitsOffset]
	// mask off lesser bits
	currentBits &= ^uint32(0) << uint(from&0x1F)
	for currentBits == 0 {
		bitsOffset++
		if bitsOffset == len(ba.bits) {
			return ba.size
		}
		currentBits = ^ba.bits[bitsOffset]
	}
	result := bitsOffset*32 + bits.TrailingZeros32(currentBits)
	if result > ba.size {
		return ba.size
	}
	return result
}

// SetBulk sets a block of 32 bits starting at bit i.
func (ba *BitArray) SetBulk(i int, newBits uint32) {
	ba.bits[i/32] = newBits
}

// SetRange sets a range of bits [start, end).
func (ba *BitArray) SetRange(start, end int) {
	if end < start || start < 0 || end > ba.size {
		panic("bitarray: invalid range")
	}
	if end == start {
		return
	}
	end-- // treat as last set bit (inclusive)
	firstInt := start / 32
	lastInt := end / 32
	for i := firstInt; i <= lastInt; i++ {
		firstBit := 0
		if i > firstInt {
			firstBit = 0
		} else {
			firstBit = start & 0x1F
		}
		lastBit := 31
		if i < lastInt {
			lastBit = 31
		} else {
			lastBit = end & 0x1F
		}
		mask := uint32((2 << uint(lastBit)) - (1 << uint(firstBit)))
		ba.bits[i] |= mask
	}
}

// Clear clears all bits.
func (ba *BitArray) Clear() {
	for i := range ba.bits {
		ba.bits[i] = 0
	}
}

// IsRange checks if all bits in [start, end) have the given value.
func (ba *BitArray) IsRange(start, end int, value bool) bool {
	if end < start || start < 0 || end > ba.size {
		panic("bitarray: invalid range")
	}
	if end == start {
		return true
	}
	end--
	firstInt := start / 32
	lastInt := end / 32
	for i := firstInt; i <= lastInt; i++ {
		firstBit := 0
		if i == firstInt {
			firstBit = start & 0x1F
		}
		lastBit := 31
		if i == lastInt {
			lastBit = end & 0x1F
		}
		mask := uint32((2 << uint(lastBit)) - (1 << uint(firstBit)))
		if value {
			if (ba.bits[i] & mask) != mask {
				return false
			}
		} else {
			if (ba.bits[i] & mask) != 0 {
				return false
			}
		}
	}
	return true
}

// AppendBit appends a single bit.
func (ba *BitArray) AppendBit(bit bool) {
	ba.ensureCapacity(ba.size + 1)
	if bit {
		ba.bits[ba.size/32] |= 1 << uint(ba.size&0x1F)
	}
	ba.size++
}

// AppendBits appends the least-significant numBits bits of value, from most
// significant to least significant.
func (ba *BitArray) AppendBits(value uint32, numBits int) {
	if numBits < 0 || numBits > 32 {
		panic("bitarray: numBits must be between 0 and 32")
	}
	nextSize := ba.size
	ba.ensureCapacity(nextSize + numBits)
	for numBitsLeft := numBits - 1; numBitsLeft >= 0; numBitsLeft-- {
		if (value & (1 << uint(numBitsLeft))) != 0 {
			ba.bits[nextSize/32] |= 1 << uint(nextSize&0x1F)
		}
		nextSize++
	}
	ba.size = nextSize
}

// AppendBitArray appends another BitArray to this one.
func (ba *BitArray) AppendBitArray(other *BitArray) {
	otherSize := other.size
	ba.ensureCapacity(ba.size + otherSize)
	for i := 0; i < otherSize; i++ {
		ba.AppendBit(other.Get(i))
	}
}

// Xor performs XOR with another BitArray.
func (ba *BitArray) Xor(other *BitArray) {
	if ba.size != other.size {
		panic("bitarray: sizes don't match")
	}
	for i := range ba.bits {
		ba.bits[i] ^= other.bits[i]
	}
}

// ToBytes writes bits to a byte slice (most-significant byte first within each byte).
func (ba *BitArray) ToBytes(bitOffset int, array []byte, offset, numBytes int) {
	for i := 0; i < numBytes; i++ {
		theByte := byte(0)
		for j := 0; j < 8; j++ {
			if ba.Get(bitOffset) {
				theByte |= 1 << uint(7-j)
			}
			bitOffset++
		}
		array[offset+i] = theByte
	}
}

// BitData returns the underlying uint32 slice.
func (ba *BitArray) BitData() []uint32 {
	return ba.bits
}

// Reverse reverses all bits in the array.
func (ba *BitArray) Reverse() {
	newBits := make([]uint32, len(ba.bits))
	ln := (ba.size - 1) / 32
	oldBitsLen := ln + 1
	for i := 0; i < oldBitsLen; i++ {
		newBits[ln-i] = bits.Reverse32(ba.bits[i])
	}
	if ba.size != oldBitsLen*32 {
		leftOffset := uint(oldBitsLen*32 - ba.size)
		currentInt := newBits[0] >> leftOffset
		for i := 1; i < oldBitsLen; i++ {
			nextInt := newBits[i]
			currentInt |= nextInt << (32 - leftOffset)
			newBits[i-1] = currentInt
			currentInt = nextInt >> leftOffset
		}
		newBits[oldBitsLen-1] = currentInt
	}
	ba.bits = newBits
}

// Clone returns a copy of this BitArray.
func (ba *BitArray) Clone() *BitArray {
	b := make([]uint32, len(ba.bits))
	copy(b, ba.bits)
	return &BitArray{bits: b, size: ba.size}
}

// String returns a string representation using 'X' for set and '.' for unset.
func (ba *BitArray) String() string {
	var sb strings.Builder
	sb.Grow(ba.size + ba.size/8 + 1)
	for i := 0; i < ba.size; i++ {
		if i&0x07 == 0 {
			sb.WriteByte(' ')
		}
		if ba.Get(i) {
			sb.WriteByte('X')
		} else {
			sb.WriteByte('.')
		}
	}
	return sb.String()
}

func makeArray(size int) []uint32 {
	return make([]uint32, (size+31)/32)
}
