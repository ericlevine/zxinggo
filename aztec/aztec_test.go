package aztec

import (
	"testing"

	zxinggo "github.com/ericlevine/zxinggo"
	"github.com/ericlevine/zxinggo/aztec/decoder"
	"github.com/ericlevine/zxinggo/aztec/encoder"
)

func TestAztecEncoderDecoder(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"Hello", "Hello"},
		{"Digits", "1234567890"},
		{"Upper", "ABCDEF"},
		{"Mixed", "Hello, World!"},
		{"Lower", "abcdef"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, err := encoder.Encode([]byte(tc.data), 25, 0)
			if err != nil {
				t.Fatalf("encode error: %v", err)
			}

			// Feed the encoder's output directly to the decoder, bypassing
			// the detector (which requires image-level bullseye finding).
			ddata := &decoder.AztecDetectorResult{
				Bits:         code.Matrix,
				Compact:      code.Compact,
				NbDataBlocks: code.CodeWords,
				NbLayers:     code.Layers,
			}

			dr, err := decoder.Decode(ddata)
			if err != nil {
				t.Fatalf("decode error for %q: %v", tc.data, err)
			}
			if dr.Text != tc.data {
				t.Errorf("round-trip mismatch: got %q, want %q", dr.Text, tc.data)
			}
		})
	}
}

func TestAztecWriterFormatValidation(t *testing.T) {
	_, err := NewWriter().Encode("TEST", zxinggo.FormatQRCode, 200, 200, nil)
	if err == nil {
		t.Error("expected error for wrong format on AztecWriter")
	}
}
