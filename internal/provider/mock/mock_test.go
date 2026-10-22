package mock

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"example.com/sms-platform/internal/provider"
	"example.com/sms-platform/internal/provider/providertest"
)

// TestContract runs the shared provider contract (AD-1). The mock has no
// callback authentication, so UnauthorizedCallback stays nil (D018; known gap
// against AD-7).
func TestContract(t *testing.T) {
	providertest.Run(t, providertest.Harness{
		New: func(t *testing.T, o providertest.Outcome) provider.Provider {
			p := NewWithClock(fixedClock)
			switch o {
			case providertest.FailTransient:
				p.FailTransient(0)
			case providertest.FailPermanent:
				p.FailPermanent()
			}
			return p
		},
		Callback: func(t *testing.T, ref string) (*http.Request, provider.Status) {
			return callbackRequest(url.Values{"ref": {ref}, "status": {"delivered"}}), provider.StatusDelivered
		},
		MalformedCallback: func(t *testing.T) *http.Request {
			return callbackRequest(url.Values{"status": {"delivered"}})
		},
	})
}

func callbackRequest(form url.Values) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/v1/providers/mock/callbacks", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

var msg = provider.OutboundMessage{ID: "m1", To: "+886900000001", Body: "hi"}

func fixedClock() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }

func TestSend_DefaultSucceedsWithSequentialRefs(t *testing.T) {
	p := NewWithClock(fixedClock)
	for _, want := range []string{"mock-1", "mock-2"} {
		r, err := p.Send(context.Background(), msg)
		if err != nil || r.ProviderRef != want {
			t.Fatalf("got %q, %v; want %q", r.ProviderRef, err, want)
		}
	}
}

func TestFailTransient_NThenRecovers(t *testing.T) {
	p := New()
	p.FailTransient(2)
	for i := 0; i < 2; i++ {
		if _, err := p.Send(context.Background(), msg); !errors.Is(err, provider.ErrTransient) {
			t.Fatalf("send %d: err = %v, want ErrTransient", i, err)
		}
	}
	r, err := p.Send(context.Background(), msg)
	if err != nil || r.ProviderRef != "mock-1" {
		t.Fatalf("after recovery: %q, %v", r.ProviderRef, err)
	}
}

func TestFailTransient_PersistsUntilReset(t *testing.T) {
	p := New()
	p.FailTransient(0)
	for i := 0; i < 5; i++ {
		if _, err := p.Send(context.Background(), msg); !errors.Is(err, provider.ErrTransient) {
			t.Fatalf("send %d: err = %v", i, err)
		}
	}
	p.Reset()
	if _, err := p.Send(context.Background(), msg); err != nil {
		t.Fatalf("after Reset: %v", err)
	}
}

func TestFailPermanent(t *testing.T) {
	p := New()
	p.FailPermanent()
	_, err := p.Send(context.Background(), msg)
	if !errors.Is(err, provider.ErrPermanent) || errors.Is(err, provider.ErrTransient) {
		t.Fatalf("err = %v, want ErrPermanent only", err)
	}
	if strings.Contains(err.Error(), msg.To) || strings.Contains(err.Error(), msg.Body) {
		t.Fatalf("error leaks recipient or content: %v", err)
	}
}

func TestTimeout_BlocksUntilContextEnds(t *testing.T) {
	p := New()
	p.Timeout()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := p.Send(ctx, msg); done <- err }()
	select {
	case err := <-done:
		t.Fatalf("returned before cancel: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestTimeout_DeadlineExceeded(t *testing.T) {
	p := New()
	p.Timeout()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := p.Send(ctx, msg); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
}

func TestDelay_UsesInjectedWaitAndHonoursCancel(t *testing.T) {
	p := New()
	fire := make(chan time.Time)
	var got time.Duration
	p.SetAfter(func(d time.Duration) <-chan time.Time { got = d; return fire })
	p.Delay(3 * time.Second)

	done := make(chan error, 1)
	go func() { _, err := p.Send(context.Background(), msg); done <- err }()
	fire <- time.Time{}
	if err := <-done; err != nil || got != 3*time.Second {
		t.Fatalf("err = %v, waited %v", err, got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _, err := p.Send(ctx, msg); done <- err }()
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestSend_CancelledContextShortCircuits(t *testing.T) {
	p := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Send(ctx, msg); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestInjection_ConcurrentSafe(t *testing.T) {
	p := New()
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				p.FailTransient(1)
				_, _ = p.Send(context.Background(), msg)
				p.Reset()
			}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
}

func TestEmitCallbacks_DeliveredAfterWaitThroughSink(t *testing.T) {
	p := NewWithClock(fixedClock)
	got := make(chan *http.Request, 4)
	waited := make(chan time.Duration, 4)
	wait := func(_ context.Context, d time.Duration) error {
		waited <- d
		return nil
	}
	p.EmitCallbacks(context.Background(), func(_ context.Context, r *http.Request) { got <- r }, wait, 3*time.Second)

	p.FailPermanent()
	if _, err := p.Send(context.Background(), provider.OutboundMessage{ID: "m0", To: "+886900000001"}); err == nil {
		t.Fatal("injected failure did not fail")
	}
	p.Reset()
	receipt, err := p.Send(context.Background(), provider.OutboundMessage{ID: "m1", To: "+886900000001"})
	if err != nil {
		t.Fatal(err)
	}

	var r *http.Request
	select {
	case r = <-got:
	case <-time.After(10 * time.Second):
		t.Fatal("no callback reached the sink")
	}
	if d := <-waited; d != 3*time.Second {
		t.Fatalf("waited %v", d)
	}
	if r.Method != http.MethodPost || r.URL.Path != CallbackPath {
		t.Fatalf("callback %s %s", r.Method, r.URL.Path)
	}
	rep, err := p.ParseCallback(r)
	if err != nil || rep.ProviderRef != receipt.ProviderRef || rep.Status != provider.StatusDelivered {
		t.Fatalf("parsed %+v %v (receipt %s)", rep, err, receipt.ProviderRef)
	}
	select {
	case extra := <-got:
		t.Fatalf("failed Send emitted a callback: %v", extra.URL)
	default:
	}
}

func TestEmitCallbacks_CancelledWaitAndNilSinkEmitNothing(t *testing.T) {
	p := NewWithClock(fixedClock)
	got := make(chan *http.Request, 1)
	done := make(chan struct{})
	wait := func(context.Context, time.Duration) error {
		defer close(done)
		return context.Canceled
	}
	p.EmitCallbacks(context.Background(), func(_ context.Context, r *http.Request) { got <- r }, wait, time.Second)
	if _, err := p.Send(context.Background(), provider.OutboundMessage{ID: "m1"}); err != nil {
		t.Fatal(err)
	}
	<-done
	p.EmitCallbacks(context.Background(), nil, nil, 0)
	if _, err := p.Send(context.Background(), provider.OutboundMessage{ID: "m2"}); err != nil {
		t.Fatal(err)
	}
	select {
	case r := <-got:
		t.Fatalf("callback emitted: %v", r.URL)
	default:
	}
}
