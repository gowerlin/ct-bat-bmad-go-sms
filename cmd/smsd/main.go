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
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/sms-platform/internal/httpapi"
	"example.com/sms-platform/internal/provider/mock"
	"example.com/sms-platform/internal/queue"
	"example.com/sms-platform/internal/ratelimit"
	"example.com/sms-platform/internal/store"
	"example.com/sms-platform/internal/template"
	"example.com/sms-platform/internal/tenant"
	"example.com/sms-platform/internal/webhook"
)

const (
	defaultListenAddr = ":8080"
	purgeInterval     = time.Minute
	shutdownTimeout   = 10 * time.Second
	// mockCallbackDelay is how long PROVIDER=mock waits after an accepted Send
	// before it reports delivered, so the queue has written sent first.
	mockCallbackDelay = time.Second
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
	prov, err := newProvider(os.Getenv, time.Now)
	if err != nil {
		logProviderError(slog.Default(), err)
		os.Exit(1)
	}
	q := queue.NewInProcess(st, queue.WithProvider(prov))
	// TEMPORARY: the registry is empty, so every /v1/* request gets 401 and no
	// tenant has a CallbackURL until config-file loading of tenants and API
	// keys lands (a later story).
	tenants := tenant.NewRegistry()
	h := httpapi.NewHandler(httpapi.Deps{
		Now: time.Now, Queue: q, Store: st, Tenants: tenants, Templates: template.NewService(st),
		Limiter:   ratelimit.New(time.Now, tenants.RateLimitPerMin),
		Callbacks: webhook.NewInbound(st, tenants, prov),
	})
	if m, ok := prov.(*mock.Provider); ok {
		// AD-6: mock callbacks go through the real route and ParseCallback.
		m.EmitCallbacks(ctx, func(_ context.Context, r *http.Request) {
			h.ServeHTTP(discardResponse{}, r)
		}, nil, mockCallbackDelay)
	}
	// AD-9: tenant webhook hosts are allow-listed once at startup, when the
	// tenants are loaded (AD-11).
	dispatcher := webhook.NewDispatcher(st, tenants, newProviderClient(callbackHosts(tenants.CallbackURLs())...), nil)

	go func() { _ = q.Run(ctx) }()
	stopDispatcher := startDispatcher(dispatcher)
	go purgeLoop(ctx, st)

	slog.Info("listening", "provider", prov.Name())
	if err := run(ctx, ln, h, q, stopDispatcher); err != nil {
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

// startDispatcher runs d in its own goroutine and returns the function that
// stops it: it cancels Run (a notification in flight is abandoned with the
// process) and waits for Run to return or ctx to end.
func startDispatcher(d *webhook.Dispatcher) func(context.Context) error {
	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := d.Run(runCtx); err != nil {
			slog.Error("webhook dispatcher stopped", "error_class", "dispatcher")
		}
	}()
	return func(ctx context.Context) error {
		cancel()
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// callbackHosts returns the host of every absolute http(s) URL in urls.
func callbackHosts(urls []string) []string {
	var hosts []string
	for _, raw := range urls {
		if u, err := url.Parse(raw); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
			hosts = append(hosts, u.Host)
		}
	}
	return hosts
}

// discardResponse is the ResponseWriter of an in-process mock callback: the
// mock has no use for the answer.
type discardResponse struct{}

func (discardResponse) Header() http.Header         { return http.Header{} }
func (discardResponse) WriteHeader(int)             {}
func (discardResponse) Write(b []byte) (int, error) { return len(b), nil }

// run serves h on ln until ctx is cancelled, then stops the listener first,
// closes the queue second and stops the webhook dispatcher last, bounded by
// shutdownTimeout. The listener is injected so tests can share this exact
// serving path.
func run(ctx context.Context, ln net.Listener, h http.Handler, q queue.Queue, stopDispatcher func(context.Context) error) error {
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
		stopErr := stopDispatcher(shutdownCtx)
		return errors.Join(shutErr, closeErr, stopErr)
	}
}
