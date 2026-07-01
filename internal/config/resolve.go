package config

import (
	"errors"
	"os"
	"strings"
)

// Source identifies where a resolved API key came from.
type Source int

const (
	SourceNone Source = iota
	SourceEnv
	SourceConfig
)

// ErrNoAPIKey is returned when no key is found in any source. The CLI has no
// keyless mode.
var ErrNoAPIKey = errors.New("no API key configured; run `demografix login` or set DEMOGRAFIX_API_KEY")

// Resolved is a resolved API key plus where it came from.
type Resolved struct {
	Key    string
	Source Source
}

// Getenv reads an environment variable; injectable for testing.
type Getenv func(string) string

// ResolveAPIKey applies the precedence DEMOGRAFIX_API_KEY > config file. It
// returns ErrNoAPIKey when neither resolves — there is no keyless fallback.
func ResolveAPIKey(getenv Getenv, configPath string) (Resolved, error) {
	if getenv == nil {
		getenv = os.Getenv
	}

	if v := strings.TrimSpace(getenv(EnvAPIKey)); v != "" {
		return Resolved{Key: v, Source: SourceEnv}, nil
	}

	cfg, err := Load(configPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Resolved{}, err
	}
	if k := strings.TrimSpace(cfg.APIKey); k != "" {
		return Resolved{Key: k, Source: SourceConfig}, nil
	}

	return Resolved{}, ErrNoAPIKey
}
