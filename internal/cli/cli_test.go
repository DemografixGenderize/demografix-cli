package cli

import (
	"errors"
	"testing"

	"github.com/DemografixGenderize/demografix-cli/internal/api"
	"github.com/DemografixGenderize/demografix-cli/internal/config"
)

func TestCodeMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, ExitOK},
		{"usage", UsageErrorf("bad"), ExitUsage},
		{"auth", &api.AuthError{DemografixError: api.DemografixError{Status: 401}}, ExitAuth},
		{"nokey", config.ErrNoAPIKey, ExitAuth},
		{"sub", &api.SubscriptionError{DemografixError: api.DemografixError{Status: 402}}, ExitSubscript},
		{"validation", &api.ValidationError{DemografixError: api.DemografixError{Status: 422}}, ExitValidation},
		{"ratelimit", &api.RateLimitError{DemografixError: api.DemografixError{Status: 429}}, ExitRateLimit},
		{"transport", &api.TransportError{DemografixError: api.DemografixError{}, Err: errors.New("x")}, ExitTransport},
		{"other", errors.New("boom"), ExitGeneric},
	}
	for _, c := range cases {
		if got := Code(c.err); got != c.want {
			t.Errorf("%s: Code = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestValidateCountry(t *testing.T) {
	if err := validateCountry(""); err != nil {
		t.Error("empty country should be valid")
	}
	if err := validateCountry("US"); err != nil {
		t.Error("US should be valid")
	}
	if err := validateCountry("USA"); err == nil {
		t.Error("USA should be invalid")
	}
	if err := validateCountry("1!"); err == nil {
		t.Error("non-alpha should be invalid")
	}
}

func TestNormalizeCountry(t *testing.T) {
	if normalizeCountry("  us ") != "US" {
		t.Errorf("normalizeCountry = %q", normalizeCountry("  us "))
	}
}
