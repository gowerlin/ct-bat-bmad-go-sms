package queue

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"example.com/sms-platform/internal/store"
)

func now() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }

func TestSubmitStoresQueuedMessage(t *testing.T) {
	st := store.New(now)
	q := NewInProcess(st)
	id, err := q.Submit(context.Background(), "default", Draft{To: "+886900000001", Body: "hi"})
	if err != nil || !strings.HasPrefix(id, "msg_") {
		t.Fatalf("id=%q err=%v", id, err)
	}
	m, err := st.Get("default", id)
	if err != nil || m.Status != store.StatusQueued {
		t.Fatalf("msg=%+v err=%v", m, err)
	}
}

func TestSubmitFullLeavesNoMessage(t *testing.T) {
	st := store.New(now)
	q := newInProcess(st, 2)
	for i := 0; i < 2; i++ {
		if _, err := q.Submit(context.Background(), "default", Draft{To: "+886900000001", Body: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := q.Submit(context.Background(), "default", Draft{To: "+886900000001", Body: "x"}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("err=%v", err)
	}
	if st.Len() != 2 {
		t.Fatalf("store has %d messages, want 2", st.Len())
	}
}

func TestSubmitStoreFailureReleasesReservation(t *testing.T) {
	st := store.New(now)
	q := newInProcess(st, 1)
	// an empty tenant makes Create fail
	if _, err := q.Submit(context.Background(), "", Draft{}); err == nil {
		t.Fatal("want error")
	}
	if _, err := q.Submit(context.Background(), "default", Draft{}); err != nil {
		t.Fatalf("slot not released: %v", err)
	}
}

func TestCloseRejectsSubmit(t *testing.T) {
	q := NewInProcess(store.New(now))
	q.Close(context.Background())
	if _, err := q.Submit(context.Background(), "default", Draft{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("err=%v", err)
	}
}
