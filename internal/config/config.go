// Package config resolves the API key and reads/writes the CLI config file.
package config

import (
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// EnvAPIKey is the environment variable consulted for the API key.
const EnvAPIKey = "DEMOGRAFIX_API_KEY"

// Config is the on-disk CLI configuration.
type Config struct {
	APIKey string `toml:"api_key"`
}

// DefaultPath returns the config file path, honoring XDG_CONFIG_HOME and
// falling back to ~/.config on every platform.
func DefaultPath() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "demografix", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "demografix", "config.toml"), nil
}

// Load reads the config file. A missing file surfaces as os.ErrNotExist.
func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := toml.Unmarshal(b, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Save writes the config atomically (temp file + rename), with the directory at
// 0700 and the file at 0600. The key is never passed on a command line.
func Save(path string, c Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".config-*.toml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := toml.NewEncoder(tmp).Encode(c); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
