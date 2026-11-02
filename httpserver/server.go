package httpserver

import (
	"fmt"
	"net/http"
	"time"
)

// DefaultReadHeaderTimeout is the ReadHeaderTimeout of servers whose Options
// do not set a positive one. It bounds how long a client may take to send the
// request headers (protection against Slowloris).
const DefaultReadHeaderTimeout = 10 * time.Second

type Options struct {
	Address string
	Port    int
	// ReadHeaderTimeout is the time allowed to read the request headers.
	// Zero or negative uses DefaultReadHeaderTimeout; it is never disabled.
	ReadHeaderTimeout time.Duration
}

// NewServer creates the net/http server without application-specific config.
func NewServer(handler http.Handler, options Options) *http.Server {
	address := options.Address
	if address == "localhost" || address == "" {
		address = "0.0.0.0"
	}
	readHeaderTimeout := options.ReadHeaderTimeout
	if readHeaderTimeout <= 0 {
		readHeaderTimeout = DefaultReadHeaderTimeout
	}

	return &http.Server{
		Addr:              fmt.Sprintf("%s:%d", address, options.Port),
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}
}
