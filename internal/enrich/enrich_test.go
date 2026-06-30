package enrich

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DemografixGenderize/demografix-cli/internal/api"
	"github.com/DemografixGenderize/demografix-cli/internal/sheet"
)

type fakePredictor struct{ gender string }

func (f fakePredictor) Genderize(_ context.Context, names []string, _ string) ([]api.GenderizePrediction, api.Quota, error) {
	out := make([]api.GenderizePrediction, len(names))
	for i, n := range names {
		out[i] = api.GenderizePrediction{Name: n, Gender: f.gender, Probability: 0.99, Count: 100}
	}
	return out, api.Quota{Limit: 1000, Remaining: 900, Reset: 60}, nil
}

func (f fakePredictor) Agify(_ context.Context, names []string, _ string) ([]api.AgifyPrediction, api.Quota, error) {
	out := make([]api.AgifyPrediction, len(names))
	age := 42
	for i, n := range names {
		out[i] = api.AgifyPrediction{Name: n, Age: &age, Count: 50}
	}
	return out, api.Quota{Limit: 1000, Remaining: 880}, nil
}

func (f fakePredictor) Nationalize(_ context.Context, names []string) ([]api.NationalizePrediction, api.Quota, error) {
	out := make([]api.NationalizePrediction, len(names))
	for i, n := range names {
		out[i] = api.NationalizePrediction{
			Name:  n,
			Count: 7,
			Country: []api.NationalizeCountry{
				{CountryID: "VN", Probability: 0.83},
				{CountryID: "US", Probability: 0.04},
			},
		}
	}
	return out, api.Quota{Limit: 1000, Remaining: 870}, nil
}

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func cs(c sheet.Cell) string { return sheet.CellToString(c) }

func TestRunGenderHappyPath(t *testing.T) {
	in := writeTemp(t, "people.csv", "id,full_name\n1,Andrea\n2,Mary\n")
	res, err := Run(context.Background(), fakePredictor{gender: "male"}, Options{InputPath: in, Gender: true, NameCol: "full_name"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(res.Out.Headers, ","); got != "id,full_name,gender,gender_count,gender_probability" {
		t.Fatalf("headers = %s", got)
	}
	r0 := res.Out.Rows[0]
	if cs(r0[2]) != "male" || cs(r0[3]) != "100" || cs(r0[4]) != "0.99" {
		t.Fatalf("row0 = %v", r0)
	}
	if res.NamesBilled != 2 {
		t.Errorf("billed = %d, want 2", res.NamesBilled)
	}
	if res.RowsWritten != 2 {
		t.Errorf("written = %d, want 2", res.RowsWritten)
	}
}

func TestRunEmptyNameSynthesizesNoMatch(t *testing.T) {
	in := writeTemp(t, "p.csv", "id,full_name\n1,\n2,Mary\n")
	res, err := Run(context.Background(), fakePredictor{gender: "male"}, Options{InputPath: in, Gender: true, NameCol: "full_name"})
	if err != nil {
		t.Fatal(err)
	}
	r0 := res.Out.Rows[0]
	if cs(r0[2]) != "" {
		t.Errorf("empty-name gender = %q, want blank", cs(r0[2]))
	}
	if cs(r0[3]) != "0" || cs(r0[4]) != "0.0" {
		t.Errorf("empty-name count/prob = %q/%q, want 0/0.0", cs(r0[3]), cs(r0[4]))
	}
	if res.NamesBilled != 1 {
		t.Errorf("billed = %d, want 1 (Mary only)", res.NamesBilled)
	}
}

func TestRunNationalityTopN(t *testing.T) {
	in := writeTemp(t, "p.csv", "name\nnguyen\n")
	res, err := Run(context.Background(), fakePredictor{}, Options{InputPath: in, Nationality: true, NameCol: "name", TopN: 2})
	if err != nil {
		t.Fatal(err)
	}
	want := "name,country_1,country_2,country_1_probability,country_2_probability,nationality_count"
	if got := strings.Join(res.Out.Headers, ","); got != want {
		t.Fatalf("headers = %s", got)
	}
	r0 := res.Out.Rows[0]
	if cs(r0[1]) != "VN" || cs(r0[2]) != "US" {
		t.Errorf("countries = %q,%q", cs(r0[1]), cs(r0[2]))
	}
	if cs(r0[3]) != "0.83" || cs(r0[5]) != "7" {
		t.Errorf("prob/count = %q,%q", cs(r0[3]), cs(r0[5]))
	}
}

func TestPlanCost(t *testing.T) {
	in := writeTemp(t, "p.csv", "full_name\nA\nB\nC\n")
	info, err := Plan(Options{InputPath: in, Gender: true, Age: true, NameCol: "full_name"})
	if err != nil {
		t.Fatal(err)
	}
	if info.Rows != 3 {
		t.Errorf("rows = %d", info.Rows)
	}
	if info.Cost != 6 {
		t.Errorf("cost = %d, want 6 (2 services * 3 rows)", info.Cost)
	}
}

func TestRunResumeFillsOnlyBlanks(t *testing.T) {
	in := writeTemp(t, "p.csv", "id,full_name,gender,gender_count,gender_probability\n1,Andrea,female,5,0.5\n2,Mary,,,\n")
	res, err := Run(context.Background(), fakePredictor{gender: "male"}, Options{InputPath: in, Gender: true, NameCol: "full_name", Resume: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Out.Headers) != 5 {
		t.Fatalf("headers grew: %v", res.Out.Headers)
	}
	if cs(res.Out.Rows[0][2]) != "female" {
		t.Errorf("done row was overwritten: %q", cs(res.Out.Rows[0][2]))
	}
	if cs(res.Out.Rows[1][2]) != "male" {
		t.Errorf("blank row not filled: %q", cs(res.Out.Rows[1][2]))
	}
	if res.NamesBilled != 1 {
		t.Errorf("billed = %d, want 1 (Mary only)", res.NamesBilled)
	}
}

func TestRunCollisionErrors(t *testing.T) {
	in := writeTemp(t, "p.csv", "full_name,gender\nAndrea,x\n")
	_, err := Run(context.Background(), fakePredictor{gender: "male"}, Options{InputPath: in, Gender: true, NameCol: "full_name"})
	var ce *ConfigError
	if err == nil || !asConfigError(err, &ce) {
		t.Fatalf("want ConfigError for collision, got %v", err)
	}
}

func asConfigError(err error, target **ConfigError) bool {
	ce, ok := err.(*ConfigError)
	if ok {
		*target = ce
	}
	return ok
}

func TestMissingNameColumnIsConfigError(t *testing.T) {
	in := writeTemp(t, "p.csv", "full_name\nAndrea\n")
	_, err := Run(context.Background(), fakePredictor{}, Options{InputPath: in, Gender: true})
	if _, ok := err.(*ConfigError); !ok {
		t.Fatalf("want ConfigError, got %v", err)
	}
}
