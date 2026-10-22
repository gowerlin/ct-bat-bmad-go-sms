package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"example.com/sms-platform/internal/store"
)

// messageListResponse is the MessageList schema; NextCursor is omitted on the
// last page.
type messageListResponse struct {
	Messages   []messageResponse `json:"messages"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

func (s *server) registerMessagesList(mux *http.ServeMux) {
	mux.Handle("GET /v1/messages", s.v1(s.handler(s.listMessages)))
}

// listMessages serves GET /v1/messages?status=&limit=&cursor=. Error messages
// never echo the query values.
func (s *server) listMessages(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	f := store.MessageFilter{Cursor: q.Get("cursor")}
	if q.Has("status") {
		f.Status = store.Status(q.Get("status"))
		if !f.Status.Valid() {
			return invalid("query parameter 'status' must be one of queued, sending, sent, delivered, undelivered, failed")
		}
	}
	if q.Has("limit") {
		n, err := strconv.Atoi(q.Get("limit"))
		if err != nil || n < 1 || n > store.MaxListLimit {
			return invalid("query parameter 'limit' must be an integer from 1 to %d", store.MaxListLimit)
		}
		f.Limit = n
	}

	page, err := s.deps.Store.ListMessages(tenantID(r.Context()), f)
	if errors.Is(err, store.ErrInvalidCursor) {
		return invalid("query parameter 'cursor' is invalid")
	}
	if err != nil {
		return err
	}
	resp := messageListResponse{Messages: make([]messageResponse, 0, len(page.Messages)), NextCursor: page.NextCursor}
	for _, m := range page.Messages {
		resp.Messages = append(resp.Messages, toMessageResponse(m))
	}
	return writeJSON(w, http.StatusOK, resp)
}
