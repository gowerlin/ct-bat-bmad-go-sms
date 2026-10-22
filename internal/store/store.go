package store

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Status is the platform-side message status (AD-5).
type Status string

const (
	StatusQueued      Status = "queued"
	StatusSending     Status = "sending"
	StatusSent        Status = "sent"
	StatusDelivered   Status = "delivered"
	StatusUndelivered Status = "undelivered"
	StatusFailed      Status = "failed"
)

// Valid reports whether s is one of the statuses above.
func (s Status) Valid() bool {
	switch s {
	case StatusQueued, StatusSending, StatusSent, StatusDelivered, StatusUndelivered, StatusFailed:
		return true
	}
	return false
}

// Retention is how long a terminal message stays readable (AD-14).
const Retention = 24 * time.Hour

// Page sizes for ListMessages: Limit <= 0 means DefaultListLimit, larger
// values are capped at MaxListLimit.
const (
	DefaultListLimit = 50
	MaxListLimit     = 100
)

var (
	// ErrNotFound is returned for unknown ids and for cross-tenant access.
	ErrNotFound = errors.New("store: not found")
	// ErrIllegalTransition is returned for transitions outside the table, a
	// failed compare-and-set, or a missing failure reason.
	ErrIllegalTransition = errors.New("store: illegal transition")
	// ErrEmptyTenant is returned when a TenantScoped method gets tenantID "".
	ErrEmptyTenant = errors.New("store: empty tenant id")
	// ErrInvalidCursor is returned for a cursor ListMessages did not produce.
	ErrInvalidCursor = errors.New("store: invalid cursor")
)

// transitions is the single legal-transition table (AD-5).
var transitions = map[Status][]Status{
	StatusQueued:  {StatusSending},
	StatusSending: {StatusSent, StatusFailed},
	StatusSent:    {StatusDelivered, StatusUndelivered},
}

// IsTerminal reports whether s can no longer change.
func IsTerminal(s Status) bool {
	return s == StatusDelivered || s == StatusUndelivered || s == StatusFailed
}

func legal(from, to Status) bool {
	for _, t := range transitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

// Message is the stored message entity.
type Message struct {
	ID            string
	TenantID      string
	To            string
	Body          string
	Status        Status
	Attempts      int
	Provider      string
	ProviderRef   string
	FailureReason string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	TerminalAt    time.Time // zero until the message reaches a terminal status
}

// Template is the stored template record. The template package owns its
// rules (placeholder syntax, id prefix); the store only keeps it per tenant.
type Template struct {
	ID        string
	TenantID  string
	Name      string
	Body      string
	CreatedAt time.Time
}

// MessageFilter selects one page of a tenant's messages. An empty Status
// matches every status; Cursor is a previous page's NextCursor ("" = first
// page).
type MessageFilter struct {
	Status Status
	Cursor string
	Limit  int
}

// MessagePage is one page of ListMessages. NextCursor is "" on the last page.
type MessagePage struct {
	Messages   []Message
	NextCursor string
}

// Patch carries the fields written together with a transition. Zero values
// leave the stored field unchanged.
type Patch struct {
	Attempts      int
	ProviderRef   string
	FailureReason string
}

// TenantScoped is the repository view held by httpapi: every method takes the
// tenant id first and an empty id is an error (AD-4).
type TenantScoped interface {
	Get(tenantID, id string) (Message, error)
	// ListMessages returns one page of the tenant's messages matching f,
	// ordered by (CreatedAt, ID) newest first (AD-14).
	ListMessages(tenantID string, f MessageFilter) (MessagePage, error)
	// CreateTemplate stores t under tenantID and stamps CreatedAt; t.ID is required.
	CreateTemplate(tenantID string, t Template) (Template, error)
	GetTemplate(tenantID, id string) (Template, error)
	// ListTemplates returns the tenant's templates newest first (ties: higher id first).
	ListTemplates(tenantID string) ([]Template, error)
	DeleteTemplate(tenantID, id string) error
}

// System is the repository view held by queue and webhook only (AD-4).
type System interface {
	// Create stores m as a new queued message; m.ID and m.TenantID are required.
	Create(m Message) (Message, error)
	GetByID(id string) (Message, error)
	// Transition is a compare-and-set from -> to applying p.
	Transition(id string, from, to Status, p Patch) (Message, error)
	// PurgeExpired removes terminal messages older than Retention at now.
	PurgeExpired(now time.Time) int
}

// Memory is the in-memory implementation of TenantScoped and System.
type Memory struct {
	now  func() time.Time
	mu   sync.Mutex
	msgs map[string]Message
	tpls map[string]Template
}

var (
	_ TenantScoped = (*Memory)(nil)
	_ System       = (*Memory)(nil)
)

// New returns an empty store stamping times from now.
func New(now func() time.Time) *Memory {
	if now == nil {
		panic("store: New requires a Now function")
	}
	return &Memory{now: now, msgs: make(map[string]Message), tpls: make(map[string]Template)}
}

// NewID returns prefix followed by 128 random bits in hex (AD-10).
func NewID(prefix string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b[:]), nil
}

