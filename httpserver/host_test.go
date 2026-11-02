package httpserver

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestHostServesUntilStopped(t *testing.T) {
	host := NewHost(&http.Server{Addr: "127.0.0.1:0", Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	})}, nil)
	if err := host.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	response, err := http.Get("http://" + host.Addr())
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if string(body) != "ok" {
		t.Fatalf("body = %q", body)
	}

	if err := host.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := http.Get("http://" + host.Addr()); err == nil {
		t.Fatal("server still accepts requests after Stop")
	}
}

func TestHostStartFailsWhenAddressIsTaken(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	host := NewHost(&http.Server{Addr: listener.Addr().String()}, nil)
	if err := host.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "listen on") {
		t.Fatalf("Start() error = %v, want listen error", err)
	}
}
