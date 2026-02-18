package oned

import (
	"testing"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/bitutil"
)

// roundTrip1D encodes a barcode, then decodes the resulting BitMatrix row by row.
func roundTrip1D(t *testing.T, contents string, format zxinggo.Format, encoder func(string) ([]bool, error), decoder RowDecoder) {
	t.Helper()

	code, err := encoder(contents)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	// Add quiet zones (10 modules each side)
	quiet := 10
	padded := make([]bool, len(code)+2*quiet)
	copy(padded[quiet:], code)

	// Build a BitArray from the boolean pattern
	row := bitutil.NewBitArray(len(padded))
	for i, b := range padded {
		if b {
			row.Set(i)
		}
	}

	result, err := decoder.DecodeRow(0, row, nil)
	if err != nil {
		t.Fatalf("decode error for %q: %v", contents, err)
	}

	if result.Text != contents {
		t.Errorf("round-trip mismatch: got %q, want %q", result.Text, contents)
	}
	if result.Format != format {
		t.Errorf("format mismatch: got %v, want %v", result.Format, format)
	}
}

// --- Code 39 ---

func TestCode39RoundTrip(t *testing.T) {
	tests := []string{
		"HELLO",
		"WORLD",
		"12345",
		"TEST-123",
		"A B.C",
	}
	writer := NewCode39Writer()
	reader := NewCode39Reader()
	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			roundTrip1D(t, tc, zxinggo.FormatCode39, writer.encode, reader)
		})
	}
}

// --- Code 128 ---

func TestCode128RoundTrip(t *testing.T) {
	tests := []string{
		"Hello",
		"12345678",
		"Test 123",
		"ABC-def",
		"1234567890",
	}
	reader := NewCode128Reader()
	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			code, err := encodeCode128Fast(tc, -1)
			if err != nil {
				t.Fatalf("encode error: %v", err)
			}

			quiet := 10
			padded := make([]bool, len(code)+2*quiet)
			copy(padded[quiet:], code)

			row := bitutil.NewBitArray(len(padded))
			for i, b := range padded {
				if b {
					row.Set(i)
				}
			}

			result, err := reader.DecodeRow(0, row, nil)
			if err != nil {
				t.Fatalf("decode error for %q: %v", tc, err)
			}
			if result.Text != tc {
				t.Errorf("round-trip mismatch: got %q, want %q", result.Text, tc)
			}
			if result.Format != zxinggo.FormatCode128 {
				t.Errorf("format mismatch: got %v, want %v", result.Format, zxinggo.FormatCode128)
			}
		})
	}
}

// --- EAN-13 ---

func TestEAN13RoundTrip(t *testing.T) {
	tests := []string{
		"5901234123457",
		"4006381333931",
		"0012345678905",
	}
	writer := NewEAN13Writer()
	reader := NewEAN13Reader()
	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			roundTrip1D(t, tc, zxinggo.FormatEAN13, writer.EncodeContents, reader)
		})
	}
}

func TestEAN13RoundTripWithoutCheckDigit(t *testing.T) {
	// Input 12 digits, writer computes check digit
	writer := NewEAN13Writer()
	reader := NewEAN13Reader()

	code, err := writer.EncodeContents("590123412345")
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	quiet := 10
	padded := make([]bool, len(code)+2*quiet)
	copy(padded[quiet:], code)

	row := bitutil.NewBitArray(len(padded))
	for i, b := range padded {
		if b {
			row.Set(i)
		}
	}

	result, err := reader.DecodeRow(0, row, nil)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if result.Text != "5901234123457" {
		t.Errorf("got %q, want %q", result.Text, "5901234123457")
	}
}

// --- EAN-8 ---

func TestEAN8RoundTrip(t *testing.T) {
	tests := []string{
		"96385074",
		"12345670",
	}
	writer := NewEAN8Writer()
	reader := NewEAN8Reader()
	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			roundTrip1D(t, tc, zxinggo.FormatEAN8, writer.EncodeContents, reader)
		})
	}
}

// --- UPC-A ---

func TestUPCARoundTrip(t *testing.T) {
	tests := []string{
		"012345678905",
	}
	reader := NewUPCAReader()
	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			// UPC-A writer prepends 0 and uses EAN-13, so we encode via the writer
			// and decode the raw row
			ean13Writer := NewEAN13Writer()
			code, err := ean13Writer.EncodeContents("0" + tc)
			if err != nil {
				t.Fatalf("encode error: %v", err)
			}

			quiet := 10
			padded := make([]bool, len(code)+2*quiet)
			copy(padded[quiet:], code)

			row := bitutil.NewBitArray(len(padded))
			for i, b := range padded {
				if b {
					row.Set(i)
				}
			}

			result, err := reader.DecodeRow(0, row, nil)
			if err != nil {
				t.Fatalf("decode error for %q: %v", tc, err)
			}
			if result.Text != tc {
				t.Errorf("round-trip mismatch: got %q, want %q", result.Text, tc)
			}
			if result.Format != zxinggo.FormatUPCA {
				t.Errorf("format mismatch: got %v, want %v", result.Format, zxinggo.FormatUPCA)
			}
		})
	}
}

