package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateMessage(t *testing.T) {
	cases := []struct {
		name, to, body string
		ok             bool
	}{
		{"ok", "+886900000001", "hi", true},
		{"70 code points", "+886900000001", strings.Repeat("字", 70), true},
		{"71 code points", "+886900000001", strings.Repeat("字", 71), false},
		{"missing to", "", "hi", false},
		{"missing body", "+886900000001", "", false},
		{"no plus", "886900000001", "hi", false},
		{"leading zero", "+0886900000001", "hi", false},
		{"too long", "+8869000000012345", "hi", false},
	}
	for _, c := range cases {
		if err := validateMessage(c.to, c.body); (err == nil) != c.ok {
			t.Errorf("%s: err=%v, want ok=%v", c.name, err, c.ok)
		}
	}
}

func TestWriteErrorUnknownIs500FixedMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, errors.New("secret internals: +886900000001"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	var got struct {
		Error struct{ Code, Message string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Error.Code != "internal_error" || got.Error.Message != internalMessage {
		t.Fatalf("envelope = %+v", got)
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("leaked internals: %s", rec.Body.String())
	}
}

func TestNewHandlerRequiresDeps(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for missing Queue/Store")
		}
	}()
	NewHandler(Deps{})
}