// Create implements System.
func (s *Memory) Create(m Message) (Message, error) {
	if m.ID == "" || m.TenantID == "" {
		return Message{}, errors.New("store: message needs id and tenant id")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, dup := s.msgs[m.ID]; dup {
		return Message{}, errors.New("store: duplicate id")
	}
	t := s.now().UTC()
	m.Status = StatusQueued
	m.CreatedAt, m.UpdatedAt, m.TerminalAt = t, t, time.Time{}
	s.msgs[m.ID] = m
	return m, nil
}

// Get implements TenantScoped; other tenants' messages are ErrNotFound.
func (s *Memory) Get(tenantID, id string) (Message, error) {
	if tenantID == "" {
		return Message{}, ErrEmptyTenant
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.msgs[id]
	if !ok || m.TenantID != tenantID {
		return Message{}, ErrNotFound
	}
	return m, nil
}

// ListMessages implements TenantScoped; only tenantID's messages are visible.
func (s *Memory) ListMessages(tenantID string, f MessageFilter) (MessagePage, error) {
	if tenantID == "" {
		return MessagePage{}, ErrEmptyTenant
	}
	var after *cursor
	if f.Cursor != "" {
		c, err := decodeCursor(f.Cursor)
		if err != nil {
			return MessagePage{}, err
		}
		after = &c
	}
	limit := f.Limit
	if limit <= 0 {
		limit = DefaultListLimit
	}
	if limit > MaxListLimit {
		limit = MaxListLimit
	}

	s.mu.Lock()
	out := []Message{}
	for _, m := range s.msgs {
		if m.TenantID != tenantID || (f.Status != "" && m.Status != f.Status) {
			continue
		}
		if after != nil && !after.before(m) {
			continue
		}
		out = append(out, m)
	}
	s.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return newer(out[i], out[j]) })

	page := MessagePage{Messages: out}
	if len(out) > limit {
		page.Messages = out[:limit]
		page.NextCursor = encodeCursor(out[limit-1])
	}
	return page, nil
}

// newer orders messages by (CreatedAt, ID) descending.
func newer(a, b Message) bool {
	if !a.CreatedAt.Equal(b.CreatedAt) {
		return a.CreatedAt.After(b.CreatedAt)
	}
	return a.ID > b.ID
}

// cursor is the (CreatedAt, ID) of the last message of a page. On the wire it
// is base64url("<unix nanos>:<id>"); callers treat it as opaque (AD-14).
type cursor struct {
	createdAt time.Time
	id        string
}

// before reports whether m sorts after the cursor, i.e. belongs to a later page.
func (c cursor) before(m Message) bool {
	return newer(Message{CreatedAt: c.createdAt, ID: c.id}, m)
}

func encodeCursor(m Message) string {
	raw := strconv.FormatInt(m.CreatedAt.UnixNano(), 10) + ":" + m.ID
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return cursor{}, ErrInvalidCursor
	}
	nanos, id, ok := strings.Cut(string(raw), ":")
	if !ok || id == "" {
		return cursor{}, ErrInvalidCursor
	}
	n, err := strconv.ParseInt(nanos, 10, 64)
	if err != nil {
		return cursor{}, ErrInvalidCursor
	}
	return cursor{createdAt: time.Unix(0, n).UTC(), id: id}, nil
}

// CreateTemplate implements TenantScoped.
func (s *Memory) CreateTemplate(tenantID string, t Template) (Template, error) {
	if tenantID == "" {
		return Template{}, ErrEmptyTenant
	}
	if t.ID == "" {
		return Template{}, errors.New("store: template needs an id")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, dup := s.tpls[t.ID]; dup {
		return Template{}, errors.New("store: duplicate id")
	}
	t.TenantID = tenantID
	t.CreatedAt = s.now().UTC()
	s.tpls[t.ID] = t
	return t, nil
}

// GetTemplate implements TenantScoped; other tenants' templates are ErrNotFound.
func (s *Memory) GetTemplate(tenantID, id string) (Template, error) {
	if tenantID == "" {
		return Template{}, ErrEmptyTenant
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tpls[id]
	if !ok || t.TenantID != tenantID {
		return Template{}, ErrNotFound
	}
	return t, nil
}

// ListTemplates implements TenantScoped.
func (s *Memory) ListTemplates(tenantID string) ([]Template, error) {
	if tenantID == "" {
		return nil, ErrEmptyTenant
	}
	s.mu.Lock()
	out := []Template{}
	for _, t := range s.tpls {
		if t.TenantID == tenantID {
			out = append(out, t)
		}
	}
	s.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].ID > out[j].ID
	})
	return out, nil
}

// DeleteTemplate implements TenantScoped; other tenants' templates are ErrNotFound.
func (s *Memory) DeleteTemplate(tenantID, id string) error {
	if tenantID == "" {
		return ErrEmptyTenant
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tpls[id]
	if !ok || t.TenantID != tenantID {
		return ErrNotFound
	}
	delete(s.tpls, id)
	return nil
}

// GetByID implements System.
func (s *Memory) GetByID(id string) (Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.msgs[id]
	if !ok {
		return Message{}, ErrNotFound
	}
	return m, nil
}

// Transition implements System.
func (s *Memory) Transition(id string, from, to Status, p Patch) (Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.msgs[id]
	if !ok {
		return Message{}, ErrNotFound
	}
	if m.Status != from || !legal(from, to) {
		return Message{}, ErrIllegalTransition
	}
	if (to == StatusFailed || to == StatusUndelivered) && p.FailureReason == "" {
		return Message{}, ErrIllegalTransition
	}
	if p.Attempts != 0 {
		m.Attempts = p.Attempts
	}
	if p.ProviderRef != "" {
		m.ProviderRef = p.ProviderRef
	}
	if p.FailureReason != "" {
		m.FailureReason = p.FailureReason
	}
	t := s.now().UTC()
	m.Status, m.UpdatedAt = to, t
	if IsTerminal(to) {
		m.TerminalAt = t
	}
	s.msgs[id] = m
	return m, nil
}

// PurgeExpired implements System and returns how many messages were removed.
func (s *Memory) PurgeExpired(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for id, m := range s.msgs {
		if IsTerminal(m.Status) && now.Sub(m.TerminalAt) > Retention {
			delete(s.msgs, id)
			n++
		}
	}
	return n
}

// Len returns the number of stored messages (for tests and diagnostics).
func (s *Memory) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.msgs)
}
