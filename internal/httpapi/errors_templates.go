package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"example.com/sms-platform/internal/template"
)

// templateError maps template sentinel errors to apiError (T0004
// ErrInvalidTemplate -> 400, T0007 ErrMissingVariable -> 422). It is listed in
// featureErrorMappers so writeError does not change for them.
func templateError(err error) (*apiError, bool) {
	var mv *template.MissingVariableError
	switch {
	case errors.Is(err, template.ErrInvalidTemplate):
		return &apiError{
			Status: http.StatusBadRequest, Code: "invalid_template",
			Message: "template placeholders must be balanced {{name}} pairs",
		}, true
	case errors.As(err, &mv) && len(mv.Names) > 0:
		// Only the placeholder names: never the template body or var values.
		return &apiError{
			Status: http.StatusUnprocessableEntity, Code: "missing_template_variable",
			Message: "missing template variables: " + strings.Join(mv.Names, ", "),
		}, true
	case errors.Is(err, template.ErrMissingVariable):
		return &apiError{
			Status: http.StatusUnprocessableEntity, Code: "missing_template_variable",
			Message: "missing template variables",
		}, true
	}
	return nil, false
}
