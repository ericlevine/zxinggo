package oned

import (
	"fmt"
	"strconv"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

var extensionStartPattern = []int{1, 1, 2}

var checkDigitEncodings = [10]int{
	0x18, 0x14, 0x12, 0x11, 0x0C, 0x06, 0x03, 0x0A, 0x09, 0x05,
}

// decodeUPCEANExtension attempts to decode a 2-digit or 5-digit supplemental
// barcode after the main UPC/EAN barcode.
func decodeUPCEANExtension(rowNumber int, row *bitutil.BitArray, rowOffset int) (*zxinggo.Result, error) {
	extStartRange, err := findUPCEANGuardPattern(row, rowOffset, false, extensionStartPattern, make([]int, len(extensionStartPattern)))
	if err != nil {
		return nil, err
	}

	// Try 5-digit first, then fall back to 2-digit
	result, err := decodeExtension5(rowNumber, row, extStartRange)
	if err == nil {
		return result, nil
	}
	return decodeExtension2(rowNumber, row, extStartRange)
}

func decodeExtension2(rowNumber int, row *bitutil.BitArray, extensionStartRange [2]int) (*zxinggo.Result, error) {
	counters := make([]int, 4)
	end := row.Size()
	rowOffset := extensionStartRange[1]

	checkParity := 0

	var resultStr [2]byte
	for x := 0; x < 2 && rowOffset < end; x++ {
		bestMatch, err := DecodeUPCEANDigit(row, counters, rowOffset, LAndGPatterns[:])
		if err != nil {
			return nil, err
		}
		resultStr[x] = '0' + byte(bestMatch%10)
		for _, c := range counters {
			rowOffset += c
		}
		if bestMatch >= 10 {
			checkParity |= 1 << uint(1-x)
		}
		if x != 1 {
			rowOffset = row.GetNextSet(rowOffset)
			rowOffset = row.GetNextUnset(rowOffset)
		}
	}

	s := string(resultStr[:])
	val, err := strconv.Atoi(s)
	if err != nil {
		return nil, zxinggo.ErrNotFound
	}
	if val%4 != checkParity {
		return nil, zxinggo.ErrNotFound
	}

	result := zxinggo.NewResult(
		s, nil,
		[]zxinggo.ResultPoint{
			{X: float64(extensionStartRange[0]+extensionStartRange[1]) / 2.0, Y: float64(rowNumber)},
			{X: float64(rowOffset), Y: float64(rowNumber)},
		},
		zxinggo.FormatEAN13, // extension uses parent format
	)
	result.PutMetadata(zxinggo.MetadataIssueNumber, val)
	return result, nil
}

func decodeExtension5(rowNumber int, row *bitutil.BitArray, extensionStartRange [2]int) (*zxinggo.Result, error) {
	counters := make([]int, 4)
	end := row.Size()
	rowOffset := extensionStartRange[1]

	lgPatternFound := 0

	var resultStr [5]byte
	for x := 0; x < 5 && rowOffset < end; x++ {
		bestMatch, err := DecodeUPCEANDigit(row, counters, rowOffset, LAndGPatterns[:])
		if err != nil {
			return nil, err
		}
		resultStr[x] = '0' + byte(bestMatch%10)
		for _, c := range counters {
			rowOffset += c
		}
		if bestMatch >= 10 {
			lgPatternFound |= 1 << uint(4-x)
		}
		if x != 4 {
			rowOffset = row.GetNextSet(rowOffset)
			rowOffset = row.GetNextUnset(rowOffset)
		}
	}

	s := string(resultStr[:])
	if len(s) != 5 {
		return nil, zxinggo.ErrNotFound
	}

	checkDigit, err := ext5DetermineCheckDigit(lgPatternFound)
	if err != nil {
		return nil, err
	}
	if ext5Checksum(s) != checkDigit {
		return nil, zxinggo.ErrNotFound
	}

	result := zxinggo.NewResult(
		s, nil,
		[]zxinggo.ResultPoint{
			{X: float64(extensionStartRange[0]+extensionStartRange[1]) / 2.0, Y: float64(rowNumber)},
			{X: float64(rowOffset), Y: float64(rowNumber)},
		},
		zxinggo.FormatEAN13, // extension uses parent format
	)
	price := parseExtension5String(s)
	if price != "" {
		result.PutMetadata(zxinggo.MetadataSuggestedPrice, price)
	}
	return result, nil
}

func ext5Checksum(s string) int {
	length := len(s)
	sum := 0
	for i := length - 2; i >= 0; i -= 2 {
		sum += int(s[i] - '0')
	}
	sum *= 3
	for i := length - 1; i >= 0; i -= 2 {
		sum += int(s[i] - '0')
	}
	sum *= 3
	return sum % 10
}

func ext5DetermineCheckDigit(lgPatternFound int) (int, error) {
	for d := 0; d < 10; d++ {
		if lgPatternFound == checkDigitEncodings[d] {
			return d, nil
		}
	}
	return 0, zxinggo.ErrNotFound
}

func parseExtension5String(raw string) string {
	if len(raw) != 5 {
		return ""
	}
	var currency string
	switch raw[0] {
	case '0':
		currency = "\u00a3" // £
	case '5':
		currency = "$"
	case '9':
		switch raw {
		case "90000":
			return ""
		case "99991":
			return "0.00"
		case "99990":
			return "Used"
		}
		currency = ""
	default:
		currency = ""
	}
	rawAmount, err := strconv.Atoi(raw[1:])
	if err != nil {
		return ""
	}
	units := rawAmount / 100
	hundredths := rawAmount % 100
	return fmt.Sprintf("%s%d.%02d", currency, units, hundredths)
}
