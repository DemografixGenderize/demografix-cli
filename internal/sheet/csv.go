package sheet

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
)

func parseDelimited(b []byte, format Format, forced rune) (*Spreadsheet, error) {
	text, enc, bom, err := DetectAndDecode(b)
	if err != nil {
		return nil, err
	}

	delim := forced
	if delim == 0 {
		delim = sniffDelimiter(text)
	}

	r := csv.NewReader(strings.NewReader(text))
	r.Comma = delim
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", strings.ToUpper(formatExt(format)), err)
	}

	var kept [][]string
	for _, rec := range records {
		if allEmptyStr(rec) {
			continue
		}
		kept = append(kept, rec)
	}
	if len(kept) == 0 {
		return nil, errors.New("file has no rows")
	}

	header := make([]string, len(kept[0]))
	for i, h := range kept[0] {
		header[i] = strings.TrimSpace(h)
		if header[i] == "" {
			return nil, fmt.Errorf("blank header in column %d", i+1)
		}
	}
	if len(kept) < 2 {
		return nil, errors.New("file has no data rows")
	}

	rows := make([][]Cell, 0, len(kept)-1)
	for ri, rec := range kept[1:] {
		if len(rec) != len(header) {
			return nil, fmt.Errorf("row %d has %d columns, expected %d", ri+2, len(rec), len(header))
		}
		cells := make([]Cell, len(rec))
		for i, f := range rec {
			cells[i] = f
		}
		rows = append(rows, cells)
	}

	return &Spreadsheet{
		Format:   format,
		Headers:  header,
		Rows:     rows,
		Encoding: enc,
		BOM:      bom,
		Meta:     Meta{Delimiter: delim},
	}, nil
}

func sniffDelimiter(text string) rune {
	best := ','
	bestCols := 1
	for _, d := range []rune{',', ';', '\t', '|'} {
		if cols, ok := consistentCols(text, d); ok && cols > bestCols {
			bestCols = cols
			best = d
		}
	}
	return best
}

func consistentCols(text string, d rune) (int, bool) {
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = d
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	cols := -1
	seen := 0
	for seen < 10 {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, false
		}
		if allEmptyStr(rec) {
			continue
		}
		if cols == -1 {
			cols = len(rec)
		} else if len(rec) != cols {
			return 0, false
		}
		seen++
	}
	if cols < 2 {
		return cols, false
	}
	return cols, true
}

func emitDelimited(w io.Writer, s *Spreadsheet) error {
	if _, err := w.Write(utf8BOM); err != nil {
		return err
	}
	delim := s.Meta.Delimiter
	if delim == 0 {
		if s.Format == TSV {
			delim = '\t'
		} else {
			delim = ','
		}
	}

	if err := writeDelimitedRow(w, s.Headers, delim); err != nil {
		return err
	}
	for _, row := range s.Rows {
		rec := make([]string, len(s.Headers))
		for i := range s.Headers {
			if i < len(row) {
				rec[i] = CellToString(row[i])
			}
		}
		if err := writeDelimitedRow(w, rec, delim); err != nil {
			return err
		}
	}
	return nil
}

// writeDelimitedRow writes one CRLF-terminated record, matching NimbleCSV's
// quoting: a field is quoted only when it contains the quote char, the
// separator, or a newline. Leading whitespace and lone carriage returns are
// written verbatim (Go's encoding/csv would quote/alter them).
func writeDelimitedRow(w io.Writer, fields []string, delim rune) error {
	sep := string(delim)
	var b strings.Builder
	for i, f := range fields {
		if i > 0 {
			b.WriteString(sep)
		}
		if strings.ContainsRune(f, '"') || strings.Contains(f, sep) || strings.Contains(f, "\n") {
			b.WriteByte('"')
			b.WriteString(strings.ReplaceAll(f, `"`, `""`))
			b.WriteByte('"')
		} else {
			b.WriteString(f)
		}
	}
	b.WriteString("\r\n")
	_, err := io.WriteString(w, b.String())
	return err
}

func allEmptyStr(rec []string) bool {
	for _, f := range rec {
		if strings.TrimSpace(f) != "" {
			return false
		}
	}
	return true
}

func formatExt(f Format) string {
	switch f {
	case TSV:
		return "tsv"
	default:
		return "csv"
	}
}
