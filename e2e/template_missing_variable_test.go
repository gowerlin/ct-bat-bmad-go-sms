package e2e

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"example.com/sms-platform/e2e/testutil"
)

func TestE2E_Template_MissingVariableReturns422(t *testing.T) {
	env := testutil.NewTestServer(t)

	tpl := createTemplate(t, env, testutil.KeyA, `{"name":"shipped","body":"{{name}} 您好，訂單 {{order_id}} 已出貨"}`)

	// Only order_id is given: 422 naming the missing variable, and the message
	// lists names only — no template text and no other variable's value.
	resp, body := send(t, env, `{"to":"+886900000001","template_id":"`+tpl.ID+`","vars":{"order_id":"A123"}}`)
	wantEnvelope(t, resp, body, http.StatusUnprocessableEntity, "missing_template_variable")
	var e envelope
	if err := json.Unmarshal(body, &e); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(e.Error.Message, ": name") {
		t.Fatalf("message = %q, want it to list the missing variable 'name'", e.Error.Message)
	}
	for _, leak := range []string{"A123", "order_id", "您好", "已出貨", "{{"} {
		if strings.Contains(string(body), leak) {
			t.Fatalf("error echoes %q: %s", leak, body)
		}
	}

	// No vars at all: every missing name, in order of first appearance.
	resp, body = send(t, env, `{"to":"+886900000001","template_id":"`+tpl.ID+`"}`)
	wantEnvelope(t, resp, body, http.StatusUnprocessableEntity, "missing_template_variable")
	if err := json.Unmarshal(body, &e); err != nil || !strings.HasSuffix(e.Error.Message, ": name, order_id") {
		t.Fatalf("message = %q, want both names listed (err=%v)", e.Error.Message, err)
	}

	// Render fails before Submit (AD-16): nothing stored, nothing sent.
	if n := env.Store.Len(); n != 0 {
		t.Fatalf("store has %d messages after missing-variable sends, want 0", n)
	}
	if sent := env.Provider.Sent(); len(sent) != 0 {
		t.Fatalf("provider received %d messages, want 0", len(sent))
	}
}
