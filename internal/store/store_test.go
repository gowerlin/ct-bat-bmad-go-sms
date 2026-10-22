package store

import (
	"errors"
	"sync"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newStore() (*Memory, *clock) {
	c := &clock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	return New(c.now), c
}

func create(t *testing.T, s *Memory, id, tenant string) {
	t.Helper()
	if _, err := s.Create(Message{ID: id, TenantID: tenant, To: "+886900000001", Body: "hi"}); err != nil {
		t.Fatal(err)
	}
}

func TestTenantScopedIsolation(t *testing.T) {
	s, _ := newStore()
	create(t, s, "msg_1", "a")
	if _, err := s.Get("", "msg_1"); !errors.Is(err, ErrEmptyTenant) {
		t.Fatalf("empty tenant: %v", err)
	}
	if _, err := s.Get("b", "msg_1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross tenant: %v", err)
	}
	m, err := s.Get("a", "msg_1")
	if err != nil || m.Status != StatusQueued {
		t.Fatalf("get: %v %v", m, err)
	}
}

func TestTransitionTable(t *testing.T) {
	s, _ := newStore()
	create(t, s, "m", "a")
	if _, err := s.Transition("m", StatusQueued, StatusSent, Patch{}); !errors.Is(err, ErrIllegalTransition) {
		t.Fatalf("queued->sent: %v", err)
	}
	if _, err := s.Transition("m", StatusSending, StatusSent, Patch{}); !errors.Is(err, ErrIllegalTransition) {
		t.Fatalf("CAS mismatch: %v", err)
	}
	if _, err := s.Transition("m", StatusQueued, StatusSending, Patch{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Transition("m", StatusSending, StatusFailed, Patch{}); !errors.Is(err, ErrIllegalTransition) {
		t.Fatalf("failed without reason: %v", err)
	}
	m, err := s.Transition("m", StatusSending, StatusSent, Patch{Attempts: 2, ProviderRef: "r1"})
	if err != nil || m.Attempts != 2 || m.ProviderRef != "r1" {
		t.Fatalf("sent: %+v %v", m, err)
	}
	m, err = s.Transition("m", StatusSent, StatusDelivered, Patch{})
	if err != nil || m.TerminalAt.IsZero() {
		t.Fatalf("delivered: %+v %v", m, err)
	}
	if _, err := s.Transition("m", StatusDelivered, StatusSent, Patch{}); !errors.Is(err, ErrIllegalTransition) {
		t.Fatalf("terminal is final: %v", err)
	}
	if _, err := s.Transition("nope", StatusQueued, StatusSending, Patch{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
}

func TestPurgeExpired(t *testing.T) {
	s, c := newStore()
	create(t, s, "done", "a")
	create(t, s, "open", "a")
	s.Transition("done", StatusQueued, StatusSending, Patch{})
	s.Transition("done", StatusSending, StatusFailed, Patch{FailureReason: "x"})

	if n := s.PurgeExpired(c.t.Add(23 * time.Hour)); n != 0 {
		t.Fatalf("purged early: %d", n)
	}
	c.t = c.t.Add(25 * time.Hour)
	if n := s.PurgeExpired(c.t); n != 1 {
		t.Fatalf("purged %d, want 1", n)
	}
	if _, err := s.Get("a", "done"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("done still there: %v", err)
	}
	if _, err := s.Get("a", "open"); err != nil {
		t.Fatalf("non-terminal removed: %v", err)
	}
}

func TestConcurrentTransitionOnlyOneWins(t *testing.T) {
	s, _ := newStore()
	create(t, s, "m", "a")
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Transition("m", StatusQueued, StatusSending, Patch{}); err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
			s.Get("a", "m")
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("wins = %d", wins)
	}
}

func TestNewID(t *testing.T) {
	a, _ := NewID("msg_")
	b, _ := NewID("msg_")
	if len(a) != 4+32 || a == b {
		t.Fatalf("ids %q %q", a, b)
	}
}
