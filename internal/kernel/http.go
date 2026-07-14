package kernel

import (
	"context"
	"errors"
	"net/http"
	"skyrix/internal/engine"
	"skyrix/internal/logger"
	"time"
)

// HTTPApp is the final runnable HTTP application.
type HTTPApp struct {
	Server   *http.Server
	Kernel   *Kernel
	Platform engine.Platform
}

// NewHTTPApp is now a very simple constructor.
// It takes the assembled Core and the fully configured http.Server.
func NewHTTPApp(
	server *http.Server,
	kernel *Kernel,
	platform engine.Platform,
) (*HTTPApp, error) {
	return &HTTPApp{
		Server:   server,
		Kernel:   kernel,
		Platform: platform,
	}, nil
}

func (a *HTTPApp) Run(ctx context.Context, log logger.Interface) error {
	log.Info("Entering HTTPApp.Run method", "address", a.Server.Addr) // Added log

	if a.Platform != nil {
		log.Info("platform runtime starting")
		if err := a.Platform.Start(ctx); err != nil {
			log.Error("platform runtime start failed", "error", err)
			return err
		}
		log.Info("platform runtime started")
		defer func() {
			stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			log.Info("platform runtime stopping")
			if err := a.Platform.Stop(stopCtx); err != nil {
				log.Error("platform stop failed", "error", err)
				return
			}
			log.Info("platform runtime stopped")
		}()
	}

	errCh := make(chan error, 1)

	go func() {
		log.Info("HTTP server starting", "addr", a.Server.Addr)
		if err := a.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) { // Check for error
			log.Error("HTTP server ListenAndServe failed", "error", err, "address", a.Server.Addr) // Log the error
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Info("HTTP server shutting down")
		if err := a.Server.Shutdown(context.Background()); err != nil {
			log.Error("HTTP server shutdown failed", "error", err)
			return err
		}
		log.Info("HTTP server stopped")
		return nil
	case err := <-errCh:
		return err
	}
}
