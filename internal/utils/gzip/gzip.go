package gzip

import (
	"bytes"
	"compress/gzip"
	"io"
	"time"
)

const OptimalCompressionLevel = 5
const FastCompressionLevel = 5

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
		return nil, io.ErrUnexpectedEOF
	}
	return out.Bytes(), nil
}
