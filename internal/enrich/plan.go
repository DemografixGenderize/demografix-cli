package enrich

import (
	"strings"

	"github.com/DemografixGenderize/demografix-cli/internal/sheet"
)

// PlanInfo describes a run without executing it (for --dry-run).
type PlanInfo struct {
	InputFormat   string
	OutputFormat  string
	OutputPath    string
	Rows          int
	Services      []string
	NameColumns   string
	Country       string
	OutputColumns []string
	Cost          int
	Resume        bool
}

// Plan validates the inputs and computes the cost without calling the API.
func Plan(o Options) (*PlanInfo, error) {
	s, r, err := prepare(o)
	if err != nil {
		return nil, err
	}

	outFormat, err := sheet.ResolveOutput(s.Format, o.OutputPath)
	if err != nil {
		return nil, err
	}

	cost := 0
	if r.gender {
		cost += billedRows(s, r, r.prefix+"gender_count")
	}
	if r.age {
		cost += billedRows(s, r, r.prefix+"age_count")
	}
	if r.nationality {
		cost += billedRows(s, r, r.prefix+"nationality_count")
	}

	info := &PlanInfo{
		InputFormat:   formatName(s.Format),
		OutputFormat:  formatName(outFormat),
		OutputPath:    o.OutputPath,
		Rows:          len(s.Rows),
		Services:      ServiceNames(o),
		NameColumns:   nameColumnDesc(s.Headers, r),
		Country:       countryDesc(s.Headers, r),
		OutputColumns: outputKeys(r),
		Cost:          cost,
		Resume:        r.resume,
	}
	return info, nil
}

// billedRows counts the rows that would consume quota for one service: every row
// with a non-empty name that is not already done (in resume mode).
func billedRows(s *sheet.Spreadsheet, r *resolved, countKey string) int {
	need := needSet(s, r, countKey, true)
	n := 0
	for i, row := range s.Rows {
		if !need[i] {
			continue
		}
		if strings.TrimSpace(combinedName(row, r)) != "" {
			n++
		}
	}
	return n
}

func nameColumnDesc(headers []string, r *resolved) string {
	if !r.splitMode {
		return colName(headers, r.fullCol)
	}
	var parts []string
	if r.firstCol >= 0 {
		parts = append(parts, "first="+colName(headers, r.firstCol))
	}
	if r.lastCol >= 0 {
		parts = append(parts, "last="+colName(headers, r.lastCol))
	}
	return strings.Join(parts, " ")
}

func countryDesc(headers []string, r *resolved) string {
	switch {
	case r.fixedCountry != "":
		return r.fixedCountry + " (fixed)"
	case r.countryCol >= 0:
		return colName(headers, r.countryCol) + " (column)"
	default:
		return "none (global)"
	}
}

func colName(headers []string, idx int) string {
	if idx < 0 || idx >= len(headers) {
		return "?"
	}
	return headers[idx]
}

func formatName(f sheet.Format) string {
	switch f {
	case sheet.CSV:
		return "csv"
	case sheet.TSV:
		return "tsv"
	case sheet.JSON:
		return "json"
	case sheet.JSONL:
		return "jsonl"
	case sheet.XLSX:
		return "xlsx"
	default:
		return "unknown"
	}
}
