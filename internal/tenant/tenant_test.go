package tenant

import (
	"errors"
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
