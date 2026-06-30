package sheet

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormatFloatShort(t *testing.T) {
	cases := map[float64]string{1: "1.0", 0: "0.0", 0.98: "0.98", 0.831234: "0.831234"}
	for in, want := range cases {
		if got := FormatFloatShort(in); got != want {
			t.Errorf("FormatFloatShort(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestParseCSVCommaSniff(t *testing.T) {
	in := []byte("id,full_name,country\r\n1,Andrea Rossi,IT\r\n2,Mary Smith,US\r\n")
	s, err := parseDelimited(in, CSV, 0)
	if err != nil {
		t.Fatal(err)
	}
	if s.Meta.Delimiter != ',' {
		t.Errorf("delimiter = %q", s.Meta.Delimiter)
	}
	if len(s.Headers) != 3 || s.Headers[1] != "full_name" {
		t.Errorf("headers = %v", s.Headers)
	}
	if len(s.Rows) != 2 || CellToString(s.Rows[0][1]) != "Andrea Rossi" {
		t.Errorf("rows = %v", s.Rows)
	}
}

func TestParseCSVSemicolonSniff(t *testing.T) {
	s, err := parseDelimited([]byte("a;b;c\n1;2;3\n"), CSV, 0)
	if err != nil {
		t.Fatal(err)
	}
	if s.Meta.Delimiter != ';' {
		t.Errorf("delimiter = %q, want ';'", s.Meta.Delimiter)
	}
}

func TestBlankHeaderRejected(t *testing.T) {
	if _, err := parseDelimited([]byte("a,,c\n1,2,3\n"), CSV, 0); err == nil {
		t.Error("want error for blank header cell")
	}
}

func TestEmitCSVBOMandCRLF(t *testing.T) {
	s := &Spreadsheet{Format: CSV, Headers: []string{"a", "b"}, Rows: [][]Cell{{"x", int64(5)}, {nil, 0.5}}, Meta: Meta{Delimiter: ','}}
	var buf bytes.Buffer
	if err := emitDelimited(&buf, s); err != nil {
		t.Fatal(err)
	}
	out := buf.Bytes()
	if !bytes.HasPrefix(out, bomUTF8) {
		t.Fatal("missing UTF-8 BOM")
	}
	text := string(out[len(bomUTF8):])
	for _, want := range []string{"a,b\r\n", "x,5\r\n", ",0.5\r\n"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
}

func TestParseJSONKeyUnionAndEscapeOff(t *testing.T) {
	headers, rows, top, err := parseJSON([]byte(`[{"name":"a<b","n":1},{"n":2,"extra":true}]`))
	if err != nil {
		t.Fatal(err)
	}
	if top != TopArray {
		t.Error("want TopArray")
	}
	if strings.Join(headers, ",") != "name,n,extra" {
		t.Errorf("headers = %v (want first-appearance union)", headers)
	}
	if CellToString(rows[0][0]) != "a<b" {
		t.Errorf("row0 name = %v", rows[0][0])
	}
	if _, ok := rows[0][1].(int64); !ok {
		t.Errorf("integer not preserved as int64: %T", rows[0][1])
	}
	if rows[0][2] != nil {
		t.Errorf("missing key should be nil, got %v", rows[0][2])
	}

	s := &Spreadsheet{Format: JSON, Headers: headers, Rows: rows, Meta: Meta{TopLevel: TopArray}}
	var buf bytes.Buffer
	if err := emitJSON(&buf, s); err != nil {
		t.Fatal(err)
	}
	text := string(buf.Bytes()[len(bomUTF8):])
	if !strings.Contains(text, `"a<b"`) {
		t.Errorf("HTML escaping should be off: %s", text)
	}
	if !strings.HasPrefix(text, "[{") {
		t.Errorf("array envelope expected: %s", text)
	}
}

func TestParseJSONDataObject(t *testing.T) {
	headers, rows, top, err := parseJSON([]byte(`{"data":[{"a":1}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if top != TopDataObject {
		t.Error("want TopDataObject")
	}
	if len(headers) != 1 || headers[0] != "a" || len(rows) != 1 {
		t.Errorf("headers=%v rows=%v", headers, rows)
	}
}
