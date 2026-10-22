package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"example.com/sms-platform/e2e/testutil"
	"example.com/sms-platform/internal/queue"
	"example.com/sms-platform/internal/store"
)

type envelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func send(t *testing.T, env *testutil.Env, body string) (*http.Response, []byte) {
	t.Helper()
	return env.Do(t, http.MethodPost, "/v1/messages", testutil.KeyA, strings.NewReader(body))
}

func wantEnvelope(t *testing.T, resp *http.Response, body []byte, status int, code string) {
	t.Helper()
	if resp.StatusCode != status {
		t.Fatalf("status = %d, want %d; body = %s", resp.StatusCode, status, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q", ct)
	}
	var e envelope
	if err := json.Unmarshal(body, &e); err != nil || e.Error.Code != code || e.Error.Message == "" {
		t.Fatalf("body %q is not an envelope with code %q (err=%v)", body, code, err)
	}
}

func submit(t *testing.T, env *testutil.Env) string {
	t.Helper()
	resp, body := send(t, env, `{"to":"+886900000001","body":"hi"}`)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("send: %d %s", resp.StatusCode, body)
	}
	var got struct{ ID string }
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	return got.ID
}

func TestE2E_SendMessage_Returns202AndQueryable(t *testing.T) {
	env := testutil.NewTestServer(t, testutil.WithoutWorker())

	resp, body := send(t, env, `{"to":"+886900000001","body":"hi"}`)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("POST status = %d; body = %s", resp.StatusCode, body)
	}
	var sent struct{ ID, Status string }
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sent.ID, "msg_") || len(sent.ID) != len("msg_")+32 || sent.Status != "queued" {
		t.Fatalf("unexpected response %+v", sent)
	}

	resp, body = env.Get(t, "/v1/messages/"+sent.ID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d; body = %s", resp.StatusCode, body)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["id"] != sent.ID || got["status"] != "queued" {
		t.Fatalf("GET body = %s", body)
	}
	if attempts, ok := got["attempts"].(float64); !ok || attempts != 0 {
		t.Fatalf("attempts = %v, want integer 0", got["attempts"])
	}
	if got["to"] != "+886900000001" {
		t.Fatalf("to = %v, want +886900000001", got["to"])
	}
	for _, k := range []string{"created_at", "updated_at"} {
		s, _ := got[k].(string)
		ts, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatalf("%s = %q is not RFC 3339: %v", k, s, err)
		}
		if !ts.Equal(env.Clock.Now()) {
			t.Fatalf("%s = %v, want FakeClock time %v", k, ts, env.Clock.Now())
		}
	}
}

func TestE2E_SendMessage_RejectsInvalidRequests(t *testing.T) {
	env := testutil.NewTestServer(t)
	cases := map[string]string{
		"missing to":      `{"body":"hi"}`,
		"missing body":    `{"to":"+886900000001"}`,
		"blank body":      `{"to":"+886900000001","body":"   "}`,
		"not E.164":       `{"to":"0900000001","body":"hi"}`,
		"71 code points":  `{"to":"+886900000001","body":"` + strings.Repeat("字", 71) + `"}`,
		"invalid JSON":    `{"to":`,
		"empty request":   ``,
		"wrong JSON type": `[]`,
	}
	for name, payload := range cases {
		resp, body := send(t, env, payload)
		t.Run(name, func(t *testing.T) { wantEnvelope(t, resp, body, http.StatusBadRequest, "invalid_request") })
	}
	if env.Store.Len() != 0 {
		t.Fatalf("store has %d messages after only invalid requests", env.Store.Len())
	}
	// exactly 70 code points is accepted
	resp, body := send(t, env, `{"to":"+886900000001","body":"`+strings.Repeat("字", 70)+`"}`)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("70 code points: %d %s", resp.StatusCode, body)
	}
}

