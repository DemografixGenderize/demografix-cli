package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Source identifies where a resolved API key came from.
type Source int

const (
	SourceNone Source = iota
	SourceFile
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

// ResolveAPIKey applies the precedence --api-key-file > DEMOGRAFIX_API_KEY >
// config file. It returns ErrNoAPIKey when none resolves — there is no keyless
// fallback.
func ResolveAPIKey(keyFile string, getenv Getenv, configPath string) (Resolved, error) {
	if getenv == nil {
		getenv = os.Getenv
	}

	if keyFile != "" {
		b, err := os.ReadFile(keyFile)
		if err != nil {
			return Resolved{}, fmt.Errorf("read api key file %q: %w", keyFile, err)
		}
		key := strings.TrimSpace(string(b))
		if key == "" {
			return Resolved{}, fmt.Errorf("api key file %q is empty", keyFile)
		}
		return Resolved{Key: key, Source: SourceFile}, nil
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
