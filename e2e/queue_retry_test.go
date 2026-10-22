package e2e

// stories.yaml id 9、10：送信 worker 重試後成功、重試用盡標記 failed（T0008）。

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"example.com/sms-platform/e2e/testutil"
	"example.com/sms-platform/internal/provider"
	"example.com/sms-platform/internal/queue"
	"example.com/sms-platform/internal/store"
)

type queueMessage struct {
	Status        string `json:"status"`
	Attempts      int    `json:"attempts"`
	FailureReason string `json:"failure_reason"`
}

func getQueueMessage(t *testing.T, env *testutil.Env, id string) queueMessage {
	t.Helper()
	resp, body := env.Get(t, "/v1/messages/"+id)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %d %s", id, resp.StatusCode, body)
	}
	var m queueMessage
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// advanceBackoff lets the worker reach each back-off wait before advancing the
// FakeClock by exactly that wait, so no retry can run ahead of the clock. It
// assumes a single message is in flight: any waiter on the clock counts.
func advanceBackoff(t *testing.T, env *testutil.Env, waits ...time.Duration) {
	t.Helper()
	for _, d := range waits {
		env.Clock.BlockUntilWaiters(t, 1)
		env.Clock.Advance(d)
	}
}

func TestE2E_Queue_RetriesThenSucceeds(t *testing.T) {
	env := testutil.NewTestServer(t)
	env.Provider.Enqueue(
		testutil.Response{Err: provider.ErrTransient},
		testutil.Response{Err: provider.ErrTransient},
	) // 第 3 次回到 FakeProvider 預設的成功

	id := submit(t, env)
	advanceBackoff(t, env, 1*time.Second, 2*time.Second) // 退避 1s、2s（假時鐘，不睡真實時間）
	env.AwaitStatus(t, id, store.StatusSent)

	m := getQueueMessage(t, env, id)
	if m.Status != "sent" || m.Attempts != 3 {
		t.Fatalf("status=%q attempts=%d，want sent／3", m.Status, m.Attempts)
	}
	if n := len(env.Provider.Sent()); n != 3 {
		t.Fatalf("Provider 被呼叫 %d 次，want 3（成功後不再呼叫）", n)
	}
}

func TestE2E_Queue_ExhaustsRetriesMarksFailed(t *testing.T) {
	env := testutil.NewTestServer(t)
	for i := 0; i < 4; i++ {
		env.Provider.Enqueue(testutil.Response{Err: provider.ErrTransient})
	}

	id := submit(t, env)
	advanceBackoff(t, env, 1*time.Second, 2*time.Second, 4*time.Second)
	env.AwaitStatus(t, id, store.StatusFailed)

	m := getQueueMessage(t, env, id)
	if m.Status != "failed" || m.Attempts != 4 || m.FailureReason == "" {
		t.Fatalf("status=%q attempts=%d failure_reason=%q，want failed／4／非空",
			m.Status, m.Attempts, m.FailureReason)
	}
	if m.FailureReason != queue.ReasonRetriesExhausted {
		t.Fatalf("failure_reason=%q，want %q", m.FailureReason, queue.ReasonRetriesExhausted)
	}
	if n := len(env.Provider.Sent()); n != 4 {
		t.Fatalf("Provider 被呼叫 %d 次，want 4（首次＋重試 3 次）", n)
	}
}
