package gzip_test

import (
	"bytes"
	"errors"
	"testing"

	"gitlab.com/skyrix-lib/framework/httpserver/gzip"
)

func TestRoundTrip(t *testing.T) {
	in := bytes.Repeat([]byte("skyrix "), 1000)
	compressed, err := gzip.GzipBytes(in, 0)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := gzip.GzipBytes(in, 0)
	if !bytes.Equal(compressed, again) {
		t.Fatal("GzipBytes output is not deterministic")
	}
	out, err := gzip.GunzipBytesWithLimit(compressed, int64(len(in)))
	if err != nil || !bytes.Equal(out, in) {
		t.Fatalf("GunzipBytesWithLimit() = %d bytes, %v", len(out), err)
	}
}

func TestGunzipLimit(t *testing.T) {
	compressed, _ := gzip.GzipBytes(make([]byte, 100), gzip.FastCompressionLevel)
	if _, err := gzip.GunzipBytesWithLimit(compressed, 99); !errors.Is(err, gzip.ErrTooLarge) {
		t.Fatalf("error = %v, want ErrTooLarge", err)
	}
	if out, err := gzip.GunzipBytesWithLimit(compressed, 100); err != nil || len(out) != 100 {
		t.Fatalf("at the limit = %d bytes, %v", len(out), err)
	}
}

func TestFastLevelDiffersFromOptimal(t *testing.T) {
	if gzip.FastCompressionLevel == gzip.OptimalCompressionLevel {
		t.Fatal("FastCompressionLevel equals OptimalCompressionLevel")
	}
	in := bytes.Repeat([]byte("skyrix fast "), 1000)
	if _, err := gzip.GzipBytes(in, gzip.FastCompressionLevel); err != nil {
		t.Fatal(err)
	}
	if _, err := gzip.GunzipBytesWithLimit([]byte("not gzip"), 10); err == nil {
		t.Fatal("invalid input error = nil")
	}
}
