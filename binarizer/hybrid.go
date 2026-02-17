package binarizer

import (
	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

const (
	blockSizePower  = 3
	blockSize       = 1 << blockSizePower
	blockSizeMask   = blockSize - 1
	minimumDimension = blockSize * 5
	minDynamicRange = 24
)

// Hybrid implements a local thresholding algorithm. It is more effective than
// GlobalHistogram for images with shadows and gradients.
type Hybrid struct {
	GlobalHistogram
	matrix *bitutil.BitMatrix
}

// NewHybrid creates a new Hybrid binarizer.
func NewHybrid(source zxinggo.LuminanceSource) *Hybrid {
	return &Hybrid{
		GlobalHistogram: *NewGlobalHistogram(source),
	}
}

// BlackMatrix returns the binarized matrix using local thresholding.
func (h *Hybrid) BlackMatrix() (*bitutil.BitMatrix, error) {
	if h.matrix != nil {
		return h.matrix, nil
	}
	source := h.LuminanceSource()
	width := source.Width()
	height := source.Height()

	if width >= minimumDimension && height >= minimumDimension {
		luminances := source.Matrix()
		subWidth := width >> blockSizePower
		if (width & blockSizeMask) != 0 {
			subWidth++
		}
		subHeight := height >> blockSizePower
		if (height & blockSizeMask) != 0 {
			subHeight++
		}
		blackPoints := calculateBlackPoints(luminances, subWidth, subHeight, width, height)

		newMatrix := bitutil.NewBitMatrixWithSize(width, height)
		calculateThresholdForBlock(luminances, subWidth, subHeight, width, height, blackPoints, newMatrix)
		h.matrix = newMatrix
	} else {
		m, err := h.GlobalHistogram.BlackMatrix()
		if err != nil {
			return nil, err
		}
		h.matrix = m
	}
	return h.matrix, nil
}

func calculateThresholdForBlock(luminances []byte, subWidth, subHeight, width, height int,
	blackPoints [][]int, matrix *bitutil.BitMatrix) {
	maxYOffset := height - blockSize
	maxXOffset := width - blockSize
	for y := 0; y < subHeight; y++ {
		yoffset := y << blockSizePower
		if yoffset > maxYOffset {
			yoffset = maxYOffset
		}
		top := cap3(y, subHeight-3)
		for x := 0; x < subWidth; x++ {
			xoffset := x << blockSizePower
			if xoffset > maxXOffset {
				xoffset = maxXOffset
			}
			left := cap3(x, subWidth-3)
			sum := 0
			for z := -2; z <= 2; z++ {
				blackRow := blackPoints[top+z]
				sum += blackRow[left-2] + blackRow[left-1] + blackRow[left] + blackRow[left+1] + blackRow[left+2]
			}
			average := sum / 25
			thresholdBlock(luminances, xoffset, yoffset, average, width, matrix)
		}
	}
}

func cap3(value, max int) int {
	if value < 2 {
		return 2
	}
	if value > max {
		return max
	}
	return value
}

func thresholdBlock(luminances []byte, xoffset, yoffset, threshold, stride int, matrix *bitutil.BitMatrix) {
	for y, offset := 0, yoffset*stride+xoffset; y < blockSize; y, offset = y+1, offset+stride {
		for x := 0; x < blockSize; x++ {
			if int(luminances[offset+x]&0xFF) <= threshold {
				matrix.Set(xoffset+x, yoffset+y)
			}
		}
	}
}

func calculateBlackPoints(luminances []byte, subWidth, subHeight, width, height int) [][]int {
	maxYOffset := height - blockSize
	maxXOffset := width - blockSize
	blackPoints := make([][]int, subHeight)
	for i := range blackPoints {
		blackPoints[i] = make([]int, subWidth)
	}

	for y := 0; y < subHeight; y++ {
		yoffset := y << blockSizePower
		if yoffset > maxYOffset {
			yoffset = maxYOffset
		}
		for x := 0; x < subWidth; x++ {
			xoffset := x << blockSizePower
			if xoffset > maxXOffset {
				xoffset = maxXOffset
			}
			sum := 0
			mn := 0xFF
			mx := 0
			for yy, offset := 0, yoffset*width+xoffset; yy < blockSize; yy, offset = yy+1, offset+width {
				for xx := 0; xx < blockSize; xx++ {
					pixel := int(luminances[offset+xx] & 0xFF)
					sum += pixel
					if pixel < mn {
						mn = pixel
					}
					if pixel > mx {
						mx = pixel
					}
				}
				if mx-mn > minDynamicRange {
					for yy, offset = yy+1, offset+width; yy < blockSize; yy, offset = yy+1, offset+width {
						for xx := 0; xx < blockSize; xx++ {
							sum += int(luminances[offset+xx] & 0xFF)
						}
					}
				}
			}

			average := sum >> (blockSizePower * 2)
			if mx-mn <= minDynamicRange {
				average = mn / 2
				if y > 0 && x > 0 {
					averageNeighborBlackPoint :=
						(blackPoints[y-1][x] + 2*blackPoints[y][x-1] + blackPoints[y-1][x-1]) / 4
					if mn < averageNeighborBlackPoint {
						average = averageNeighborBlackPoint
					}
				}
			}
			blackPoints[y][x] = average
		}
	}
	return blackPoints
}
