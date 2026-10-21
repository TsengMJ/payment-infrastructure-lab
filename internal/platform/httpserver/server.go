// Package httpserver runs an HTTP server until the context is cancelled,
// then waits for in-flight requests to finish.
package httpserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

// Run listens on addr and serves handler until ctx is cancelled.
func Run(ctx context.Context, addr string, handler http.Handler, shutdownTimeout time.Duration) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return Serve(ctx, ln, handler, shutdownTimeout)
}

// Serve uses an existing listener. Shutdown waits up to shutdownTimeout for
// requests already in progress. It does not accept new work after ctx is done.
func Serve(ctx context.Context, ln net.Listener, handler http.Handler, shutdownTimeout time.Duration) error {
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		err := srv.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}

	shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutCtx)
}
