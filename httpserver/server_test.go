package httpserver

import (
	"net/http"
	"testing"
	"time"
)

func TestNewServerReadHeaderTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		want    time.Duration
	}{
		{name: "zero uses default", timeout: 0, want: DefaultReadHeaderTimeout},
		{name: "negative uses default", timeout: -time.Second, want: DefaultReadHeaderTimeout},
		{name: "positive is kept", timeout: 3 * time.Second, want: 3 * time.Second},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := NewServer(http.NotFoundHandler(), Options{Port: 8080, ReadHeaderTimeout: test.timeout})
			if server.ReadHeaderTimeout != test.want {
				t.Fatalf("ReadHeaderTimeout = %v, want %v", server.ReadHeaderTimeout, test.want)
			}
		})
	}
}
