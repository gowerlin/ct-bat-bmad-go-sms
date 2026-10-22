// Command smsd is the SMS platform HTTP server.
//
// This skeleton serves only GET /healthz. Every other route in
// docs/openapi.yaml arrives with its own work order (T0001 onward).
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/sms-platform/internal/httpapi"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "TCP address to listen on")
	flag.Parse()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("smsd: listen %s: %v", *addr, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("smsd: listening on http://%s", ln.Addr())
	if err := run(ctx, ln, httpapi.NewHandler(httpapi.Deps{})); err != nil {
		log.Fatalf("smsd: %v", err)
	}
}

// run serves h on ln until ctx is cancelled, then shuts the server down
// gracefully. The listener is injected so that tests can pass their own
// (e.g. a loopback listener on port 0) and share this exact serving path.
func run(ctx context.Context, ln net.Listener, h http.Handler) error {
	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
