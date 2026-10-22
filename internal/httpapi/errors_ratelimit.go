package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"example.com/sms-platform/internal/ratelimit"
)

// rateLimitError maps a rejected request to 429 rate_limited with Retry-After
// in whole seconds (AD-12). It is listed in featureErrorMappers.
func rateLimitError(err error) (*apiError, bool) {
	var le *ratelimit.LimitedError
	if !errors.As(err, &le) {
		return nil, false
	}
	return &apiError{
		Status:  http.StatusTooManyRequests,
		Code:    "rate_limited",
		Message: "rate limit exceeded, retry later",
		Header:  http.Header{"Retry-After": {strconv.Itoa(le.RetryAfter)}},
	}, true
}
