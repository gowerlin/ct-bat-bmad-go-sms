package tenant

import (
	"bytes"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"testing"
)

func newTestRegistry() *Registry {
	r := NewRegistry()
	r.AddTenant(Tenant{ID: "tenant-a", CallbackURL: "https://example.com/a"})
	r.AddTenant(Tenant{ID: "tenant-b"})
	r.AddKey("tenant-a", "test-key-a")
	r.AddKey("tenant-b", "test-key-b")
	return r
}

func TestHashKeyIsSHA256Hex(t *testing.T) {
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" // sha256("abc")
	if got := HashKey("abc"); got != want {
		t.Fatalf("HashKey = %s, want %s", got, want)
	}
}

func TestAuthenticate(t *testing.T) {
	r := newTestRegistry()
	r.AddKey("tenant-a", "test-key-a-disabled")
	if !r.DisableKey("test-key-a-disabled") {
		t.Fatal("DisableKey reported unknown key")
	}

	cases := []struct {
		name, key string
		wantID    string
		wantErr   bool
	}{
		{"tenant a", "test-key-a", "tenant-a", false},
		{"tenant b", "test-key-b", "tenant-b", false},
		{"empty", "", "", true},
		{"unknown", "nope", "", true},
		{"disabled", "test-key-a-disabled", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := r.Authenticate(c.key)
			if c.wantErr {
				if !errors.Is(err, ErrUnauthorized) {
					t.Fatalf("err = %v, want ErrUnauthorized", err)
				}
				return
			}
			if err != nil || got.ID != c.wantID {
				t.Fatalf("got (%+v, %v), want tenant %s", got, err, c.wantID)
			}
		})
	}
}

func TestAuthenticateKeepsCallbackURL(t *testing.T) {
	got, err := newTestRegistry().Authenticate("test-key-a")
	if err != nil || got.CallbackURL != "https://example.com/a" {
		t.Fatalf("got (%+v, %v)", got, err)
	}
}

func TestEmptyRegistryRejectsEverything(t *testing.T) {
	if _, err := NewRegistry().Authenticate("anything"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v", err)
	}
}

func TestAddKeyUnknownTenant(t *testing.T) {
	if NewRegistry().AddKey("ghost", "k") {
		t.Fatal("AddKey accepted unknown tenant")
	}
}

func TestDisableUnknownKey(t *testing.T) {
	if newTestRegistry().DisableKey("nope") {
		t.Fatal("DisableKey accepted unknown key")
	}
}

func TestRateLimitPerMin(t *testing.T) {
	r := newTestRegistry()
	r.AddTenant(Tenant{ID: "tenant-c", RateLimitPerMin: 3})
	cases := map[string]int{"tenant-c": 3, "tenant-a": 0, "ghost": 0}
	for id, want := range cases {
		if got := r.RateLimitPerMin(id); got != want {
			t.Errorf("RateLimitPerMin(%q) = %d, want %d", id, got, want)
		}
	}
	// AddTenant replaces, so a new limit takes effect immediately.
	r.AddTenant(Tenant{ID: "tenant-c", RateLimitPerMin: 10})
	if got := r.RateLimitPerMin("tenant-c"); got != 10 {
		t.Fatalf("after replace = %d, want 10", got)
	}
}

func TestCallbackAndSetCallback(t *testing.T) {
	r := newTestRegistry()
	if url, secret := r.Callback("tenant-b"); url != "" || secret != "" {
		t.Fatalf("tenant-b without callback = %q, %q", url, secret)
	}
	if url, secret := r.Callback("nobody"); url != "" || secret != "" {
		t.Fatalf("unknown tenant = %q, %q", url, secret)
	}
	if r.SetCallback("nobody", "https://example.com/x", "test-secret") {
		t.Fatal("SetCallback on unknown tenant reported true")
	}
	if !r.SetCallback("tenant-b", "https://example.com/b", "test-secret") {
		t.Fatal("SetCallback on tenant-b reported false")
	}
	if url, secret := r.Callback("tenant-b"); url != "https://example.com/b" || secret != "test-secret" {
		t.Fatalf("tenant-b callback = %q, %q", url, secret)
	}
	// The key stays valid: SetCallback only touches the callback fields.
	if got, err := r.Authenticate("test-key-b"); err != nil || got.CallbackSecret != "test-secret" {
		t.Fatalf("authenticate after SetCallback: %v", err)
	}
	urls := r.CallbackURLs()
	sort.Strings(urls)
	if strings.Join(urls, ",") != "https://example.com/a,https://example.com/b" {
		t.Fatalf("CallbackURLs = %v", urls)
	}
}

func TestTenantLogValueOmitsSecret(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	log.Info("tenant", "tenant", Tenant{ID: "tenant-a", CallbackURL: "https://example.com/a", CallbackSecret: "test-secret"})
	if out := buf.String(); strings.Contains(out, "test-secret") || !strings.Contains(out, "tenant-a") {
		t.Fatalf("log line = %s", out)
	}
}
