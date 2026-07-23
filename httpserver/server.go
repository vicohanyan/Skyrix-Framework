package httpserver

import (
	"fmt"
	"net/http"
)

type Options struct {
	Address string
	Port    int
}

// NewServer creates the net/http server without application-specific config.
func NewServer(handler http.Handler, options Options) *http.Server {
	address := options.Address
	if address == "localhost" || address == "" {
		address = "0.0.0.0"
	}

	return &http.Server{
		Addr:    fmt.Sprintf("%s:%d", address, options.Port),
		Handler: handler,
	}
}
