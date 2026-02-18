package zxinggo

import (
	"image"
	"image/color"
)

// ImageLuminanceSource is a LuminanceSource implementation that wraps a Go
// image.Image, converting each pixel to greyscale luminance on the fly.
type ImageLuminanceSource struct {
	luminances []byte
	width      int
	height     int
}

// NewImageLuminanceSource creates a LuminanceSource from a Go image.Image.
// The image is converted to greyscale luminance values upon construction.
// Uses the same luminance formula as Java ZXing's BufferedImageLuminanceSource:
// (306*R + 601*G + 117*B + 0x200) >> 10, operating on 8-bit color components.
func NewImageLuminanceSource(img image.Image) *ImageLuminanceSource {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	luminances := make([]byte, w*h)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := img.At(bounds.Min.X+x, bounds.Min.Y+y)
			_, _, _, a := c.RGBA()
			if a == 0 {
				// Fully-transparent pixels are forced to white, matching Java behavior.
				luminances[y*w+x] = 0xFF
			} else {
				r, g, b, _ := c.RGBA()
				// Convert 16-bit premultiplied RGBA to 8-bit per component.
				// For opaque pixels (a=0xFFFF), r>>8 gives the 8-bit value.
				// Use Java's exact formula: (306*R + 601*G + 117*B + 0x200) >> 10
				r8 := r >> 8
				g8 := g >> 8
				b8 := b >> 8
				luminances[y*w+x] = byte((306*r8 + 601*g8 + 117*b8 + 0x200) >> 10)
			}
		}
	}

	return &ImageLuminanceSource{
		luminances: luminances,
		width:      w,
		height:     h,
	}
}

// NewGrayImageLuminanceSource creates a LuminanceSource from a *image.Gray,
// using the pixel data directly without conversion.
func NewGrayImageLuminanceSource(img *image.Gray) *ImageLuminanceSource {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// If the image stride matches the width, we can use the pixel data directly
	if img.Stride == w && bounds.Min.X == 0 && bounds.Min.Y == 0 {
		lum := make([]byte, w*h)
		copy(lum, img.Pix[:w*h])
		return &ImageLuminanceSource{
			luminances: lum,
			width:      w,
			height:     h,
		}
	}

	// Otherwise copy row by row
	luminances := make([]byte, w*h)
	for y := 0; y < h; y++ {
		srcOff := (bounds.Min.Y+y)*img.Stride + bounds.Min.X
		copy(luminances[y*w:], img.Pix[srcOff:srcOff+w])
	}
	return &ImageLuminanceSource{
		luminances: luminances,
		width:      w,
		height:     h,
	}
}

// Row returns a row of luminance data.
func (s *ImageLuminanceSource) Row(y int, row []byte) []byte {
	if y < 0 || y >= s.height {
		return nil
	}
	if row == nil || len(row) < s.width {
		row = make([]byte, s.width)
	}
	offset := y * s.width
	copy(row, s.luminances[offset:offset+s.width])
	return row
}

// Matrix returns the entire luminance matrix.
func (s *ImageLuminanceSource) Matrix() []byte {
	result := make([]byte, len(s.luminances))
	copy(result, s.luminances)
	return result
}

// Width returns the width of the image.
func (s *ImageLuminanceSource) Width() int {
	return s.width
}

// Height returns the height of the image.
func (s *ImageLuminanceSource) Height() int {
	return s.height
}

// RotateCounterClockwise returns a new ImageLuminanceSource rotated 90 degrees
// counterclockwise. This is used by 1D readers to try reading barcodes that
// may be oriented vertically.
func (s *ImageLuminanceSource) RotateCounterClockwise() *ImageLuminanceSource {
	newWidth := s.height
	newHeight := s.width
	newLum := make([]byte, newWidth*newHeight)
	for y := 0; y < s.height; y++ {
		for x := 0; x < s.width; x++ {
			// (x, y) in old image -> (y, width - 1 - x) in new image
			newLum[(s.width-1-x)*newWidth+y] = s.luminances[y*s.width+x]
		}
	}
	return &ImageLuminanceSource{
		luminances: newLum,
		width:      newWidth,
		height:     newHeight,
	}
}

// BitMatrixToImage converts a BitMatrix to a grayscale image where black
// modules are black (0) and white modules are white (255).
func BitMatrixToImage(matrix interface{ Width() int; Height() int; Get(x, y int) bool }) *image.Gray {
	w := matrix.Width()
	h := matrix.Height()
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if matrix.Get(x, y) {
				img.SetGray(x, y, color.Gray{Y: 0})
			} else {
				img.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
	return img
}
