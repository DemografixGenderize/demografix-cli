package api

import "fmt"

// DemografixError is the base error for every API failure. Status is the HTTP
// status (0 for client-side or transport failures); Quota is the rate-limit
// view parsed from the response headers, when present.
type DemografixError struct {
	Status  int
	Message string
	Quota   *Quota
}

func (e *DemografixError) Error() string {
	if e.Status == 0 {
		return e.Message
	}
	return fmt.Sprintf("demografix: %d: %s", e.Status, e.Message)
}

// Base returns the embedded base error. It lets callers reach the status,
// message, and quota of any typed error via errors.As(err, &apiErr) where
// apiErr is an Error.
func (e *DemografixError) Base() *DemografixError { return e }

// Error is implemented by every Demografix SDK error (the base and each typed
// variant), exposing the underlying DemografixError.
type Error interface {
	error
	Base() *DemografixError
}

// AuthError is a 401 (invalid or missing API key).
type AuthError struct{ DemografixError }

// SubscriptionError is a 402 (subscription inactive or expired).
type SubscriptionError struct{ DemografixError }

// ValidationError is a 422 from the server, or a client-side validation failure
// (Status 0) raised before any HTTP call.
type ValidationError struct{ DemografixError }

// RateLimitError is a 429. Quota is always populated and Quota.Reset carries the
// seconds until the window resets.
type RateLimitError struct{ DemografixError }

// TransportError wraps a network, timeout, or non-JSON-body failure.
type TransportError struct {
	DemografixError
	Err error
}

func (e *TransportError) Unwrap() error { return e.Err }

func newAPIError(status int, message string, quota *Quota) error {
	base := DemografixError{Status: status, Message: message, Quota: quota}
	switch status {
	case 401:
		return &AuthError{base}
	case 402:
		return &SubscriptionError{base}
	case 422:
		return &ValidationError{base}
	case 429:
		return &RateLimitError{base}
	default:
		return &base
	}
}

func newValidationError(message string) error {
	return &ValidationError{DemografixError{Message: message}}
}

func wrapTransportError(message string, err error) error {
	return &TransportError{DemografixError{Message: message}, err}
}
