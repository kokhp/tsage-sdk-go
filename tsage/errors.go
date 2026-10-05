// Package tsage is the official Go SDK for the TranslatorSage API.
package tsage

import (
	"encoding/json"
	"fmt"
)

// APIError is the base error type returned by all SDK methods. Callers can
// use errors.As to extract the typed subkind (*AuthError, *RateLimitError, …)
// and inspect StatusCode / Body / RequestID.
type APIError struct {
	Kind       string
	Message    string
	StatusCode int
	// Body holds the parsed JSON body (map/slice) when available, or the raw
	// response string on parse failure.
	Body      any
	RequestID string
}

func (e *APIError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("tsage: %s (%d) %s", e.Kind, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("tsage: %s %s", e.Kind, e.Message)
}

// Typed error subkinds. All embed *APIError so you can type-assert / errors.As
// to grab the structured context.

type AuthError struct{ *APIError }       // 401

func (e *AuthError) Error() string { return e.APIError.Error() }

type ForbiddenError struct{ *APIError }  // 403

func (e *ForbiddenError) Error() string { return e.APIError.Error() }

type NotFoundError struct{ *APIError }   // 404

func (e *NotFoundError) Error() string { return e.APIError.Error() }

type ValidationError struct{ *APIError } // 422

func (e *ValidationError) Error() string { return e.APIError.Error() }

type RateLimitError struct{ *APIError }  // 429

func (e *RateLimitError) Error() string { return e.APIError.Error() }

type ServerError struct{ *APIError }     // 5xx

func (e *ServerError) Error() string { return e.APIError.Error() }

// errorForStatus maps an HTTP status code into the right typed error.
func errorForStatus(status int, body []byte, requestID string) error {
	var parsed any
	msg := ""
	if len(body) > 0 {
		if err := json.Unmarshal(body, &parsed); err != nil {
			parsed = string(body)
			msg = string(body)
		} else {
			msg = extractMessage(parsed)
		}
	}
	if msg == "" {
		msg = fmt.Sprintf("http %d", status)
	}
	base := &APIError{Kind: kindFor(status), Message: msg, StatusCode: status, Body: parsed, RequestID: requestID}
	switch status {
	case 401:
		return &AuthError{base}
	case 403:
		return &ForbiddenError{base}
	case 404:
		return &NotFoundError{base}
	case 422:
		return &ValidationError{base}
	case 429:
		return &RateLimitError{base}
	}
	if status >= 500 && status < 600 {
		return &ServerError{base}
	}
	return base
}

func kindFor(status int) string {
	switch status {
	case 401:
		return "auth"
	case 403:
		return "forbidden"
	case 404:
		return "not_found"
	case 422:
		return "validation"
	case 429:
		return "rate_limit"
	}
	if status >= 500 && status < 600 {
		return "server"
	}
	return "api"
}

func extractMessage(body any) string {
	m, ok := body.(map[string]any)
	if !ok {
		return ""
	}
	for _, k := range []string{"detail", "error", "message"} {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
			b, _ := json.Marshal(v)
			return string(b)
		}
	}
	return ""
}
