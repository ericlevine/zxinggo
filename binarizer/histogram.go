// Package binarizer provides implementations for converting luminance data to binary.
package binarizer

import (
	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

const (
	luminanceBits    = 5
	luminanceShift   = 8 - luminanceBits
	luminanceBuckets = 1 << luminanceBits
)

// GlobalHistogram uses a global histogram approach to binarize luminance data.
// Suitable for lower-end devices; higher-end should use Hybrid.
type GlobalHistogram struct {
	source     zxinggo.LuminanceSource
	luminances []byte
	buckets    [luminanceBuckets]int
}

// NewGlobalHistogram creates a new GlobalHistogram binarizer.
func NewGlobalHistogram(source zxinggo.LuminanceSource) *GlobalHistogram {
	return &GlobalHistogram{source: source}
}

// LuminanceSource returns the underlying source.
func (g *GlobalHistogram) LuminanceSource() zxinggo.LuminanceSource {
	return g.source
}

// Width returns the image width.
func (g *GlobalHistogram) Width() int { return g.source.Width() }

// Height returns the image height.
func (g *GlobalHistogram) Height() int { return g.source.Height() }

// BlackRow returns a row binarized using the global histogram approach with sharpening.
func (g *GlobalHistogram) BlackRow(y int, row *bitutil.BitArray) (*bitutil.BitArray, error) {
	width := g.source.Width()
	if row == nil || row.Size() < width {
		row = bitutil.NewBitArray(width)
	} else {
		row.Clear()
	}

	g.initArrays(width)
	localLuminances := g.source.Row(y, g.luminances)
	for x := 0; x < width; x++ {
		g.buckets[int(localLuminances[x]&0xff)>>luminanceShift]++
	}
	blackPoint, err := estimateBlackPoint(g.buckets[:])
	if err != nil {
		return nil, err
	}

	if width < 3 {
		for x := 0; x < width; x++ {
			if int(localLuminances[x]&0xff) < blackPoint {
				row.Set(x)
			}
		}
	} else {
		left := int(localLuminances[0] & 0xff)
		center := int(localLuminances[1] & 0xff)
		for x := 1; x < width-1; x++ {
			right := int(localLuminances[x+1] & 0xff)
			if ((center*4)-left-right)/2 < blackPoint {
				row.Set(x)
			}
			left = center
			center = right
		}
	}
	return row, nil
}

// BlackMatrix returns the full binarized matrix.
func (g *GlobalHistogram) BlackMatrix() (*bitutil.BitMatrix, error) {
	width := g.source.Width()
	height := g.source.Height()
	matrix := bitutil.NewBitMatrixWithSize(width, height)

	g.initArrays(width)
	for y := 1; y < 5; y++ {
		row := height * y / 5
		localLuminances := g.source.Row(row, g.luminances)
		right := (width * 4) / 5
		for x := width / 5; x < right; x++ {
			g.buckets[int(localLuminances[x]&0xff)>>luminanceShift]++
		}
	}
	blackPoint, err := estimateBlackPoint(g.buckets[:])
	if err != nil {
		return nil, err
	}

	localLuminances := g.source.Matrix()
	for y := 0; y < height; y++ {
		offset := y * width
		for x := 0; x < width; x++ {
			pixel := int(localLuminances[offset+x] & 0xff)
			if pixel < blackPoint {
				matrix.Set(x, y)
			}
		}
	}
	return matrix, nil
}

func (g *GlobalHistogram) initArrays(luminanceSize int) {
	if len(g.luminances) < luminanceSize {
		g.luminances = make([]byte, luminanceSize)
	}
	g.buckets = [luminanceBuckets]int{}
}

func estimateBlackPoint(buckets []int) (int, error) {
	numBuckets := len(buckets)
	maxBucketCount := 0
	firstPeak := 0
	firstPeakSize := 0
	for x := 0; x < numBuckets; x++ {
		if buckets[x] > firstPeakSize {
			firstPeak = x
			firstPeakSize = buckets[x]
		}
		if buckets[x] > maxBucketCount {
			maxBucketCount = buckets[x]
		}
	}

	secondPeak := 0
	secondPeakScore := 0
	for x := 0; x < numBuckets; x++ {
		dist := x - firstPeak
		score := buckets[x] * dist * dist
		if score > secondPeakScore {
			secondPeak = x
			secondPeakScore = score
		}
	}

	if firstPeak > secondPeak {
		firstPeak, secondPeak = secondPeak, firstPeak
	}

	if secondPeak-firstPeak <= numBuckets/16 {
		return 0, zxinggo.ErrNotFound
	}

	bestValley := secondPeak - 1
	bestValleyScore := -1
	for x := secondPeak - 1; x > firstPeak; x-- {
		fromFirst := x - firstPeak
		score := fromFirst * fromFirst * (secondPeak - x) * (maxBucketCount - buckets[x])
		if score > bestValleyScore {
			bestValley = x
			bestValleyScore = score
		}
	}

	return bestValley << luminanceShift, nil
}
