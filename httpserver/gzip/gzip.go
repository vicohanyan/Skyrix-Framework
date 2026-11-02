// Package gzip contains helpers for gzip-encoded HTTP payloads: compressing
// a response or message body and decompressing one with a size limit. Request
// bodies are decompressed by middleware.GzipDecompressMiddleware.
package gzip

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"time"
)

// OptimalCompressionLevel is the default level of GzipBytes.
const OptimalCompressionLevel = 5

// FastCompressionLevel trades ratio for speed (gzip.BestSpeed).
const FastCompressionLevel = gzip.BestSpeed

// ErrTooLarge is returned by GunzipBytesWithLimit when the decompressed data
// exceeds the limit.
var ErrTooLarge = errors.New("gzip: decompressed data exceeds limit")

// GzipBytes compresses in at level (OptimalCompressionLevel when 0). The
// output is deterministic: no name, comment or modification time.
func GzipBytes(in []byte, level int) ([]byte, error) {
	if level == 0 {
		level = OptimalCompressionLevel
	}
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, level)
	if err != nil {
		return nil, err
	}
	zw.Name = ""
	zw.Comment = ""
	zw.ModTime = time.Unix(0, 0)

	if _, err = zw.Write(in); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err = zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GunzipBytesWithLimit decompresses gz and fails with ErrTooLarge when the
// result exceeds maxUnzipped bytes.
func GunzipBytesWithLimit(gz []byte, maxUnzipped int64) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	var out bytes.Buffer
	n, err := io.CopyN(&out, gr, maxUnzipped+1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if n > maxUnzipped {
		return nil, ErrTooLarge
	}
	return out.Bytes(), nil
}
