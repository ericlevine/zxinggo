// Package qrcode provides QR code reading and writing.
package qrcode

import (
	"fmt"
	"math"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
	"github.com/ericlevine/zxinggo/qrcode/decoder"
	"github.com/ericlevine/zxinggo/qrcode/detector"
)

// Reader decodes QR codes from binary images.
type Reader struct {
	dec *decoder.Decoder
}

// NewReader creates a new QR code Reader.
func NewReader() *Reader {
	return &Reader{
		dec: decoder.NewDecoder(),
	}
}

// Decode locates and decodes a QR code in the given image.
func (r *Reader) Decode(image *zxinggo.BinaryBitmap, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	if opts == nil {
		opts = &zxinggo.DecodeOptions{}
	}

	matrix, err := image.BlackMatrix()
	if err != nil {
		return nil, err
	}

	if opts.PureBarcode {
		bits, err := extractPureBits(matrix)
		if err != nil {
			return nil, err
		}
		dr, err := r.dec.Decode(bits, opts.CharacterSet)
		if err != nil {
			return nil, err
		}

		result := zxinggo.NewResult(dr.Text, dr.RawBytes, nil, zxinggo.FormatQRCode)
		populateMetadata(result, dr.ByteSegments, dr.ECLevel,
			dr.HasStructuredAppend(), dr.StructuredAppendSequenceNumber,
			dr.StructuredAppendParity, dr.ErrorsCorrected, dr.SymbologyModifier)
		return result, nil
	}

	det := detector.NewDetector(matrix)
	detectorResult, err := det.Detect(false)
	if err != nil {
		return nil, err
	}
	dr, err := r.dec.Decode(detectorResult.Bits, opts.CharacterSet)
	if err != nil {
		return nil, err
	}

	points := make([]zxinggo.ResultPoint, len(detectorResult.Points))
	for i, p := range detectorResult.Points {
		points[i] = zxinggo.ResultPoint{X: p.X, Y: p.Y}
	}

	result := zxinggo.NewResult(dr.Text, dr.RawBytes, points, zxinggo.FormatQRCode)
	populateMetadata(result, dr.ByteSegments, dr.ECLevel,
		dr.HasStructuredAppend(), dr.StructuredAppendSequenceNumber,
		dr.StructuredAppendParity, dr.ErrorsCorrected, dr.SymbologyModifier)
	return result, nil
}

// Reset resets internal state.
func (r *Reader) Reset() {
	// nothing to reset
}

func populateMetadata(result *zxinggo.Result, byteSegments [][]byte, ecLevel string,
	hasStructuredAppend bool, saSequence, saParity, errorsCorrected, symbologyModifier int) {
	if byteSegments != nil {
		result.PutMetadata(zxinggo.MetadataByteSegments, byteSegments)
	}
	if ecLevel != "" {
		result.PutMetadata(zxinggo.MetadataErrorCorrectionLevel, ecLevel)
	}
	if hasStructuredAppend {
		result.PutMetadata(zxinggo.MetadataStructuredAppendSequence, saSequence)
		result.PutMetadata(zxinggo.MetadataStructuredAppendParity, saParity)
	}
	result.PutMetadata(zxinggo.MetadataErrorsCorrected, errorsCorrected)
	result.PutMetadata(zxinggo.MetadataSymbologyIdentifier, fmt.Sprintf("]Q%d", symbologyModifier))
}

// extractPureBits extracts a QR code from a "pure" image — one that contains
// only the unrotated, unskewed barcode with some white border.
func extractPureBits(image *bitutil.BitMatrix) (*bitutil.BitMatrix, error) {
	leftTopBlack := image.TopLeftOnBit()
	rightBottomBlack := image.BottomRightOnBit()
	if leftTopBlack == nil || rightBottomBlack == nil {
		return nil, zxinggo.ErrNotFound
	}

	moduleSize, err := moduleSizePure(leftTopBlack, image)
	if err != nil {
		return nil, err
	}

	top := leftTopBlack[1]
	bottom := rightBottomBlack[1]
	left := leftTopBlack[0]
	right := rightBottomBlack[0]

	if left >= right || top >= bottom {
		return nil, zxinggo.ErrNotFound
	}

	if bottom-top != right-left {
		right = left + (bottom - top)
		if right >= image.Width() {
			return nil, zxinggo.ErrNotFound
		}
	}

	matrixWidth := int(math.Round(float64(right-left+1) / moduleSize))
	matrixHeight := int(math.Round(float64(bottom-top+1) / moduleSize))
	if matrixWidth <= 0 || matrixHeight <= 0 {
		return nil, zxinggo.ErrNotFound
	}
	if matrixHeight != matrixWidth {
		return nil, zxinggo.ErrNotFound
	}

	nudge := int(moduleSize / 2.0)
	top += nudge
	left += nudge

	nudgedTooFarRight := left + int(float64(matrixWidth-1)*moduleSize) - right
	if nudgedTooFarRight > 0 {
		if nudgedTooFarRight > nudge {
			return nil, zxinggo.ErrNotFound
		}
		left -= nudgedTooFarRight
	}
	nudgedTooFarDown := top + int(float64(matrixHeight-1)*moduleSize) - bottom
	if nudgedTooFarDown > 0 {
		if nudgedTooFarDown > nudge {
			return nil, zxinggo.ErrNotFound
		}
		top -= nudgedTooFarDown
	}

	bits := bitutil.NewBitMatrix(matrixWidth)
	for y := 0; y < matrixHeight; y++ {
		iOffset := top + int(float64(y)*moduleSize)
		for x := 0; x < matrixWidth; x++ {
			if image.Get(left+int(float64(x)*moduleSize), iOffset) {
				bits.Set(x, y)
			}
		}
	}
	return bits, nil
}

func moduleSizePure(leftTopBlack []int, image *bitutil.BitMatrix) (float64, error) {
	height := image.Height()
	width := image.Width()
	x := leftTopBlack[0]
	y := leftTopBlack[1]
	inBlack := true
	transitions := 0
	for x < width && y < height {
		if inBlack != image.Get(x, y) {
			transitions++
			if transitions == 5 {
				break
			}
			inBlack = !inBlack
		}
		x++
		y++
	}
	if x == width || y == height {
		return 0, zxinggo.ErrNotFound
	}
	return float64(x-leftTopBlack[0]) / 7.0, nil
}
