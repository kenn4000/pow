package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// shutdownGrace bounds how long in-flight requests may finish after a stop
// signal. It stays below Kubernetes' default 30-second termination grace period.
const shutdownGrace = 20 * time.Second

// serveUntilSignal serves until SIGTERM or SIGINT, then stops accepting new
// connections and lets in-flight requests finish, so rolling updates and local
// Ctrl-C do not cut requests off mid-response.
func serveUntilSignal(listener net.Listener, handler http.Handler) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	return serveUntilDone(ctx, listener, handler, shutdownGrace)
}

func serveUntilDone(ctx context.Context, listener net.Listener, handler http.Handler, grace time.Duration) error {
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	select {
	case err := <-served:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), grace)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	if err := <-served; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
