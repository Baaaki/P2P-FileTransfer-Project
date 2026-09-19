package transfer

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestCompressChunkRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"short text", []byte("merhaba dunya, p2p transferilla!")},
		{"repeated text", bytes.Repeat([]byte("filetransferilla on-the-fly huffman compression testing "), 100)},
		{"binary zeroes", make([]byte, 4096)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			compressed := CompressChunk(nil, tc.data)
			decompressed, err := DecompressChunk(nil, compressed, len(tc.data)+100)
			if err != nil {
				t.Fatalf("DecompressChunk failed: %v", err)
			}
			if !bytes.Equal(decompressed, tc.data) {
				t.Fatalf("Decompressed data does not match original (got %d bytes, want %d)", len(decompressed), len(tc.data))
			}
		})
	}
}

func TestCompressChunkEfficiency(t *testing.T) {
	// A 32KB block of repetitive text (like logs or source code)
	source := bytes.Repeat([]byte("2026-09-19 INFO [transfer] chunk completed successfully in 12ms\n"), 500)
	compressed := CompressChunk(nil, source)

	// Huffman compression on structured log text should yield at least 40% reduction
	if len(compressed) >= len(source)*6/10 {
		t.Errorf("Expected substantial compression, got %d from %d", len(compressed), len(source))
	}

	// Should be identified as compressible
	if !IsProbablyCompressible(source[:512]) {
		t.Errorf("IsProbablyCompressible returned false for log data")
	}
}

func TestIsProbablyCompressibleRandom(t *testing.T) {
	randomData := make([]byte, 1024)
	_, _ = rand.Read(randomData)

	if IsProbablyCompressible(randomData) {
		t.Errorf("Random uncompressible data was marked as compressible")
	}
}

func TestDecompressChunkOversized(t *testing.T) {
	data := bytes.Repeat([]byte("A"), 1000)
	compressed := CompressChunk(nil, data)

	_, err := DecompressChunk(nil, compressed, 500) // limit to 500 bytes
	if err != ErrChunkTooLarge {
		t.Fatalf("Expected ErrChunkTooLarge, got: %v", err)
	}
}