func TestE2E_SendMessage_QueueFullReturns503AndKeepsNothing(t *testing.T) {
	env := testutil.NewTestServer(t, testutil.WithoutWorker())
	for i := 0; i < queue.Capacity; i++ {
		submit(t, env)
	}
	resp, body := send(t, env, `{"to":"+886900000001","body":"hi"}`)
	wantEnvelope(t, resp, body, http.StatusServiceUnavailable, "queue_full")
	if env.Store.Len() != queue.Capacity {
		t.Fatalf("store has %d messages, want %d", env.Store.Len(), queue.Capacity)
	}
}

func TestE2E_SendMessage_AfterCloseReturns503(t *testing.T) {
	env := testutil.NewTestServer(t)
	if err := env.Queue.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	resp, body := send(t, env, `{"to":"+886900000001","body":"hi"}`)
	wantEnvelope(t, resp, body, http.StatusServiceUnavailable, "queue_full")
	if !strings.Contains(string(body), "service shutting down") {
		t.Fatalf("body = %s", body)
	}
}

func TestE2E_GetMessage_UnknownIsNotFound(t *testing.T) {
	env := testutil.NewTestServer(t)
	resp, body := env.Get(t, "/v1/messages/msg_doesnotexist")
	wantEnvelope(t, resp, body, http.StatusNotFound, "not_found")
}

func TestE2E_GetMessage_AfterRestartIsNotFound(t *testing.T) {
	before := testutil.NewTestServer(t)
	id := submit(t, before)

	after := testutil.NewTestServer(t) // a restart: fresh in-memory store
	resp, body := after.Get(t, "/v1/messages/"+id)
	wantEnvelope(t, resp, body, http.StatusNotFound, "not_found")
}

func TestE2E_Errors_UnroutedRequestsUseEnvelope(t *testing.T) {
	env := testutil.NewTestServer(t)

	resp, body := env.Get(t, "/nope")
	wantEnvelope(t, resp, body, http.StatusNotFound, "not_found")

	resp, body = env.Do(t, http.MethodDelete, "/v1/messages", testutil.KeyA, nil)
	wantEnvelope(t, resp, body, http.StatusMethodNotAllowed, "invalid_request")
	if allow := resp.Header.Get("Allow"); !strings.Contains(allow, "POST") {
		t.Fatalf("Allow = %q, want it to list POST", allow)
	}

	resp, body = env.Do(t, http.MethodPost, "/healthz", testutil.KeyA, nil)
	wantEnvelope(t, resp, body, http.StatusMethodNotAllowed, "invalid_request")
	if allow := resp.Header.Get("Allow"); !strings.Contains(allow, "GET") {
		t.Fatalf("Allow = %q, want it to list GET", allow)
	}
}

func TestE2E_PurgeExpired_RemovesOnlyOldTerminalMessages(t *testing.T) {
	env := testutil.NewTestServer(t, testutil.WithoutWorker())
	done, open := submit(t, env), submit(t, env)

	if _, err := env.Store.Transition(done, store.StatusQueued, store.StatusSending, store.Patch{}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Store.Transition(done, store.StatusSending, store.StatusFailed, store.Patch{FailureReason: "永久失敗"}); err != nil {
		t.Fatal(err)
	}

	env.Clock.Advance(23 * time.Hour)
	env.Store.PurgeExpired(env.Clock.Now())
	if resp, body := env.Get(t, "/v1/messages/"+done); resp.StatusCode != http.StatusOK {
		t.Fatalf("terminal message purged before 24h: %d %s", resp.StatusCode, body)
	}

	env.Clock.Advance(2 * time.Hour)
	env.Store.PurgeExpired(env.Clock.Now())
	resp, body := env.Get(t, "/v1/messages/"+done)
	wantEnvelope(t, resp, body, http.StatusNotFound, "not_found")
	if resp, body := env.Get(t, "/v1/messages/"+open); resp.StatusCode != http.StatusOK {
		t.Fatalf("non-terminal message purged: %d %s", resp.StatusCode, body)
	}
}
