// Command smsd is the SMS platform HTTP server and the composition root: it
// wires dependencies, opens the listener and shuts down gracefully. It holds
// no business logic.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/sms-platform/internal/httpapi"
	"example.com/sms-platform/internal/provider/mock"
	"example.com/sms-platform/internal/queue"
	"example.com/sms-platform/internal/store"
	"example.com/sms-platform/internal/tenant"
)

const (
	defaultListenAddr = ":8080"
	purgeInterval     = time.Minute
	shutdownTimeout   = 10 * time.Second
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && a.Value.Kind() == slog.KindTime {
				a.Value = slog.TimeValue(a.Value.Time().UTC())
			}
			return a
		},
	})))

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = defaultListenAddr
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("listen failed", "error_class", "listen")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st := store.New(time.Now)
	q := queue.NewInProcess(st)
	prov := mock.New() // consumed by the queue workers from Story 1.6
	// TEMPORARY: the registry is empty, so every /v1/* request gets 401 until
	// config-file loading of tenants and API keys lands (a later story).
	tenants := tenant.NewRegistry()
	h := httpapi.NewHandler(httpapi.Deps{Now: time.Now, Queue: q, Store: st, Tenants: tenants})

	go func() { _ = q.Run(ctx) }()
	go purgeLoop(ctx, st)

	slog.Info("listening", "provider", prov.Name())
	if err := run(ctx, ln, h, q); err != nil {
		slog.Error("server stopped", "error_class", "run")
		os.Exit(1)
	}
}

// purgeLoop periodically asks the store to drop expired terminal messages.
func purgeLoop(ctx context.Context, st store.System) {
	t := time.NewTicker(purgeInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			st.PurgeExpired(time.Now())
		}
	}
}

// run serves h on ln until ctx is cancelled, then stops the listener first and
// closes the queue second, bounded by shutdownTimeout. The listener is
// injected so tests can share this exact serving path.
func run(ctx context.Context, ln net.Listener, h http.Handler, q queue.Queue) error {
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
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		shutErr := srv.Shutdown(shutdownCtx)
		closeErr := q.Close(shutdownCtx)
		return errors.Join(shutErr, closeErr)
	}
}
