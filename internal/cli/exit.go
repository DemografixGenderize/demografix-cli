package cli

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/DemografixGenderize/demografix-cli/internal/api"
	"github.com/DemografixGenderize/demografix-cli/internal/config"
	"github.com/DemografixGenderize/demografix-cli/internal/output"
)

// Exit codes. These are part of the CLI contract.
const (
	ExitOK         = 0
	ExitGeneric    = 1
	ExitUsage      = 2
	ExitAuth       = 3
	ExitSubscript  = 4
	ExitValidation = 5
	ExitRateLimit  = 6
	ExitTransport  = 7
)

// usageError marks bad flags/args so they map to ExitUsage.
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

// UsageErrorf builds a usage error.
func UsageErrorf(format string, a ...any) error {
	return &usageError{msg: fmt.Sprintf(format, a...)}
}

// Code maps an error to a process exit code.
func Code(err error) int {
	if err == nil {
		return ExitOK
	}
	var ue *usageError
	if errors.As(err, &ue) {
		return ExitUsage
	}
	var ae *api.AuthError
	if errors.As(err, &ae) {
		return ExitAuth
	}
	if errors.Is(err, config.ErrNoAPIKey) {
		return ExitAuth
	}
	var se *api.SubscriptionError
	if errors.As(err, &se) {
		return ExitSubscript
	}
	var ve *api.ValidationError
	if errors.As(err, &ve) {
		return ExitValidation
	}
	var rle *api.RateLimitError
	if errors.As(err, &rle) {
		return ExitRateLimit
	}
	var te *api.TransportError
	if errors.As(err, &te) {
		return ExitTransport
	}
	return ExitGeneric
}

// Render writes a friendly one-line error to w.
func Render(w io.Writer, err error) {
	if err == nil {
		return
	}

	var rle *api.RateLimitError
	if errors.As(err, &rle) && rle.Quota != nil {
		resetAt := time.Now().Add(time.Duration(rle.Quota.Reset) * time.Second).UTC()
		fmt.Fprintf(w, "error: rate limit reached (%d/%d left); resets in %s (%s)\n",
			rle.Quota.Remaining, rle.Quota.Limit, output.HumanDuration(rle.Quota.Reset),
			resetAt.Format("2006-01-02 15:04:05 UTC"))
		return
	}

	var ae *api.AuthError
	if errors.As(err, &ae) {
		fmt.Fprintln(w, "error: invalid API key; run `demografix login` to update it")
		return
	}

	var se *api.SubscriptionError
	if errors.As(err, &se) {
		fmt.Fprintln(w, "error: subscription inactive or expired")
		return
	}

	var te *api.TransportError
	if errors.As(err, &te) {
		fmt.Fprintf(w, "error: request failed: %s\n", te.Message)
		return
	}

	fmt.Fprintf(w, "error: %s\n", message(err))
}

// message returns the cleanest user-facing string for an error.
func message(err error) string {
	var e api.Error
	if errors.As(err, &e) {
		return e.Base().Message
	}
	return err.Error()
}
