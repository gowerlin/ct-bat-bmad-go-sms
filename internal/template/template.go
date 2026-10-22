package template

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"example.com/sms-platform/internal/store"
)

var (
	// ErrInvalidTemplate is a body whose {{…}} placeholders cannot be parsed:
	// unbalanced braces or an empty or malformed variable name.
	ErrInvalidTemplate = errors.New("template: invalid template")
	// ErrMissingVariable is returned by Render when vars lacks a placeholder;
	// errors.As the error to *MissingVariableError for the names.
	ErrMissingVariable = errors.New("template: missing variable")
)

// MissingVariableError lists the placeholder names Render had no value for,
// in order of first appearance. It matches errors.Is(err, ErrMissingVariable).
type MissingVariableError struct{ Names []string }

func (e *MissingVariableError) Error() string {
	return ErrMissingVariable.Error() + ": " + strings.Join(e.Names, ", ")
}

func (e *MissingVariableError) Unwrap() error { return ErrMissingVariable }

// Template is a tenant's message template (§ 5.3).
type Template struct {
	ID        string
	TenantID  string
	Name      string
	Body      string
	CreatedAt time.Time
}

const (
	openDelim  = "{{"
	closeDelim = "}}"
)

// varName is a placeholder name: a letter or underscore, then letters,
// digits or underscores. Spaces just inside the braces are ignored.
var varName = regexp.MustCompile(`^[\p{L}_][\p{L}\p{N}_]*$`)

// segment is literal text, or a placeholder when isVar is set.
type segment struct {
	text  string
	isVar bool
}

func parse(body string) ([]segment, error) {
	var segs []segment
	for body != "" {
		i, j := strings.Index(body, openDelim), strings.Index(body, closeDelim)
		if i < 0 {
			if j >= 0 {
				return nil, ErrInvalidTemplate // "}}" without "{{"
			}
			return append(segs, segment{text: body}), nil
		}
		if j >= 0 && j < i {
			return nil, ErrInvalidTemplate
		}
		rest := body[i+len(openDelim):]
		end := strings.Index(rest, closeDelim)
		if end < 0 {
			return nil, ErrInvalidTemplate // "{{" without "}}"
		}
		name := strings.TrimSpace(rest[:end])
		if !varName.MatchString(name) {
			return nil, ErrInvalidTemplate
		}
		if i > 0 {
			segs = append(segs, segment{text: body[:i]})
		}
		segs = append(segs, segment{text: name, isVar: true})
		body = rest[end+len(closeDelim):]
	}
	return segs, nil
}

// Validate reports ErrInvalidTemplate when body cannot be parsed.
func Validate(body string) error {
	_, err := parse(body)
	return err
}

// Render replaces every {{name}} in body with vars[name]. Extra vars are
// ignored; missing ones yield a *MissingVariableError listing all of them.
func Render(body string, vars map[string]string) (string, error) {
	segs, err := parse(body)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	var missing []string
	seen := map[string]bool{}
	for _, s := range segs {
		if !s.isVar {
			b.WriteString(s.text)
			continue
		}
		v, ok := vars[s.text]
		if !ok {
			if !seen[s.text] {
				seen[s.text] = true
				missing = append(missing, s.text)
			}
			continue
		}
		b.WriteString(v)
	}
	if len(missing) > 0 {
		return "", &MissingVariableError{Names: missing}
	}
	return b.String(), nil
}

// Service is template CRUD per tenant over the tenant-scoped store (AD-4).
// Unknown and other tenants' templates are store.ErrNotFound.
type Service struct{ store store.TenantScoped }

// NewService returns a Service persisting to st.
func NewService(st store.TenantScoped) *Service {
	if st == nil {
		panic("template: NewService requires a store")
	}
	return &Service{store: st}
}

// Create validates body and stores a new template with a tpl_ id (AD-10).
func (s *Service) Create(tenantID, name, body string) (Template, error) {
	if err := Validate(body); err != nil {
		return Template{}, err
	}
	id, err := store.NewID("tpl_")
	if err != nil {
		return Template{}, err
	}
	t, err := s.store.CreateTemplate(tenantID, store.Template{ID: id, Name: name, Body: body})
	if err != nil {
		return Template{}, err
	}
	return fromStore(t), nil
}

// Get returns one of the tenant's templates.
func (s *Service) Get(tenantID, id string) (Template, error) {
	t, err := s.store.GetTemplate(tenantID, id)
	if err != nil {
		return Template{}, err
	}
	return fromStore(t), nil
}

// List returns the tenant's templates, newest first.
func (s *Service) List(tenantID string) ([]Template, error) {
	ts, err := s.store.ListTemplates(tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]Template, len(ts))
	for i, t := range ts {
		out[i] = fromStore(t)
	}
	return out, nil
}

// Delete removes one of the tenant's templates.
func (s *Service) Delete(tenantID, id string) error {
	return s.store.DeleteTemplate(tenantID, id)
}

func fromStore(t store.Template) Template {
	return Template{ID: t.ID, TenantID: t.TenantID, Name: t.Name, Body: t.Body, CreatedAt: t.CreatedAt}
}
