// Package enrich enriches a spreadsheet with gender/age/nationality predictions
// from the public API, joining results back onto the original rows. It mirrors
// the Demografix browser tool's column names and semantics.
package enrich

import (
	"fmt"
	"strconv"
	"strings"
)

// Options are the enrich command inputs.
type Options struct {
	InputPath  string
	OutputPath string

	Gender      bool
	Age         bool
	Nationality bool

	NameCol  string
	FirstCol string
	LastCol  string

	CountryCol   string
	FixedCountry string

	TopN   int
	Prefix string
	Resume bool

	Concurrency int
}

// ConfigError is a user-facing validation/column-resolution failure. The CLI
// maps it to a usage exit code.
type ConfigError struct{ msg string }

func (e *ConfigError) Error() string { return e.msg }

func cfgErr(format string, a ...any) error { return &ConfigError{msg: fmt.Sprintf(format, a...)} }

type resolved struct {
	splitMode bool
	fullCol   int
	firstCol  int
	lastCol   int

	countryCol   int
	fixedCountry string

	gender      bool
	age         bool
	nationality bool

	topN   int
	prefix string
	resume bool
}

func resolveColumn(headers []string, ref string) (int, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return -1, cfgErr("empty column reference")
	}
	if n, err := strconv.Atoi(ref); err == nil {
		if n < 1 || n > len(headers) {
			return -1, cfgErr("column index %d out of range (file has %d columns)", n, len(headers))
		}
		return n - 1, nil
	}
	idx := -1
	count := 0
	var positions []int
	for i, h := range headers {
		if h == ref {
			count++
			idx = i
			positions = append(positions, i+1)
		}
	}
	switch {
	case count == 0:
		return -1, cfgErr("no column named %q; headers are: %s", ref, strings.Join(headers, ", "))
	case count > 1:
		return -1, cfgErr("ambiguous column %q: appears at positions %v; use a 1-based index", ref, positions)
	}
	return idx, nil
}

func resolveConfig(headers []string, o Options) (*resolved, error) {
	if !o.Gender && !o.Age && !o.Nationality {
		return nil, cfgErr("enable at least one output (--gender, --age, or --nationality)")
	}

	r := &resolved{
		fullCol:     -1,
		firstCol:    -1,
		lastCol:     -1,
		countryCol:  -1,
		gender:      o.Gender,
		age:         o.Age,
		nationality: o.Nationality,
		prefix:      o.Prefix,
		resume:      o.Resume,
		topN:        o.TopN,
	}
	if r.topN < 1 {
		r.topN = 1
	}
	if r.topN > 5 {
		r.topN = 5
	}

	hasFull := strings.TrimSpace(o.NameCol) != ""
	hasSplit := strings.TrimSpace(o.FirstCol) != "" || strings.TrimSpace(o.LastCol) != ""
	switch {
	case hasFull && hasSplit:
		return nil, cfgErr("use either --name-col or --first-name-col/--last-name-col, not both")
	case hasFull:
		idx, err := resolveColumn(headers, o.NameCol)
		if err != nil {
			return nil, err
		}
		r.fullCol = idx
	case hasSplit:
		r.splitMode = true
		if strings.TrimSpace(o.FirstCol) != "" {
			idx, err := resolveColumn(headers, o.FirstCol)
			if err != nil {
				return nil, err
			}
			r.firstCol = idx
		}
		if strings.TrimSpace(o.LastCol) != "" {
			idx, err := resolveColumn(headers, o.LastCol)
			if err != nil {
				return nil, err
			}
			r.lastCol = idx
		}
	default:
		return nil, cfgErr("provide a name column (--name-col, or --first-name-col / --last-name-col)")
	}

	hasCountryCol := strings.TrimSpace(o.CountryCol) != ""
	hasFixed := strings.TrimSpace(o.FixedCountry) != ""
	if hasCountryCol && hasFixed {
		return nil, cfgErr("use either --country-col or --country, not both")
	}
	if hasCountryCol {
		idx, err := resolveColumn(headers, o.CountryCol)
		if err != nil {
			return nil, err
		}
		r.countryCol = idx
	}
	if hasFixed {
		r.fixedCountry = strings.ToUpper(strings.TrimSpace(o.FixedCountry))
	}

	// Output columns must not collide with input headers (a fresh run appends
	// them). In resume mode the columns are expected to pre-exist. Checked here
	// so --dry-run/Plan and Run agree.
	if !r.resume {
		for _, k := range outputKeys(r) {
			if indexOf(headers, k) >= 0 {
				return nil, cfgErr("output column %q already exists in the input; pass --prefix to disambiguate (e.g. --prefix pred_)", k)
			}
		}
	}

	return r, nil
}

// outputKeys returns the appended column names in the canonical Emit order, with
// the prefix applied uniformly.
func outputKeys(r *resolved) []string {
	p := r.prefix
	var keys []string
	if r.gender {
		keys = append(keys, p+"gender", p+"gender_count", p+"gender_probability")
	}
	if r.age {
		keys = append(keys, p+"age", p+"age_count")
	}
	if r.nationality {
		for i := 1; i <= r.topN; i++ {
			keys = append(keys, fmt.Sprintf("%scountry_%d", p, i))
		}
		for i := 1; i <= r.topN; i++ {
			keys = append(keys, fmt.Sprintf("%scountry_%d_probability", p, i))
		}
		keys = append(keys, p+"nationality_count")
	}
	return keys
}

// ServiceNames lists the enabled services in canonical order.
func ServiceNames(o Options) []string {
	var s []string
	if o.Gender {
		s = append(s, "gender")
	}
	if o.Age {
		s = append(s, "age")
	}
	if o.Nationality {
		s = append(s, "nationality")
	}
	return s
}

func indexOf(ss []string, target string) int {
	for i, s := range ss {
		if s == target {
			return i
		}
	}
	return -1
}
