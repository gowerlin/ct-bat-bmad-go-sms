package store

import (
	"errors"
	"strings"
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

func TestTemplatesAreTenantScoped(t *testing.T) {
	s, c := newStore()
	if _, err := s.CreateTemplate("", Template{ID: "tpl_1"}); !errors.Is(err, ErrEmptyTenant) {
		t.Fatalf("create empty tenant: %v", err)
	}
	if _, err := s.CreateTemplate("a", Template{}); err == nil {
		t.Fatal("create without id: want error")
	}
	got, err := s.CreateTemplate("a", Template{ID: "tpl_1", TenantID: "b", Name: "n", Body: "hi {{x}}"})
	if err != nil || got.TenantID != "a" || !got.CreatedAt.Equal(c.t) {
		t.Fatalf("create: %+v %v", got, err)
	}
	if _, err := s.CreateTemplate("a", Template{ID: "tpl_1"}); err == nil {
		t.Fatal("duplicate id: want error")
	}

	for _, tenant := range []string{"", "b"} {
		want := ErrNotFound
		if tenant == "" {
			want = ErrEmptyTenant
		}
		if _, err := s.GetTemplate(tenant, "tpl_1"); !errors.Is(err, want) {
			t.Fatalf("get as %q: %v", tenant, err)
		}
		if _, err := s.ListTemplates(tenant); tenant == "" && !errors.Is(err, ErrEmptyTenant) {
			t.Fatalf("list as %q: %v", tenant, err)
		}
		if err := s.DeleteTemplate(tenant, "tpl_1"); !errors.Is(err, want) {
			t.Fatalf("delete as %q: %v", tenant, err)
		}
	}
	if list, _ := s.ListTemplates("b"); len(list) != 0 {
		t.Fatalf("tenant b sees %v", list)
	}
	if g, err := s.GetTemplate("a", "tpl_1"); err != nil || g.Body != "hi {{x}}" {
		t.Fatalf("get: %+v %v", g, err)
	}
	if err := s.DeleteTemplate("a", "tpl_1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetTemplate("a", "tpl_1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}

func TestListTemplatesNewestFirst(t *testing.T) {
	s, c := newStore()
	for _, id := range []string{"tpl_a", "tpl_b"} {
		if _, err := s.CreateTemplate("a", Template{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	c.t = c.t.Add(time.Minute)
	if _, err := s.CreateTemplate("a", Template{ID: "tpl_c"}); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListTemplates("a")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, tp := range list {
		ids = append(ids, tp.ID)
	}
	if want := "tpl_c tpl_b tpl_a"; strings.Join(ids, " ") != want {
		t.Fatalf("order = %v, want %s", ids, want)
	}
}

func TestConcurrentTemplateAccess(t *testing.T) {
	s, _ := newStore()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, _ := NewID("tpl_")
			s.CreateTemplate("a", Template{ID: id})
			s.ListTemplates("a")
			s.GetTemplate("a", id)
			s.DeleteTemplate("a", id)
		}()
	}
	wg.Wait()
	if list, _ := s.ListTemplates("a"); len(list) != 0 {
		t.Fatalf("left %d templates", len(list))
	}
}

func messageIDs(page MessagePage) string {
	var ids []string
	for _, m := range page.Messages {
		ids = append(ids, m.ID)
	}
	return strings.Join(ids, " ")
}

func TestListMessagesFiltersByTenantAndStatus(t *testing.T) {
	s, c := newStore()
	create(t, s, "msg_a1", "a")
	c.t = c.t.Add(time.Second)
	create(t, s, "msg_b1", "b")
	c.t = c.t.Add(time.Second)
	create(t, s, "msg_a2", "a")
	c.t = c.t.Add(time.Second)
	create(t, s, "msg_a3", "a")
	if _, err := s.Transition("msg_a1", StatusQueued, StatusSending, Patch{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Transition("msg_b1", StatusQueued, StatusSending, Patch{}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.ListMessages("", MessageFilter{}); !errors.Is(err, ErrEmptyTenant) {
		t.Fatalf("empty tenant: %v", err)
	}
	all, err := s.ListMessages("a", MessageFilter{})
	if err != nil || messageIDs(all) != "msg_a3 msg_a2 msg_a1" || all.NextCursor != "" {
		t.Fatalf("all of a = %q next=%q err=%v", messageIDs(all), all.NextCursor, err)
	}
	queued, _ := s.ListMessages("a", MessageFilter{Status: StatusQueued})
	if messageIDs(queued) != "msg_a3 msg_a2" {
		t.Fatalf("queued of a = %q", messageIDs(queued))
	}
	sending, _ := s.ListMessages("a", MessageFilter{Status: StatusSending})
	if messageIDs(sending) != "msg_a1" {
		t.Fatalf("sending of a = %q (must not include tenant b)", messageIDs(sending))
	}
	if none, _ := s.ListMessages("c", MessageFilter{}); len(none.Messages) != 0 {
		t.Fatalf("unknown tenant sees %q", messageIDs(none))
	}
}

func TestListMessagesTiesBreakOnHigherID(t *testing.T) {
	s, _ := newStore()
	for _, id := range []string{"msg_b", "msg_c", "msg_a"} {
		create(t, s, id, "a")
	}
	page, _ := s.ListMessages("a", MessageFilter{})
	if want := "msg_c msg_b msg_a"; messageIDs(page) != want {
		t.Fatalf("order = %q, want %s", messageIDs(page), want)
	}
}

func TestListMessagesCursorPaging(t *testing.T) {
	s, c := newStore()
	for _, id := range []string{"msg_1", "msg_2", "msg_3", "msg_4", "msg_5"} {
		create(t, s, id, "a")
		create(t, s, id+"b", "b")
		c.t = c.t.Add(time.Second)
	}

	var got []string
	f := MessageFilter{Limit: 2}
	for pages := 0; ; pages++ {
		if pages > 3 {
			t.Fatal("paging does not terminate")
		}
		page, err := s.ListMessages("a", f)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Messages) > 2 {
			t.Fatalf("page of %d exceeds limit 2", len(page.Messages))
		}
		got = append(got, messageIDs(page))
		if page.NextCursor == "" {
			break
		}
		if strings.Contains(page.NextCursor, "msg_") {
			t.Fatalf("cursor %q is not opaque", page.NextCursor)
		}
		f.Cursor = page.NextCursor
	}
	if want := "msg_5 msg_4|msg_3 msg_2|msg_1"; strings.Join(got, "|") != want {
		t.Fatalf("pages = %q, want %q", strings.Join(got, "|"), want)
	}

	// A message created after the first page does not shift later pages.
	first, _ := s.ListMessages("a", MessageFilter{Limit: 2})
	c.t = c.t.Add(time.Second)
	create(t, s, "msg_6", "a")
	next, _ := s.ListMessages("a", MessageFilter{Limit: 2, Cursor: first.NextCursor})
	if messageIDs(next) != "msg_3 msg_2" {
		t.Fatalf("page after cursor = %q", messageIDs(next))
	}

	// Tenant b's cursor positions tenant a's list but never reveals b's messages.
	bPage, _ := s.ListMessages("b", MessageFilter{Limit: 1})
	cross, _ := s.ListMessages("a", MessageFilter{Cursor: bPage.NextCursor})
	for _, m := range cross.Messages {
		if m.TenantID != "a" {
			t.Fatalf("tenant a sees %s of tenant %s", m.ID, m.TenantID)
		}
	}
}

func TestListMessagesLimitsAndBadCursor(t *testing.T) {
	s, _ := newStore()
	for i := 0; i < MaxListLimit+1; i++ {
		id, _ := NewID("msg_")
		create(t, s, id, "a")
	}
	if page, _ := s.ListMessages("a", MessageFilter{}); len(page.Messages) != DefaultListLimit || page.NextCursor == "" {
		t.Fatalf("default limit: %d messages, next=%q", len(page.Messages), page.NextCursor)
	}
	if page, _ := s.ListMessages("a", MessageFilter{Limit: MaxListLimit + 50}); len(page.Messages) != MaxListLimit {
		t.Fatalf("capped limit: %d messages", len(page.Messages))
	}
	for _, bad := range []string{"!!", "bm8tY29sb24", "eDptc2dfMQ", "MTIz"} { // "no-colon", "x:msg_1", "123"
		if _, err := s.ListMessages("a", MessageFilter{Cursor: bad}); !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("cursor %q: %v", bad, err)
		}
	}
}

func TestStatusValid(t *testing.T) {
	for _, st := range []Status{StatusQueued, StatusSending, StatusSent, StatusDelivered, StatusUndelivered, StatusFailed} {
		if !st.Valid() {
			t.Fatalf("%q should be valid", st)
		}
	}
	for _, st := range []Status{"", "QUEUED", "done"} {
		if st.Valid() {
			t.Fatalf("%q should be invalid", st)
		}
	}
}
