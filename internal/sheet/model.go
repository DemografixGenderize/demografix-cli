// Package sheet is the CLI's canonical spreadsheet model plus parsers and
// emitters for CSV, TSV, JSON, JSONL and XLSX. It mirrors the Demografix browser
// tool's semantics so enriched output matches the web download.
package sheet

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// Format is a spreadsheet file format.
type Format int

const (
	CSV Format = iota
	TSV
	JSON
	JSONL
	XLSX
)

// Encoding records the detected input encoding (output is always UTF-8).
type Encoding int

const (
	EncUTF8 Encoding = iota
	EncUTF16LE
	EncUTF16BE
	EncWindows1252
)

// BOM records the detected byte-order mark.
type BOM int

const (
	BOMNone BOM = iota
	BOMUTF8
	BOMUTF16LE
	BOMUTF16BE
)

// JSONTop is the top-level shape of a JSON document.
type JSONTop int

const (
	TopArray JSONTop = iota
	TopDataObject
)

// Cell is one canonical cell value: nil | string | int64 | float64 | bool |
// DateCell | DateTimeCell | []Cell | map[string]Cell.
type Cell = any

// DateCell is a date-only value; DateTimeCell carries a time component.
type (
	DateCell     time.Time
	DateTimeCell time.Time
)

// RawNumber preserves a numeric literal that cannot be represented as a Go
// int64/float64 without loss (e.g. a 20-digit id). It is emitted verbatim.
type RawNumber string

// Meta carries format-specific parsing details needed to re-emit faithfully.
type Meta struct {
	Delimiter rune    // CSV/TSV
	TopLevel  JSONTop // JSON
	SheetName string  // XLSX
}

// Spreadsheet is the materialized representation of a parsed file.
type Spreadsheet struct {
	Format   Format
	Headers  []string
	Rows     [][]Cell
	Encoding Encoding
	BOM      BOM
	Meta     Meta
}

// CellToString stringifies a cell exactly like the Elixir cell_to_string/1.
func CellToString(c Cell) string {
	switch v := c.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return FormatFloatShort(v)
	case DateCell:
		return time.Time(v).Format("2006-01-02")
	case DateTimeCell:
		return time.Time(v).Format("2006-01-02T15:04:05")
	case RawNumber:
		return string(v)
	default:
		return string(encodeCell(v))
	}
}

// FormatFloatShort renders a float exactly like Erlang :short / Jason: the
// shortest round-trip digits, formatted positionally or in scientific notation
// (whichever is shorter; ties prefer positional), always keeping a decimal
// point. The mantissa carries a ".0" when whole, and the exponent is printed
// without a leading zero or '+' (e.g. "1.0e3", "1.23e-4", "0.0001").
func FormatFloatShort(f float64) string {
	if f == 0 {
		if math.Signbit(f) {
			return "-0.0"
		}
		return "0.0"
	}
	neg := math.Signbit(f)
	f = math.Abs(f)

	// Go 'e' with precision -1 yields the shortest round-trip digits, e.g.
	// "1e+03", "1.23456e+05", "9.9e-05".
	sci := strconv.FormatFloat(f, 'e', -1, 64)
	mant, expPart, _ := strings.Cut(sci, "e")
	exp, _ := strconv.Atoi(expPart)
	digits := strings.Replace(mant, ".", "", 1)

	pos := floatPositional(digits, exp)
	sciStr := floatScientific(digits, exp)
	out := pos
	if len(sciStr) < len(pos) {
		out = sciStr
	}
	if neg {
		return "-" + out
	}
	return out
}

// floatPositional renders the digit string with the decimal point placed per
// the scientific exponent e (value = digits[0].digits[1:] x 10^e).
func floatPositional(digits string, e int) string {
	n := len(digits)
	if e >= 0 {
		if e+1 >= n {
			return digits + strings.Repeat("0", e+1-n) + ".0"
		}
		return digits[:e+1] + "." + digits[e+1:]
	}
	return "0." + strings.Repeat("0", -e-1) + digits
}

// floatScientific renders d.ddde±E in Erlang's style (no leading-zero or '+'
// exponent, mantissa always carrying a decimal point).
func floatScientific(digits string, e int) string {
	var b strings.Builder
	b.WriteByte(digits[0])
	b.WriteByte('.')
	if len(digits) > 1 {
		b.WriteString(digits[1:])
	} else {
		b.WriteByte('0')
	}
	b.WriteByte('e')
	b.WriteString(strconv.Itoa(e))
	return b.String()
}

// DefaultMeta returns sensible Meta when converting to a format.
func DefaultMeta(f Format) Meta {
	switch f {
	case CSV:
		return Meta{Delimiter: ','}
	case TSV:
		return Meta{Delimiter: '\t'}
	case JSON:
		return Meta{TopLevel: TopArray}
	default:
		return Meta{}
	}
}
