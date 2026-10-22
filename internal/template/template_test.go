package template

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"example.com/sms-platform/internal/store"
)

func TestRender(t *testing.T) {
	cases := []struct {
		name, body string
		vars       map[string]string
		want       string
	}{
		{"spec example", "訂單 {{order_id}} 已出貨", map[string]string{"order_id": "A123"}, "訂單 A123 已出貨"},
		{"no placeholders", "hello", nil, "hello"},
		{"repeated", "{{a}}-{{a}}", map[string]string{"a": "x"}, "x-x"},
		{"spaces inside braces", "hi {{ name }}", map[string]string{"name": "Ann"}, "hi Ann"},
		{"unicode name", "{{姓名}}您好", map[string]string{"姓名": "王"}, "王您好"},
		{"extra vars ignored", "{{a}}", map[string]string{"a": "1", "b": "2"}, "1"},
		{"single braces are literal", "{a} }", nil, "{a} }"},
		{"value is not re-parsed", "{{a}}", map[string]string{"a": "{{b}}"}, "{{b}}"},
	}
	for _, c := range cases {
		got, err := Render(c.body, c.vars)
		if err != nil || got != c.want {
			t.Errorf("%s: Render = %q, %v; want %q", c.name, got, err, c.want)
		}
	}
}

func TestRenderMissingVariablesListsNames(t *testing.T) {
	_, err := Render("{{b}} {{a}} {{b}} {{c}}", map[string]string{"c": "3"})
	if !errors.Is(err, ErrMissingVariable) {
		t.Fatalf("err = %v, want ErrMissingVariable", err)
	}
	var mv *MissingVariableError
	if !errors.As(err, &mv) {
		t.Fatalf("err %T is not *MissingVariableError", err)
	}
	if want := []string{"b", "a"}; !reflect.DeepEqual(mv.Names, want) {
		t.Fatalf("names = %v, want %v", mv.Names, want)
	}
	if !strings.Contains(err.Error(), "b, a") {
		t.Fatalf("Error() = %q", err.Error())
	}
}

func TestInvalidTemplates(t *testing.T) {
	for _, body := range []string{
		"hi {{name",
		"hi name}}",
		"}}{{a}}",
		"{{a}} }}",
		"{{}}",
		"{{ }}",
		"{{a b}}",
		"{{1a}}",
		"{{a{{b}}",
		"{{a-b}}",
	} {
		if err := Validate(body); !errors.Is(err, ErrInvalidTemplate) {
			t.Errorf("Validate(%q) = %v, want ErrInvalidTemplate", body, err)
		}
		if _, err := Render(body, map[string]string{"a": "1", "name": "x"}); !errors.Is(err, ErrInvalidTemplate) {
			t.Errorf("Render(%q) = %v, want ErrInvalidTemplate", body, err)
		}
	}
}

func newService() (*Service, *store.Memory) {
	st := store.New(func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) })
	return NewService(st), st
}

func TestServiceCRUDIsTenantScoped(t *testing.T) {
	s, _ := newService()
	tp, err := s.Create("a", "shipped", "訂單 {{order_id}} 已出貨")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tp.ID, "tpl_") || len(tp.ID) != len("tpl_")+32 {
		t.Fatalf("id = %q", tp.ID)
	}
	if tp.TenantID != "a" || tp.Name != "shipped" || tp.CreatedAt.IsZero() {
		t.Fatalf("created = %+v", tp)
	}
	if got, err := s.Get("a", tp.ID); err != nil || got != tp {
		t.Fatalf("get = %+v, %v", got, err)
	}
	if _, err := s.Get("b", tp.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("cross-tenant get: %v", err)
	}
	if err := s.Delete("b", tp.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("cross-tenant delete: %v", err)
	}
	if list, err := s.List("b"); err != nil || len(list) != 0 {
		t.Fatalf("tenant b list = %v, %v", list, err)
	}
	if list, err := s.List("a"); err != nil || len(list) != 1 || list[0] != tp {
		t.Fatalf("tenant a list = %v, %v", list, err)
	}
	if err := s.Delete("a", tp.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("a", tp.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}

func TestServiceCreateRejectsInvalidAndEmptyTenant(t *testing.T) {
	s, _ := newService()
	if _, err := s.Create("a", "n", "hi {{name"); !errors.Is(err, ErrInvalidTemplate) {
		t.Fatalf("invalid body: %v", err)
	}
	if _, err := s.Create("", "n", "hi"); !errors.Is(err, store.ErrEmptyTenant) {
		t.Fatalf("empty tenant: %v", err)
	}
	if list, _ := s.List("a"); len(list) != 0 {
		t.Fatalf("rejected templates were stored: %v", list)
	}
}
