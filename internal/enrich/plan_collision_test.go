package enrich

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Plan (used by --dry-run) must report an output-column collision, matching Run.
func TestPlanReportsCollision(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "in.csv")
	if err := os.WriteFile(p, []byte("name,gender\nalice,?\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Plan(Options{InputPath: p, Gender: true, NameCol: "name", TopN: 3})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("want collision error from Plan, got %v", err)
	}
}
