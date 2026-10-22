package e2e

import (
	"context"
	"errors"
	"testing"
	"time"

	"example.com/sms-platform/internal/provider"
	"example.com/sms-platform/internal/provider/mock"
)

func TestE2E_MockAdapter_FaultInjection(t *testing.T) {
	msg := provider.OutboundMessage{ID: "msg_1", TenantID: "t1", To: "+886900000001", Body: "hello"}

	t.Run("fail transient then recover", func(t *testing.T) {
		p := mock.New()
		p.FailTransient(2)
		for i := 0; i < 2; i++ {
			if _, err := p.Send(context.Background(), msg); !errors.Is(err, provider.ErrTransient) {
				t.Fatalf("send %d: err = %v, want ErrTransient", i, err)
			}
		}
		if r, err := p.Send(context.Background(), msg); err != nil || r.ProviderRef != "mock-1" {
			t.Fatalf("after recovery: %q, %v", r.ProviderRef, err)
		}
	})

	t.Run("fail permanent", func(t *testing.T) {
		p := mock.New()
		p.FailPermanent()
		if _, err := p.Send(context.Background(), msg); !errors.Is(err, provider.ErrPermanent) {
			t.Fatalf("err = %v, want ErrPermanent", err)
		}
		p.Reset()
		if _, err := p.Send(context.Background(), msg); err != nil {
			t.Fatalf("after Reset: %v", err)
		}
	})

	t.Run("timeout ends with ctx error", func(t *testing.T) {
		p := mock.New()
		p.Timeout()
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { _, err := p.Send(ctx, msg); done <- err }()
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	})

	t.Run("delay uses injected wait", func(t *testing.T) {
		p := mock.New()
		fire := make(chan time.Time)
		var waited time.Duration
		p.SetAfter(func(d time.Duration) <-chan time.Time { waited = d; return fire })
		p.Delay(5 * time.Second)
		done := make(chan error, 1)
		go func() { _, err := p.Send(context.Background(), msg); done <- err }()
		fire <- time.Time{}
		if err := <-done; err != nil || waited != 5*time.Second {
			t.Fatalf("err = %v, waited %v", err, waited)
		}
	})
}
