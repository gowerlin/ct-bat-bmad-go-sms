package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"example.com/sms-platform/e2e/testutil"
	"example.com/sms-platform/internal/provider"
	"example.com/sms-platform/internal/store"
)

type messageList struct {
	Messages []struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		FailureReason string `json:"failure_reason"`
		CreatedAt     string `json:"created_at"`
	} `json:"messages"`
	NextCursor string `json:"next_cursor"`
}

// sendAs submits a message as apiKey and advances the FakeClock one second so
// that every message has its own created_at.
func sendAs(t *testing.T, env *testutil.Env, apiKey string) string {
	t.Helper()
	resp, body := env.Do(t, http.MethodPost, "/v1/messages", apiKey,
		strings.NewReader(`{"to":"+886900000001","body":"hi"}`))
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("send: %d %s", resp.StatusCode, body)
	}
	var got struct{ ID string }
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	env.Clock.Advance(time.Second)
	return got.ID
}

// deliver moves a queued message through sending to sent or failed the way the
// queue's send worker does: the FakeProvider's answer decides the outcome.
// Because the test runs WithoutWorker (some messages must stay queued), the
// transitions go through store.System directly.
func deliver(t *testing.T, env *testutil.Env, id string, providerErr error) {
	t.Helper()
	m, err := env.Store.GetByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.Store.Transition(id, store.StatusQueued, store.StatusSending, store.Patch{Attempts: 1}); err != nil {
		t.Fatal(err)
	}
	if providerErr != nil {
		env.Provider.Enqueue(testutil.Response{Err: providerErr})
	}
	receipt, sendErr := env.Provider.Send(context.Background(),
		provider.OutboundMessage{ID: m.ID, TenantID: m.TenantID, To: m.To, Body: m.Body})
	to, p := store.StatusSent, store.Patch{ProviderRef: receipt.ProviderRef}
	if sendErr != nil {
		to, p = store.StatusFailed, store.Patch{FailureReason: "provider_rejected"}
	}
	if _, err := env.Store.Transition(id, store.StatusSending, to, p); err != nil {
		t.Fatal(err)
	}
}

func listAs(t *testing.T, env *testutil.Env, apiKey, query string) messageList {
	t.Helper()
	resp, body := env.Do(t, http.MethodGet, "/v1/messages"+query, apiKey, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list %s: %d %s", query, resp.StatusCode, body)
	}
	var got messageList
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("list %s: %v in %s", query, err, body)
	}
	return got
}

// wantIDs checks the list is exactly want, in order, every item has status
// (when given) and created_at is non-increasing.
func wantIDs(t *testing.T, got messageList, status string, want ...string) {
	t.Helper()
	var ids []string
	for i, m := range got.Messages {
		ids = append(ids, m.ID)
		if status != "" && m.Status != status {
			t.Errorf("%s has status %q, want %q", m.ID, m.Status, status)
		}
		if i > 0 && m.CreatedAt > got.Messages[i-1].CreatedAt {
			t.Errorf("%s (%s) is newer than the item before it (%s)", m.ID, m.CreatedAt, got.Messages[i-1].CreatedAt)
		}
	}
	if strings.Join(ids, " ") != strings.Join(want, " ") {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}

func TestE2E_ListMessages_FilterByStatus(t *testing.T) {
	env := testutil.NewTestServer(t, testutil.WithoutWorker())

	a1 := sendAs(t, env, testutil.KeyA)
	b1 := sendAs(t, env, testutil.KeyB)
	a2 := sendAs(t, env, testutil.KeyA)
	a3 := sendAs(t, env, testutil.KeyA)
	b2 := sendAs(t, env, testutil.KeyB)
	a4 := sendAs(t, env, testutil.KeyA)

	deliver(t, env, a1, nil)
	deliver(t, env, a2, provider.ErrPermanent)
	deliver(t, env, a3, nil)
	deliver(t, env, b1, nil)

	// Filtered: only the caller's messages in that status, newest first.
	wantIDs(t, listAs(t, env, testutil.KeyA, "?status=sent"), "sent", a3, a1)
	wantIDs(t, listAs(t, env, testutil.KeyA, "?status=queued"), "queued", a4)
	failed := listAs(t, env, testutil.KeyA, "?status=failed")
	wantIDs(t, failed, "failed", a2)
	if failed.Messages[0].FailureReason == "" {
		t.Fatalf("failed message has no failure_reason: %+v", failed.Messages[0])
	}
	wantIDs(t, listAs(t, env, testutil.KeyA, "?status=delivered"), "")
	wantIDs(t, listAs(t, env, testutil.KeyB, "?status=sent"), "sent", b1)
	wantIDs(t, listAs(t, env, testutil.KeyB, "?status=queued"), "queued", b2)

	// No status: every message of the caller, still newest first.
	all := listAs(t, env, testutil.KeyA, "")
	wantIDs(t, all, "", a4, a3, a2, a1)
	if all.NextCursor != "" {
		t.Fatalf("single page has next_cursor %q", all.NextCursor)
	}

	// Cross-tenant: neither tenant's list mentions the other's ids.
	resp, body := env.Do(t, http.MethodGet, "/v1/messages", testutil.KeyB, nil)
	for _, id := range []string{a1, a2, a3, a4} {
		if strings.Contains(string(body), id) {
			t.Fatalf("tenant B's list (%d) reveals tenant A's %s: %s", resp.StatusCode, id, body)
		}
	}

	// Paging with an opaque cursor covers the same list.
	first := listAs(t, env, testutil.KeyA, "?limit=3")
	wantIDs(t, first, "", a4, a3, a2)
	if first.NextCursor == "" {
		t.Fatal("first page of 3 out of 4 has no next_cursor")
	}
	wantIDs(t, listAs(t, env, testutil.KeyA, "?limit=3&cursor="+first.NextCursor), "", a1)

	// Invalid status is a 400 envelope that does not echo the input.
	for _, q := range []string{"?status=bogus-status", "?status=", "?status=SENT"} {
		resp, body := env.Do(t, http.MethodGet, "/v1/messages"+q, testutil.KeyA, nil)
		wantEnvelope(t, resp, body, http.StatusBadRequest, "invalid_request")
		if strings.Contains(string(body), "bogus-status") || strings.Contains(string(body), "SENT") {
			t.Fatalf("400 body echoes the input: %s", body)
		}
	}
	for _, q := range []string{"?limit=0", "?limit=x", "?cursor=not-a-cursor"} {
		resp, body := env.Do(t, http.MethodGet, "/v1/messages"+q, testutil.KeyA, nil)
		wantEnvelope(t, resp, body, http.StatusBadRequest, "invalid_request")
	}

	// The list is behind authentication like every /v1 route.
	resp, body = env.Do(t, http.MethodGet, "/v1/messages?status=sent", "", nil)
	wantEnvelope(t, resp, body, http.StatusUnauthorized, "unauthorized")
}
