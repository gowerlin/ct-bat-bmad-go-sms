package mock

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"example.com/sms-platform/internal/provider"
)

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
