package e2e

// stories.yaml id 11（CH5 5-02 手寫紅燈骨架，T0009 改成真實斷言）：
// 供應商回呼把 sent 改成 delivered，租戶 Webhook 收到 HMAC 簽章通知，
// 重複回呼回 200 且不重複通知（AD-3、AD-7、D013、D015）。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"example.com/sms-platform/e2e/testutil"
	"example.com/sms-platform/internal/provider"
	"example.com/sms-platform/internal/store"
	"example.com/sms-platform/internal/webhook"
)

// webhookSecret is the fake signing key of every webhook test (D013).
const webhookSecret = "test-secret"

// callback posts a supplier callback to provider name, without an API key.
func callback(t *testing.T, env *testutil.Env, name string) (*http.Response, []byte) {
	t.Helper()
	return env.Do(t, http.MethodPost, "/v1/providers/"+name+"/callbacks", "",
		strings.NewReader(`{"note":"FakeProvider ignores the body"}`))
}

// sentMessage submits a message as tenant A and waits for the worker to move
// it to sent, returning it with its real provider_ref.
func sentMessage(t *testing.T, env *testutil.Env) store.Message {
	t.Helper()
	id := submit(t, env)
	m := env.AwaitStatus(t, id, store.StatusSent)
	if m.ProviderRef == "" {
		t.Fatalf("message %s is sent without provider_ref", id)
	}
	return m
}

func TestE2E_Webhook_DeliveredUpdatesStatusAndNotifiesTenantOnce(t *testing.T) {
	env := testutil.NewTestServer(t)
	recv := env.NewReceiver(t, testutil.TenantA, webhookSecret)
	m := sentMessage(t, env)
	report := provider.DeliveryReport{
		ProviderRef: m.ProviderRef, Status: provider.StatusDelivered, ReceivedAt: env.Clock.Now().Add(time.Second),
	}

	// (2) The first callback moves the message: 204, and GET shows delivered.
	env.Provider.EnqueueReport(report)
	resp, body := callback(t, env, env.Provider.Name())
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("POST callbacks = %d %s, want 204", resp.StatusCode, body)
	}
	resp, body = env.Get(t, "/v1/messages/"+m.ID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET message = %d %s", resp.StatusCode, body)
	}
	var got struct{ ID, Status string }
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != m.ID || got.Status != "delivered" {
		t.Fatalf("GET message = %s, want status delivered", body)
	}

	// (3) The tenant gets exactly one signed notification.
	env.AwaitOutboxIdle(t)
	notes := recv.Notifications()
	if len(notes) != 1 {
		t.Fatalf("receiver got %d notifications, want 1", len(notes))
	}
	n := notes[0]
	if want := fmt.Sprintf(`{"id":%q,"status":"delivered"}`, m.ID); string(n.Body) != want {
		t.Fatalf("notification body = %s, want %s", n.Body, want)
	}
	if n.ContentType != "application/json" || !strings.HasPrefix(n.Signature, "sha256=") {
		t.Fatalf("notification headers: Content-Type %q, X-Signature %q", n.ContentType, n.Signature)
	}
	if !webhook.Verify(webhookSecret, n.Body, n.Signature) {
		t.Fatalf("X-Signature %q does not verify with the tenant secret", n.Signature)
	}
	tampered := []byte(strings.Replace(string(n.Body), "delivered", "undelivered", 1))
	if webhook.Verify(webhookSecret, tampered, n.Signature) {
		t.Fatal("X-Signature verifies a tampered body")
	}
	if webhook.Verify("wrong-secret", n.Body, n.Signature) {
		t.Fatal("X-Signature verifies with a wrong secret")
	}

	// (4) The same callback again: 200, status unchanged, still one notification.
	env.Provider.EnqueueReport(report)
	resp, body = callback(t, env, env.Provider.Name())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("repeated POST callbacks = %d %s, want 200", resp.StatusCode, body)
	}
	env.AwaitOutboxIdle(t)
	if n := len(recv.Notifications()); n != 1 {
		t.Fatalf("receiver got %d notifications after the repeat, want 1", n)
	}
	if outbox := env.Store.Notifications(m.ID); len(outbox) != 1 || outbox[0].State != store.NotificationDone {
		t.Fatalf("outbox for %s = %+v, want one done entry", m.ID, outbox)
	}
	if after, _ := env.Store.GetByID(m.ID); after.Status != store.StatusDelivered {
		t.Fatalf("status after the repeat = %s", after.Status)
	}
}

func TestE2E_Webhook_UnknownProviderReturns404(t *testing.T) {
	env := testutil.NewTestServer(t)
	resp, body := callback(t, env, "no-such-provider")
	assertEnvelope(t, resp, body, http.StatusNotFound, "not_found")
}

func TestE2E_Webhook_UnauthorizedCallbackReturns401(t *testing.T) {
	env := testutil.NewTestServer(t)
	recv := env.NewReceiver(t, testutil.TenantA, webhookSecret)
	m := sentMessage(t, env)
	env.Provider.EnqueueCallbackError(fmt.Errorf("fake: wrong shared secret: %w", provider.ErrUnauthorized))
	resp, body := callback(t, env, env.Provider.Name())
	assertEnvelope(t, resp, body, http.StatusUnauthorized, "unauthorized")
	if after, _ := env.Store.GetByID(m.ID); after.Status != store.StatusSent {
		t.Fatalf("status after a rejected callback = %s, want sent", after.Status)
	}
	env.AwaitOutboxIdle(t)
	if n := len(recv.Notifications()); n != 0 {
		t.Fatalf("receiver got %d notifications", n)
	}
}

func TestE2E_Webhook_UnknownProviderRefReturns404(t *testing.T) {
	env := testutil.NewTestServer(t)
	sentMessage(t, env)
	env.Provider.EnqueueReport(provider.DeliveryReport{ProviderRef: "no-such-ref", Status: provider.StatusDelivered})
	resp, body := callback(t, env, env.Provider.Name())
	assertEnvelope(t, resp, body, http.StatusNotFound, "not_found")
}

// assertEnvelope checks status and the error envelope's code.
func assertEnvelope(t *testing.T, resp *http.Response, body []byte, status int, code string) {
	t.Helper()
	if resp.StatusCode != status {
		t.Fatalf("status = %d %s, want %d", resp.StatusCode, body, status)
	}
	var env struct {
		Error struct{ Code, Message string }
	}
	if err := json.Unmarshal(body, &env); err != nil || env.Error.Code != code || env.Error.Message == "" {
		t.Fatalf("envelope = %s, want code %s", body, code)
	}
}
