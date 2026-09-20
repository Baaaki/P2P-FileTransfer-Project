package transfer

import (
	"bytes"
	"compress/flate"
	"errors"
	"fmt"
	"io"
	"sync"
)

var (
	// ErrDecompressFailed indicates flate data was corrupt or invalid.
	ErrDecompressFailed = errors.New("decompression failed")
	// ErrChunkTooLarge indicates decompressed data exceeded the expected size.
	ErrChunkTooLarge = errors.New("decompressed chunk exceeds limit")
)

// huffmanWriterPool avoids re-allocating flate.Writer on every chunk.
// flate.HuffmanOnly provides instant, zero-CPU overhead compression
// that accelerates text, logs, code, and uncompressed payloads without
// stalling fast connections or wasting CPU cycles on already-compressed media.
var huffmanWriterPool = sync.Pool{
	New: func() any {
		w, err := flate.NewWriter(io.Discard, flate.HuffmanOnly)
		if err != nil {
			panic(err)
		}
		return w
	},
}

type resettableReadCloser interface {
	io.ReadCloser
	flate.Resetter
}

var flateReaderPool = sync.Pool{
	New: func() any {
		return flate.NewReader(bytes.NewReader(nil)).(resettableReadCloser)
	},
}

// CompressChunk compresses src using flate.HuffmanOnly into dst (reusing capacity if possible).
func CompressChunk(dst, src []byte) []byte {
	buf := bytes.NewBuffer(dst[:0])
	if cap(dst) < len(src) {
		buf.Grow(len(src) - cap(dst))
	}
	w := huffmanWriterPool.Get().(*flate.Writer)
	w.Reset(buf)
	_, _ = w.Write(src)
	_ = w.Close()
	w.Reset(io.Discard)
	huffmanWriterPool.Put(w)
	return buf.Bytes()
}

// DecompressChunk decompresses src into dst (reusing capacity if possible) up to maxLen bytes.
func DecompressChunk(dst, src []byte, maxLen int) ([]byte, error) {
	buf := bytes.NewBuffer(dst[:0])
	r := flateReaderPool.Get().(resettableReadCloser)
	defer func() {
		_ = r.Close()
		_ = r.Reset(bytes.NewReader(nil), nil)
		flateReaderPool.Put(r)
	}()

	if err := r.Reset(bytes.NewReader(src), nil); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecompressFailed, err)
	}

	lr := io.LimitReader(r, int64(maxLen)+1)
	n, err := buf.ReadFrom(lr)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: %w", ErrDecompressFailed, err)
	}
	if n > int64(maxLen) {
		return nil, ErrChunkTooLarge
	}
	return buf.Bytes(), nil
}

// IsProbablyCompressible tests the first few kilobytes of data to check if compression
// is worthwhile (e.g. not random/already compressed).
func IsProbablyCompressible(sample []byte) bool {
	if len(sample) < 64 {
		return false
	}
	comp := CompressChunk(nil, sample)
	return float64(len(comp)) < float64(len(sample))*0.92
}
