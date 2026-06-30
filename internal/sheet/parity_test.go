package sheet

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

// Expected strings are :erlang.float_to_binary(V, [:short]) ground truth.
func TestFormatFloatShortGolden(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0.0, "0.0"},
		{1.0, "1.0"},
		{0.5, "0.5"},
		{1000.0, "1.0e3"},
		{100000.0, "1.0e5"},
		{123456.0, "123456.0"},
		{1.0e6, "1.0e6"},
		{1.0e15, "1.0e15"},
		{1.0e16, "1.0e16"},
		{1.0e17, "1.0e17"},
		{1.0e21, "1.0e21"},
		{1.0e-3, "0.001"},
		{1.0e-4, "0.0001"},
		{9.9e-5, "9.9e-5"},
		{1.0e-5, "1.0e-5"},
		{0.000123, "1.23e-4"},
		{1.0e-6, "1.0e-6"},
		{1.5e-10, "1.5e-10"},
		{1.23456789e8, "123456789.0"},
		{12345.6, "12345.6"},
		{-0.000123, "-1.23e-4"},
		{3.14159, "3.14159"},
		{2.5e-4, "2.5e-4"},
		{0.831234, "0.831234"},
	}
	for _, c := range cases {
		if got := FormatFloatShort(c.in); got != c.want {
			t.Errorf("FormatFloatShort(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCSVEmitQuotingMatchesNimbleCSV(t *testing.T) {
	s := &Spreadsheet{
		Format:  CSV,
		Headers: []string{"a", "b"},
		Rows: [][]Cell{
			{" John", "x,y"},   // leading space stays unquoted; comma forces quoting
			{"plain", "he\"y"}, // embedded quote is doubled
		},
		Meta: Meta{Delimiter: ','},
	}
	var buf bytes.Buffer
	if err := emitDelimited(&buf, s); err != nil {
		t.Fatal(err)
	}
	out := string(bytes.TrimPrefix(buf.Bytes(), utf8BOM))
	want := "a,b\r\n John,\"x,y\"\r\nplain,\"he\"\"y\"\r\n"
	if out != want {
		t.Errorf("emit =\n%q\nwant\n%q", out, want)
	}
}

func TestCSVRaggedRowErrors(t *testing.T) {
	_, err := parseDelimited([]byte("a,b,c\n1,2,3\n4,5\n"), CSV, ',')
	if err == nil || !strings.Contains(err.Error(), "expected 3") {
		t.Errorf("want ragged-row error, got %v", err)
	}
}

func TestJSONBigIntRoundTrip(t *testing.T) {
	headers, rows, top, err := parseJSON([]byte(`[{"id":99999999999999999999,"n":1}]`))
	if err != nil {
		t.Fatal(err)
	}
	s := &Spreadsheet{Format: JSON, Headers: headers, Rows: rows, Meta: Meta{TopLevel: top}}
	var buf bytes.Buffer
	if err := emitJSON(&buf, s); err != nil {
		t.Fatal(err)
	}
	out := string(bytes.TrimPrefix(buf.Bytes(), utf8BOM))
	if out != `[{"id":99999999999999999999,"n":1}]` {
		t.Errorf("big-int round-trip = %s", out)
	}
}

func TestXLSXTypedParseToCSVAndJSON(t *testing.T) {
	f := excelize.NewFile()
	sh := f.GetSheetName(0)
	for cell, v := range map[string]interface{}{
		"A1": "name", "B1": "n", "C1": "flag", "D1": "when",
		"A2": "alice", "B2": 42, "C2": true,
	} {
		if err := f.SetCellValue(sh, cell, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SetCellValue(sh, "D2", time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	dateStyle, _ := f.NewStyle(&excelize.Style{NumFmt: 14}) // m/d/yy -> date type
	_ = f.SetCellStyle(sh, "D2", "D2", dateStyle)

	var xb bytes.Buffer
	if err := f.Write(&xb); err != nil {
		t.Fatal(err)
	}

	s, err := parseXLSX(xb.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	// xlsx -> csv: ISO date, "true", bare number.
	csvSheet := *s
	csvSheet.Format = CSV
	csvSheet.Meta = Meta{Delimiter: ','}
	var cb bytes.Buffer
	if err := WriteTo(&cb, &csvSheet); err != nil {
		t.Fatal(err)
	}
	if got := string(bytes.TrimPrefix(cb.Bytes(), utf8BOM)); !strings.Contains(got, "alice,42,true,2021-01-02\r\n") {
		t.Errorf("xlsx->csv = %q", got)
	}

	// xlsx -> json: number and bool unquoted, date as an ISO string.
	jsonSheet := *s
	jsonSheet.Format = JSON
	jsonSheet.Meta = Meta{TopLevel: TopArray}
	var jb bytes.Buffer
	if err := WriteTo(&jb, &jsonSheet); err != nil {
		t.Fatal(err)
	}
	js := string(bytes.TrimPrefix(jb.Bytes(), utf8BOM))
	for _, want := range []string{`"n":42`, `"flag":true`, `"when":"2021-01-02"`} {
		if !strings.Contains(js, want) {
			t.Errorf("xlsx->json missing %s in %s", want, js)
		}
	}
}
