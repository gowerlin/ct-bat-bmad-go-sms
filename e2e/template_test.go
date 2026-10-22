package e2e

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"example.com/sms-platform/e2e/testutil"
)

type templateBody struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

func createTemplate(t *testing.T, env *testutil.Env, apiKey, payload string) templateBody {
	t.Helper()
	resp, body := env.Do(t, http.MethodPost, "/v1/templates", apiKey, strings.NewReader(payload))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create template: %d %s", resp.StatusCode, body)
	}
	var got templateBody
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestE2E_Template_RendersVariables(t *testing.T) {
	env := testutil.NewTestServer(t)

	tpl := createTemplate(t, env, testutil.KeyA, `{"name":"shipped","body":"訂單 {{order_id}} 已出貨"}`)
	if !strings.HasPrefix(tpl.ID, "tpl_") || len(tpl.ID) != len("tpl_")+32 {
		t.Fatalf("id = %q", tpl.ID)
	}
	if tpl.Name != "shipped" || tpl.Body != "訂單 {{order_id}} 已出貨" {
		t.Fatalf("created = %+v", tpl)
	}
	if ts, err := time.Parse(time.RFC3339, tpl.CreatedAt); err != nil || !ts.Equal(env.Clock.Now()) {
		t.Fatalf("created_at = %q, want FakeClock time %v (err=%v)", tpl.CreatedAt, env.Clock.Now(), err)
	}

	resp, body := env.Get(t, "/v1/templates/"+tpl.ID)
	var got templateBody
	if resp.StatusCode != http.StatusOK || json.Unmarshal(body, &got) != nil || got != tpl {
		t.Fatalf("GET template: %d %s", resp.StatusCode, body)
	}
	resp, body = env.Get(t, "/v1/templates")
	var list struct{ Templates []templateBody }
	if resp.StatusCode != http.StatusOK || json.Unmarshal(body, &list) != nil ||
		len(list.Templates) != 1 || list.Templates[0] != tpl {
		t.Fatalf("list templates: %d %s", resp.StatusCode, body)
	}

	// Send with template_id + vars: rendered before Submit (AD-16). The queue
	// has no sending workers yet, so the rendered body is read back from the
	// store (T0004 Q-1); the provider must not have seen any raw template.
	resp, body = send(t, env, `{"to":"+886900000001","template_id":"`+tpl.ID+`","vars":{"order_id":"A123","extra":"ignored"}}`)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("send with template: %d %s", resp.StatusCode, body)
	}
	var sent struct{ ID, Status string }
	if err := json.Unmarshal(body, &sent); err != nil || sent.Status != "queued" {
		t.Fatalf("send response %s (err=%v)", body, err)
	}
	msg, err := env.Store.Get(testutil.TenantA, sent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Body != "訂單 A123 已出貨" {
		t.Fatalf("stored body = %q, want rendered", msg.Body)
	}
	for _, m := range env.Provider.Sent() {
		if strings.Contains(m.Body, "{{") {
			t.Fatalf("provider received an unrendered body: %q", m.Body)
		}
	}

	// Templates belong to a tenant: tenant B cannot read, use, list or delete A's.
	resp, body = env.Do(t, http.MethodGet, "/v1/templates/"+tpl.ID, testutil.KeyB, nil)
	wantEnvelope(t, resp, body, http.StatusNotFound, "not_found")
	resp, body = env.Do(t, http.MethodPost, "/v1/messages", testutil.KeyB,
		strings.NewReader(`{"to":"+886900000001","template_id":"`+tpl.ID+`","vars":{"order_id":"A123"}}`))
	wantEnvelope(t, resp, body, http.StatusNotFound, "not_found")
	resp, body = env.Do(t, http.MethodGet, "/v1/templates", testutil.KeyB, nil)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"templates":[]`) {
		t.Fatalf("tenant B list: %d %s", resp.StatusCode, body)
	}
	resp, body = env.Do(t, http.MethodDelete, "/v1/templates/"+tpl.ID, testutil.KeyB, nil)
	wantEnvelope(t, resp, body, http.StatusNotFound, "not_found")

	resp, body = env.Do(t, http.MethodDelete, "/v1/templates/"+tpl.ID, testutil.KeyA, nil)
	if resp.StatusCode != http.StatusNoContent || len(body) != 0 {
		t.Fatalf("delete: %d %q", resp.StatusCode, body)
	}
	resp, body = env.Get(t, "/v1/templates/"+tpl.ID)
	wantEnvelope(t, resp, body, http.StatusNotFound, "not_found")
	if env.Store.Len() != 1 {
		t.Fatalf("store has %d messages, want only the rendered one", env.Store.Len())
	}
}

func TestE2E_Template_RejectsInvalidRequests(t *testing.T) {
	env := testutil.NewTestServer(t)

	for name, payload := range map[string]string{
		"unclosed placeholder": `{"name":"n","body":"hi {{name"}`,
		"stray closing":        `{"name":"n","body":"hi name}}"}`,
		"empty placeholder":    `{"name":"n","body":"hi {{}}"}`,
	} {
		resp, body := env.Do(t, http.MethodPost, "/v1/templates", testutil.KeyA, strings.NewReader(payload))
		t.Run(name, func(t *testing.T) { wantEnvelope(t, resp, body, http.StatusBadRequest, "invalid_template") })
	}
	for name, payload := range map[string]string{
		"missing name": `{"body":"hi"}`,
		"missing body": `{"name":"n"}`,
		"invalid JSON": `{"name":`,
	} {
		resp, body := env.Do(t, http.MethodPost, "/v1/templates", testutil.KeyA, strings.NewReader(payload))
		t.Run(name, func(t *testing.T) { wantEnvelope(t, resp, body, http.StatusBadRequest, "invalid_request") })
	}
	resp, body := env.Get(t, "/v1/templates")
	if !strings.Contains(string(body), `"templates":[]`) {
		t.Fatalf("rejected templates were stored: %d %s", resp.StatusCode, body)
	}

	tpl := createTemplate(t, env, testutil.KeyA, `{"name":"n","body":"{{a}}"}`)
	long := strings.Repeat("字", 71)
	for name, payload := range map[string]string{
		"body and template_id":   `{"to":"+886900000001","body":"hi","template_id":"` + tpl.ID + `","vars":{"a":"x"}}`,
		"neither":                `{"to":"+886900000001"}`,
		"number var":             `{"to":"+886900000001","template_id":"` + tpl.ID + `","vars":{"a":1}}`,
		"null var":               `{"to":"+886900000001","template_id":"` + tpl.ID + `","vars":{"a":null}}`,
		"vars not an object":     `{"to":"+886900000001","template_id":"` + tpl.ID + `","vars":"a"}`,
		"rendered over 70":       `{"to":"+886900000001","template_id":"` + tpl.ID + `","vars":{"a":"` + long + `"}}`,
		"rendered blank":         `{"to":"+886900000001","template_id":"` + tpl.ID + `","vars":{"a":"  "}}`,
		"invalid to w/ template": `{"to":"0900000001","template_id":"` + tpl.ID + `","vars":{"a":"x"}}`,
	} {
		resp, body := send(t, env, payload)
		t.Run(name, func(t *testing.T) {
			wantEnvelope(t, resp, body, http.StatusBadRequest, "invalid_request")
			if strings.Contains(name, "var") && !strings.Contains(string(body), "field 'vars'") {
				t.Fatalf("want the vars shape message, got %s", body)
			}
			if strings.Contains(string(body), long) || strings.Contains(string(body), "0900000001") {
				t.Fatalf("error echoes request content: %s", body)
			}
		})
	}
	resp, body = send(t, env, `{"to":"+886900000001","template_id":"tpl_doesnotexist","vars":{"a":"x"}}`)
	wantEnvelope(t, resp, body, http.StatusNotFound, "not_found")
	if env.Store.Len() != 0 {
		t.Fatalf("store has %d messages after only rejected sends", env.Store.Len())
	}
}
