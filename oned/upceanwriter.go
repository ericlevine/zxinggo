package oned

import (
	"fmt"
	"strings"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// UPCEANEncoder encodes the middle portion of a UPC/EAN barcode.
type UPCEANEncoder interface {
	// EncodeContents encodes the full barcode contents into a boolean array.
	EncodeContents(contents string) ([]bool, error)
}

// EncodeUPCEAN encodes a UPC/EAN barcode with validation.
func EncodeUPCEAN(contents string, format zxinggo.Format, width, height int, encoder UPCEANEncoder) (*bitutil.BitMatrix, error) {
	code, err := encoder.EncodeContents(contents)
	if err != nil {
		return nil, err
	}
	return RenderOneDCode(code, width, height), nil
}

// CheckUPCEANDigits validates that a string contains only digits.
func CheckUPCEANDigits(s string) error {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return fmt.Errorf("contents contain non-digit character: %c", s[i])
		}
	}
	return nil
}

// CheckUPCEANLength validates the length and optionally computes/validates the check digit.
// expectedWithout is the length without check digit, expectedWith is the length with check digit.
func CheckUPCEANLength(contents string, expectedWithout, expectedWith int) (string, error) {
	length := len(contents)
	switch length {
	case expectedWithout:
		check := GetStandardUPCEANChecksum(contents)
		if check < 0 {
			return "", zxinggo.ErrFormat
		}
		contents += string(rune('0' + check))
	case expectedWith:
		if !CheckStandardUPCEANChecksum(contents) {
			return "", fmt.Errorf("contents do not pass checksum")
		}
	default:
		return "", fmt.Errorf("requested contents should be %d or %d digits long, but got %d",
			expectedWithout, expectedWith, length)
	}
	if err := CheckUPCEANDigits(contents); err != nil {
		return "", err
	}
	return contents, nil
}

// FormatUPCEANContents handles UPC-A to EAN-13 conversion if needed.
func FormatUPCEANContents(contents string, format zxinggo.Format) string {
	if format == zxinggo.FormatUPCA {
		// Transform UPC-A to EAN-13 by prepending 0
		if !strings.HasPrefix(contents, "0") {
			contents = "0" + contents
		}
	}
	return contents
}
