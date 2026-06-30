package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/DemografixGenderize/demografix-cli/internal/api"
)

// RenderGenderize writes gender predictions in the chosen format. withCountry
// adds the country_id column for tabular formats.
func RenderGenderize(w io.Writer, f Format, preds []api.GenderizePrediction, withCountry bool) error {
	switch f {
	case FormatJSON:
		return writeJSONArray(w, preds)
	case FormatJSONL:
		return writeJSONLines(w, preds)
	default:
		return writeTabular(w, f, genderizeTable(preds, withCountry))
	}
}

// RenderAgify writes age predictions in the chosen format.
func RenderAgify(w io.Writer, f Format, preds []api.AgifyPrediction, withCountry bool) error {
	switch f {
	case FormatJSON:
		return writeJSONArray(w, preds)
	case FormatJSONL:
		return writeJSONLines(w, preds)
	default:
		return writeTabular(w, f, agifyTable(preds, withCountry))
	}
}

// RenderNationalize writes nationality predictions in the chosen format.
func RenderNationalize(w io.Writer, f Format, preds []api.NationalizePrediction) error {
	switch f {
	case FormatJSON:
		return writeJSONArray(w, preds)
	case FormatJSONL:
		return writeJSONLines(w, preds)
	default:
		return writeTabular(w, f, nationalizeTable(preds))
	}
}

type table struct {
	headers []string
	rows    [][]string
}

func genderizeTable(preds []api.GenderizePrediction, withCountry bool) table {
	headers := []string{"name", "gender", "probability", "count"}
	if withCountry {
		headers = append(headers, "country_id")
	}
	rows := make([][]string, 0, len(preds))
	for _, p := range preds {
		row := []string{p.Name, p.Gender, prob2(p.Probability), strconv.Itoa(p.Count)}
		if withCountry {
			row = append(row, p.CountryID)
		}
		rows = append(rows, row)
	}
	return table{headers: headers, rows: rows}
}

func agifyTable(preds []api.AgifyPrediction, withCountry bool) table {
	headers := []string{"name", "age", "count"}
	if withCountry {
		headers = append(headers, "country_id")
	}
	rows := make([][]string, 0, len(preds))
	for _, p := range preds {
		age := ""
		if p.Age != nil {
			age = strconv.Itoa(*p.Age)
		}
		row := []string{p.Name, age, strconv.Itoa(p.Count)}
		if withCountry {
			row = append(row, p.CountryID)
		}
		rows = append(rows, row)
	}
	return table{headers: headers, rows: rows}
}

func nationalizeTable(preds []api.NationalizePrediction) table {
	headers := []string{"name", "country_id", "probability", "count"}
	var rows [][]string
	for _, p := range preds {
		if len(p.Country) == 0 {
			rows = append(rows, []string{p.Name, "", "", strconv.Itoa(p.Count)})
			continue
		}
		for i, c := range p.Country {
			name, count := "", ""
			if i == 0 {
				name = p.Name
				count = strconv.Itoa(p.Count)
			}
			rows = append(rows, []string{name, c.CountryID, prob2(c.Probability), count})
		}
	}
	return table{headers: headers, rows: rows}
}

func writeTabular(w io.Writer, f Format, t table) error {
	switch f {
	case FormatTSV:
		return writeDelimited(w, t, '\t')
	case FormatCSV:
		return writeDelimited(w, t, ',')
	default:
		return writeTable(w, t)
	}
}

func writeTable(w io.Writer, t table) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, strings.Join(upper(t.headers), "\t")); err != nil {
		return err
	}
	for _, row := range t.rows {
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func writeDelimited(w io.Writer, t table, comma rune) error {
	cw := csv.NewWriter(w)
	cw.Comma = comma
	if err := cw.Write(t.headers); err != nil {
		return err
	}
	for _, row := range t.rows {
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func writeJSONArray(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func writeJSONLines[T any](w io.Writer, items []T) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, it := range items {
		if err := enc.Encode(it); err != nil {
			return err
		}
	}
	return nil
}

func prob2(p float64) string { return strconv.FormatFloat(p, 'f', 2, 64) }

func upper(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = strings.ToUpper(s)
	}
	return out
}
