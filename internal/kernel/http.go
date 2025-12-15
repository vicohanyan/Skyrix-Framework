package kernel

import (
	"context"
	"net/http"
	"skyrix/internal/logger"
)

// HTTPApp is the final runnable HTTP application.
type HTTPApp struct {
	Server *http.Server
	Kernel *Kernel
}

// NewHTTPApp is now a very simple constructor.
// It takes the assembled Core and the fully configured http.Server.
func NewHTTPApp(
	server *http.Server,
	kernel *Kernel,
) (*HTTPApp, error) {
	return &HTTPApp{
		Server: server,
		Kernel: kernel,
	}, nil
}

func (a *HTTPApp) Run(ctx context.Context, log logger.Interface) error {
	log.Info("Entering HTTPApp.Run method", "address", a.Server.Addr) // Added log

	errCh := make(chan error, 1)

	go func() {
		log.Info("HTTP server starting", "addr", a.Server.Addr)
		if err := a.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed { // Check for error
			log.Error("HTTP server ListenAndServe failed", "error", err, "address", a.Server.Addr) // Log the error
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Info("HTTP server shutting down")
		_ = a.Server.Shutdown(context.Background())
		return nil
	case err := <-errCh:
		return err
	}
}
