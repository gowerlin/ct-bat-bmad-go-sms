// Package features runs the Gherkin scenarios in this directory with godog
// (CH5 5-07, optional). Steps call the same E2E harness as e2e/, so a scenario
// and its TestE2E_* case exercise the same handler.
package features

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/cucumber/godog"

	"example.com/sms-platform/e2e/testutil"
)

// world is the state of one scenario.
type world struct {
	t      testing.TB // the TestFeatures t: testutil needs a testing.TB, godog.T(ctx) is not one
	env    *testutil.Env
	apiKey string
	status int
	body   []byte
}

func (w *world) tenantA() error {
	w.env = testutil.NewTestServer(w.t, testutil.WithoutWorker()) // same as TestE2E_SendMessage_Returns202AndQueryable since T0008
	w.apiKey = testutil.KeyA
	return nil
}

func (w *world) post(payload string) error {
	resp, body := w.env.Do(w.t, http.MethodPost, "/v1/messages", w.apiKey, strings.NewReader(payload))
	w.status, w.body = resp.StatusCode, body
	return nil
}

func (w *world) statusIs(want int) error {
	if w.status != want {
		return fmt.Errorf("status = %d, want %d; body = %s", w.status, want, w.body)
	}
	return nil
}

func (w *world) messageStatusIs(want string) error {
	var sent struct{ ID string }
	if err := json.Unmarshal(w.body, &sent); err != nil || sent.ID == "" {
		return fmt.Errorf("response %s has no id (err=%v)", w.body, err)
	}
	resp, body := w.env.Do(w.t, http.MethodGet, "/v1/messages/"+sent.ID, w.apiKey, nil)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s = %d %s", sent.ID, resp.StatusCode, body)
	}
	var got struct{ Status string }
	if err := json.Unmarshal(body, &got); err != nil {
		return err
	}
	if got.Status != want {
		return fmt.Errorf("message status = %q, want %q", got.Status, want)
	}
	return nil
}

func initializeScenario(t testing.TB) func(*godog.ScenarioContext) {
	return func(sc *godog.ScenarioContext) {
		w := &world{t: t}
		sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
			*w = world{t: t} // a fresh world per scenario
			return ctx, nil
		})
		sc.Step(`^租戶 A 帶著有效的 API key$`, w.tenantA)
		sc.Step(`^對 /v1/messages 送出 (.+)$`, w.post)
		sc.Step(`^回應狀態碼是 (\d+)$`, w.statusIs)
		sc.Step(`^訊息狀態是 (\w+)$`, w.messageStatusIs)
	}
}

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		Name:                "sms-platform",
		ScenarioInitializer: initializeScenario(t),
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"."},
			Strict:   true, // undefined or pending steps fail the run
			TestingT: t,    // each scenario runs as a subtest
		},
	}
	if suite.Run() != 0 {
		t.Fatal("godog: feature scenarios failed")
	}
}