// --- UPC-E ---

func TestUPCERoundTrip(t *testing.T) {
	tests := []string{
		"01234565",
	}
	writer := NewUPCEWriter()
	reader := NewUPCEReader()
	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			roundTrip1D(t, tc, zxinggo.FormatUPCE, writer.EncodeContents, reader)
		})
	}
}

// --- Checksum ---

func TestUPCEANChecksum(t *testing.T) {
	tests := []struct {
		input string
		check int
	}{
		{"590123412345", 7},
		{"1234567890", 5},
	}
	for _, tc := range tests {
		got := GetStandardUPCEANChecksum(tc.input)
		if got != tc.check {
			t.Errorf("GetStandardUPCEANChecksum(%q) = %d, want %d", tc.input, got, tc.check)
		}
	}
}

func TestCheckStandardUPCEANChecksum(t *testing.T) {
	if !CheckStandardUPCEANChecksum("5901234123457") {
		t.Error("expected checksum to pass for 5901234123457")
	}
	if CheckStandardUPCEANChecksum("5901234123456") {
		t.Error("expected checksum to fail for 5901234123456")
	}
}

// --- UPC-E conversion ---

func TestConvertUPCEtoUPCA(t *testing.T) {
	tests := []struct {
		upce string
		upca string
	}{
		{"01234565", "012345000065"},
		{"01200003", "012000000003"},
	}
	for _, tc := range tests {
		got := ConvertUPCEtoUPCA(tc.upce)
		if got != tc.upca {
			t.Errorf("ConvertUPCEtoUPCA(%q) = %q, want %q", tc.upce, got, tc.upca)
		}
	}
}

// --- Writer format validation ---

func TestWriterFormatValidation(t *testing.T) {
	_, err := NewCode39Writer().Encode("TEST", zxinggo.FormatCode128, 100, 50, nil)
	if err == nil {
		t.Error("expected error for wrong format on Code39Writer")
	}

	_, err = NewCode128Writer().Encode("TEST", zxinggo.FormatCode39, 100, 50, nil)
	if err == nil {
		t.Error("expected error for wrong format on Code128Writer")
	}

	_, err = NewEAN13Writer().Encode("5901234123457", zxinggo.FormatCode39, 100, 50, nil)
	if err == nil {
		t.Error("expected error for wrong format on EAN13Writer")
	}

	_, err = NewEAN8Writer().Encode("96385074", zxinggo.FormatCode39, 100, 50, nil)
	if err == nil {
		t.Error("expected error for wrong format on EAN8Writer")
	}
}

// --- ITF ---

func TestITFRoundTrip(t *testing.T) {
	tests := []string{
		"123456",
		"00123456789012",
		"1234567890",
		"30712345000010",
	}
	writer := NewITFWriter()
	reader := NewITFReader()
	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			roundTrip1D(t, tc, zxinggo.FormatITF, writer.encode, reader)
		})
	}
}

func TestITFOddLengthRejected(t *testing.T) {
	_, err := NewITFWriter().Encode("12345", zxinggo.FormatITF, 200, 50, nil)
	if err == nil {
		t.Error("expected error for odd-length ITF input")
	}
}

// --- Codabar ---

func TestCodabarRoundTrip(t *testing.T) {
	tests := []string{
		"123456",
		"1234-5678",
		"29.95",
		"100.00",
	}
	writer := NewCodabarWriter()
	reader := NewCodabarReader()
	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			code, err := writer.encode(tc)
			if err != nil {
				t.Fatalf("encode error: %v", err)
			}

			quiet := 10
			padded := make([]bool, len(code)+2*quiet)
			copy(padded[quiet:], code)

			row := bitutil.NewBitArray(len(padded))
			for i, b := range padded {
				if b {
					row.Set(i)
				}
			}

			result, err := reader.DecodeRow(0, row, nil)
			if err != nil {
				t.Fatalf("decode error for %q: %v", tc, err)
			}
			if result.Text != tc {
				t.Errorf("round-trip mismatch: got %q, want %q", result.Text, tc)
			}
			if result.Format != zxinggo.FormatCodabar {
				t.Errorf("format mismatch: got %v, want %v", result.Format, zxinggo.FormatCodabar)
			}
		})
	}
}

// --- MultiFormatOneDReader ---

func TestMultiFormatOneDReaderCode39(t *testing.T) {
	writer := NewCode39Writer()
	code, err := writer.encode("HELLO")
	if err != nil {
		t.Fatal(err)
	}

	quiet := 10
	padded := make([]bool, len(code)+2*quiet)
	copy(padded[quiet:], code)

	row := bitutil.NewBitArray(len(padded))
	for i, b := range padded {
		if b {
			row.Set(i)
		}
	}

	reader := NewMultiFormatOneDReader(nil)
	result, err := reader.DecodeRow(0, row, nil)
	if err != nil {
		t.Fatalf("multi-format decode error: %v", err)
	}
	if result.Text != "HELLO" {
		t.Errorf("got %q, want %q", result.Text, "HELLO")
	}
}
