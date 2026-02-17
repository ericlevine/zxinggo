package bitutil

// BitSource reads bits from a byte sequence where the number of bits read
// is not necessarily a multiple of 8.
type BitSource struct {
	bytes      []byte
	byteOffset int
	bitOffset  int
}

// NewBitSource creates a new BitSource from a byte slice.
// Bits are read from the first byte first, from most-significant to least-significant.
func NewBitSource(bytes []byte) *BitSource {
	return &BitSource{bytes: bytes}
}

// BitOffset returns the index of the next bit within the current byte.
func (bs *BitSource) BitOffset() int {
	return bs.bitOffset
}

// ByteOffset returns the index of the next byte to be read.
func (bs *BitSource) ByteOffset() int {
	return bs.byteOffset
}

// ReadBits reads numBits bits and returns them as the least-significant bits of an int.
func (bs *BitSource) ReadBits(numBits int) (int, error) {
	if numBits < 1 || numBits > 32 || numBits > bs.Available() {
		return 0, &BitSourceError{NumBits: numBits}
	}

	result := 0

	// First, read remainder from current byte
	if bs.bitOffset > 0 {
		bitsLeft := 8 - bs.bitOffset
		toRead := numBits
		if toRead > bitsLeft {
			toRead = bitsLeft
		}
		bitsToNotRead := bitsLeft - toRead
		mask := (0xFF >> uint(8-toRead)) << uint(bitsToNotRead)
		result = (int(bs.bytes[bs.byteOffset]) & mask) >> uint(bitsToNotRead)
		numBits -= toRead
		bs.bitOffset += toRead
		if bs.bitOffset == 8 {
			bs.bitOffset = 0
			bs.byteOffset++
		}
	}

	// Next read whole bytes
	if numBits > 0 {
		for numBits >= 8 {
			result = (result << 8) | int(bs.bytes[bs.byteOffset]&0xFF)
			bs.byteOffset++
			numBits -= 8
		}

		// Finally read a partial byte
		if numBits > 0 {
			bitsToNotRead := 8 - numBits
			mask := (0xFF >> uint(bitsToNotRead)) << uint(bitsToNotRead)
			result = (result << uint(numBits)) | ((int(bs.bytes[bs.byteOffset]) & mask) >> uint(bitsToNotRead))
			bs.bitOffset += numBits
		}
	}

	return result, nil
}

// Available returns the number of bits that can still be read.
func (bs *BitSource) Available() int {
	return 8*(len(bs.bytes)-bs.byteOffset) - bs.bitOffset
}

// BitSourceError is returned when an invalid number of bits is requested.
type BitSourceError struct {
	NumBits int
}

func (e *BitSourceError) Error() string {
	return "bitsource: invalid number of bits"
}
