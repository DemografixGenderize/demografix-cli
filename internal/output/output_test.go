package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/DemografixGenderize/demografix-cli/internal/api"
)

func TestParseAndResolve(t *testing.T) {
	if f, _ := ParseFormat("jsonl"); f != FormatJSONL {
		t.Error("jsonl")
	}
	if _, err := ParseFormat("nope"); err == nil {
		t.Error("want error for unknown format")
	}
	if Resolve(FormatAuto, true) != FormatTable {
		t.Error("auto+tty -> table")
	}
	if Resolve(FormatAuto, false) != FormatJSONL {
		t.Error("auto+pipe -> jsonl")
	}
	if Resolve(FormatCSV, true) != FormatCSV {
		t.Error("explicit format kept")
	}
}

func TestRenderGenderizeTable(t *testing.T) {
	var b bytes.Buffer
	preds := []api.GenderizePrediction{{Name: "peter", Gender: "male", Probability: 1, Count: 5}}
	if err := RenderGenderize(&b, FormatTable, preds, false); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "peter") || !strings.Contains(out, "1.00") {
		t.Errorf("unexpected table:\n%s", out)
	}
}

func TestRenderGenderizeJSONL(t *testing.T) {
	var b bytes.Buffer
	preds := []api.GenderizePrediction{
		{Name: "peter", Gender: "male", Probability: 1, Count: 5},
		{Name: "lois", Gender: "female", Probability: 0.98, Count: 3},
	}
	if err := RenderGenderize(&b, FormatJSONL, preds, false); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(b.String()), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], `{"name":"peter"`) {
		t.Errorf("unexpected jsonl:\n%s", b.String())
	}
}

func TestNationalizeTableContinuationRows(t *testing.T) {
	var b bytes.Buffer
	preds := []api.NationalizePrediction{{
		Name:  "nguyen",
		Count: 100,
		Country: []api.NationalizeCountry{
			{CountryID: "VN", Probability: 0.83},
			{CountryID: "US", Probability: 0.04},
		},
	}}
	if err := RenderNationalize(&b, FormatTSV, preds); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	// header + 2 candidate rows
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d:\n%s", len(lines), b.String())
	}
	if lines[1] != "nguyen\tVN\t0.83\t100" {
		t.Errorf("row1 = %q", lines[1])
	}
	if lines[2] != "\tUS\t0.04\t" { // name + count blank on continuation
		t.Errorf("row2 = %q", lines[2])
	}
}

func TestQuotaFooterAndDuration(t *testing.T) {
	if QuotaFooter(api.Quota{}, "") != "" {
		t.Error("empty quota should yield no footer")
	}
	f := QuotaFooter(api.Quota{Limit: 25000, Remaining: 24987, Reset: 1314000}, "basic")
	if !strings.Contains(f, "24,987 / 25,000") || !strings.Contains(f, "basic") {
		t.Errorf("footer = %q", f)
	}
	if HumanDuration(1314000) != "15d 5h" {
		t.Errorf("duration = %q", HumanDuration(1314000))
	}
}

func TestRenderQuotaJSONHasResetAt(t *testing.T) {
	var b bytes.Buffer
	now := time.Date(2026, 6, 28, 4, 0, 0, 0, time.UTC)
	rl := api.RateLimit{Limit: 25000, Remaining: 24987, Reset: 0, Tier: "basic"}
	if err := RenderQuota(&b, FormatJSON, rl, now); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `"reset_at"`) || !strings.Contains(b.String(), `"tier": "basic"`) {
		t.Errorf("unexpected quota json:\n%s", b.String())
	}
}
