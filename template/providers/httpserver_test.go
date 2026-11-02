package providers

import (
	"testing"
	"time"

	"gitlab.com/skyrix-lib/template/internal/config"
)

func TestProvideHTTPServerOptions(t *testing.T) {
	options := ProvideHTTPServerOptions(&config.HttpServer{Address: "127.0.0.1", Port: 9000, ReadHeaderTimeout: 7 * time.Second})
	if options.Address != "127.0.0.1" || options.Port != 9000 || options.ReadHeaderTimeout != 7*time.Second {
		t.Fatalf("options = %+v", options)
	}
}
