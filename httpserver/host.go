package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"gitlab.com/skyrix-lib/framework/kernel"
	"gitlab.com/skyrix-lib/framework/logging"
)

// Host runs an http.Server as a kernel.Component. Start binds the listener
// synchronously, so address errors fail the start; serving errors after that
// are reported through Failed.
type Host struct {
	server   *http.Server
	log      logger.Interface
	failures chan error
}

// NewHost returns a Host for the server.
func NewHost(server *http.Server, log logger.Interface) *Host {
	return &Host{server: server, log: log, failures: make(chan error, 1)}
}

// Start listens on the server address and serves in the background.
func (h *Host) Start(context.Context) error {
	listener, err := net.Listen("tcp", h.server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", h.server.Addr, err)
	}
	h.server.Addr = listener.Addr().String()
	if h.log != nil {
		h.log.Info("HTTP server started", "address", h.server.Addr)
	}
	go func() {
		if err := h.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			h.failures <- fmt.Errorf("serve HTTP on %s: %w", h.server.Addr, err)
		}
	}()
	return nil
}

// Stop gracefully shuts the server down within ctx.
func (h *Host) Stop(ctx context.Context) error {
	if err := h.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shut down HTTP server: %w", err)
	}
	if h.log != nil {
		h.log.Info("HTTP server stopped", "address", h.server.Addr)
	}
	return nil
}

// Failed reports a serving error that happened after Start.
func (h *Host) Failed() <-chan error { return h.failures }

// Addr returns the server address; after Start it is the bound address.
func (h *Host) Addr() string { return h.server.Addr }

var (
	_ kernel.Component       = (*Host)(nil)
	_ kernel.FailureNotifier = (*Host)(nil)
)
