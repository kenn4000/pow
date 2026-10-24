package app

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// A request already in progress when shutdown begins must still complete.
func TestServeUntilDoneDrainsInFlightRequests(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(300 * time.Millisecond)
		io.WriteString(w, "finished")
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serveUntilDone(ctx, listener, handler, 5*time.Second) }()

	response := make(chan string, 1)
	go func() {
		resp, err := http.Get("http://" + listener.Addr().String())
		if err != nil {
			response <- "error: " + err.Error()
			return
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		response <- string(body)
	}()
	<-started
	cancel()
	if got := <-response; got != "finished" {
		t.Fatalf("in-flight request got %q, want it to finish during shutdown", got)
	}
	if err := <-done; err != nil {
		t.Fatalf("serveUntilDone returned %v, want nil after graceful shutdown", err)
	}
	if _, err := http.Get("http://" + listener.Addr().String()); err == nil {
		t.Fatal("server still accepted connections after shutdown")
	}
}
