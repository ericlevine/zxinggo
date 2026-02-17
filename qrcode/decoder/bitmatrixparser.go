package decoder

import (
	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// BitMatrixParser parses a BitMatrix representing a QR code.
type BitMatrixParser struct {
	bitMatrix       *bitutil.BitMatrix
	parsedVersion   *Version
	parsedFormatInfo *FormatInformation
	mirror          bool
}

// NewBitMatrixParser creates a new parser for the given BitMatrix.
func NewBitMatrixParser(bitMatrix *bitutil.BitMatrix) (*BitMatrixParser, error) {
	dimension := bitMatrix.Height()
	if dimension < 21 || (dimension&0x03) != 1 {
		return nil, zxinggo.ErrFormat
	}
	return &BitMatrixParser{bitMatrix: bitMatrix}, nil
}

// ReadFormatInformation reads format info from one of its two locations.
func (p *BitMatrixParser) ReadFormatInformation() (*FormatInformation, error) {
	if p.parsedFormatInfo != nil {
		return p.parsedFormatInfo, nil
	}

	// Read top-left format info bits
	formatInfoBits1 := 0
	for i := 0; i < 6; i++ {
		formatInfoBits1 = p.copyBit(i, 8, formatInfoBits1)
	}
	formatInfoBits1 = p.copyBit(7, 8, formatInfoBits1)
	formatInfoBits1 = p.copyBit(8, 8, formatInfoBits1)
	formatInfoBits1 = p.copyBit(8, 7, formatInfoBits1)
	for j := 5; j >= 0; j-- {
		formatInfoBits1 = p.copyBit(8, j, formatInfoBits1)
	}

	// Read top-right/bottom-left pattern
	dimension := p.bitMatrix.Height()
	formatInfoBits2 := 0
	jMin := dimension - 7
	for j := dimension - 1; j >= jMin; j-- {
		formatInfoBits2 = p.copyBit(8, j, formatInfoBits2)
	}
	for i := dimension - 8; i < dimension; i++ {
		formatInfoBits2 = p.copyBit(i, 8, formatInfoBits2)
	}

	p.parsedFormatInfo = DecodeFormatInformation(formatInfoBits1, formatInfoBits2)
	if p.parsedFormatInfo != nil {
		return p.parsedFormatInfo, nil
	}
	return nil, zxinggo.ErrFormat
}

// ReadVersion reads version information from the QR code.
func (p *BitMatrixParser) ReadVersion() (*Version, error) {
	if p.parsedVersion != nil {
		return p.parsedVersion, nil
	}

	dimension := p.bitMatrix.Height()
	provisionalVersion := (dimension - 17) / 4
	if provisionalVersion <= 6 {
		return GetVersionForNumber(provisionalVersion)
	}

	// Read top-right version info: 3 wide by 6 tall
	versionBits := 0
	ijMin := dimension - 11
	for j := 5; j >= 0; j-- {
		for i := dimension - 9; i >= ijMin; i-- {
			versionBits = p.copyBit(i, j, versionBits)
		}
	}

	theParsedVersion := DecodeVersionInformation(versionBits)
	if theParsedVersion != nil && theParsedVersion.DimensionForVersion() == dimension {
		p.parsedVersion = theParsedVersion
		return theParsedVersion, nil
	}

	// Try bottom-left: 6 wide by 3 tall
	versionBits = 0
	for i := 5; i >= 0; i-- {
		for j := dimension - 9; j >= ijMin; j-- {
			versionBits = p.copyBit(i, j, versionBits)
		}
	}

	theParsedVersion = DecodeVersionInformation(versionBits)
	if theParsedVersion != nil && theParsedVersion.DimensionForVersion() == dimension {
		p.parsedVersion = theParsedVersion
		return theParsedVersion, nil
	}
	return nil, zxinggo.ErrFormat
}

func (p *BitMatrixParser) copyBit(i, j, versionBits int) int {
	var bit bool
	if p.mirror {
		bit = p.bitMatrix.Get(j, i)
	} else {
		bit = p.bitMatrix.Get(i, j)
	}
	if bit {
		return (versionBits << 1) | 0x1
	}
	return versionBits << 1
}

// ReadCodewords reads the codewords from the bit matrix.
func (p *BitMatrixParser) ReadCodewords() ([]byte, error) {
	formatInfo, err := p.ReadFormatInformation()
	if err != nil {
		return nil, err
	}
	version, err := p.ReadVersion()
	if err != nil {
		return nil, err
	}

	// Unmask the data
	UnmaskBitMatrix(p.bitMatrix, p.bitMatrix.Height(), int(formatInfo.DataMask))

	functionPattern := version.BuildFunctionPattern()

	readingUp := true
	result := make([]byte, version.TotalCodewords)
	resultOffset := 0
	currentByte := 0
	bitsRead := 0
	dimension := p.bitMatrix.Height()

	for j := dimension - 1; j > 0; j -= 2 {
		if j == 6 {
			j--
		}
		for count := 0; count < dimension; count++ {
			i := count
			if readingUp {
				i = dimension - 1 - count
			}
			for col := 0; col < 2; col++ {
				if !functionPattern.Get(j-col, i) {
					bitsRead++
					currentByte <<= 1
					if p.bitMatrix.Get(j-col, i) {
						currentByte |= 1
					}
					if bitsRead == 8 {
						result[resultOffset] = byte(currentByte)
						resultOffset++
						bitsRead = 0
						currentByte = 0
					}
				}
			}
		}
		readingUp = !readingUp
	}

	if resultOffset != version.TotalCodewords {
		return nil, zxinggo.ErrFormat
	}
	return result, nil
}

// Remask re-applies the data mask (reversing the unmask).
func (p *BitMatrixParser) Remask() {
	if p.parsedFormatInfo == nil {
		return
	}
	UnmaskBitMatrix(p.bitMatrix, p.bitMatrix.Height(), int(p.parsedFormatInfo.DataMask))
}

// SetMirror prepares the parser for mirrored reading.
func (p *BitMatrixParser) SetMirror(mirror bool) {
	p.parsedVersion = nil
	p.parsedFormatInfo = nil
	p.mirror = mirror
}

// Mirror mirrors the bit matrix for a second reading attempt.
func (p *BitMatrixParser) Mirror() {
	for x := 0; x < p.bitMatrix.Width(); x++ {
		for y := x + 1; y < p.bitMatrix.Height(); y++ {
			if p.bitMatrix.Get(x, y) != p.bitMatrix.Get(y, x) {
				p.bitMatrix.Flip(y, x)
				p.bitMatrix.Flip(x, y)
			}
		}
	}
}
