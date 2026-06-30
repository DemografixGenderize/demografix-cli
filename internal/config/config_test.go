package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePrecedence(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	if err := Save(cfgPath, Config{APIKey: "fromconfig"}); err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(dir, "key.txt")
	if err := os.WriteFile(keyFile, []byte("  fromfile\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	env := func(string) string { return "fromenv" }
	noenv := func(string) string { return "" }

	if r, err := ResolveAPIKey(keyFile, env, cfgPath); err != nil || r.Key != "fromfile" || r.Source != SourceFile {
		t.Errorf("file precedence: %+v err %v", r, err)
	}
	if r, err := ResolveAPIKey("", env, cfgPath); err != nil || r.Key != "fromenv" || r.Source != SourceEnv {
		t.Errorf("env precedence: %+v err %v", r, err)
	}
	if r, err := ResolveAPIKey("", noenv, cfgPath); err != nil || r.Key != "fromconfig" || r.Source != SourceConfig {
		t.Errorf("config precedence: %+v err %v", r, err)
	}
	if _, err := ResolveAPIKey("", noenv, filepath.Join(dir, "missing.toml")); err == nil {
		t.Errorf("want ErrNoAPIKey when nothing resolves")
	}
}

func TestSavePermsAndRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "config.toml")
	if err := Save(p, Config{APIKey: "k"}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perm = %v, want 0600", perm)
	}
	got, err := Load(p)
	if err != nil || got.APIKey != "k" {
		t.Errorf("load = %+v err %v", got, err)
	}
}
