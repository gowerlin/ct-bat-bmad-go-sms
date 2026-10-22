package httpapi

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const maxBodyCodePoints = 70

var e164 = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

// validateMessage is the single validation function for recipient and
// content; direct bodies and (later) rendered templates both go through it
// (AD-8). Error messages never echo the number or the content.
func validateMessage(to, body string) error {
	if to == "" {
		return invalid("field 'to' is required")
	}
	if !e164.MatchString(to) {
		return invalid("field 'to' must be an E.164 number")
	}
	if strings.TrimSpace(body) == "" {
		return invalid("field 'body' is required")
	}
	if utf8.RuneCountInString(body) > maxBodyCodePoints {
		return invalid("field 'body' exceeds %d characters", maxBodyCodePoints)
	}
	return nil
}
