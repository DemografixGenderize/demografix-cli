package sheet

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type kv struct {
	k string
	v Cell
}

// encodeCell renders a cell as compact JSON with short-form floats and HTML
// escaping disabled (matching Jason).
func encodeCell(c Cell) []byte {
	switch v := c.(type) {
	case nil:
		return []byte("null")
	case string:
		return jsonString(v)
	case bool:
		if v {
			return []byte("true")
		}
		return []byte("false")
	case int:
		return []byte(strconv.Itoa(v))
	case int64:
		return []byte(strconv.FormatInt(v, 10))
	case RawNumber:
		return []byte(string(v))
	case float64:
		return []byte(FormatFloatShort(v))
	case DateCell:
		return jsonString(CellToString(v))
	case DateTimeCell:
		return jsonString(CellToString(v))
	case []Cell:
		var b bytes.Buffer
		b.WriteByte('[')
		for i, e := range v {
			if i > 0 {
				b.WriteByte(',')
			}
			b.Write(encodeCell(e))
		}
		b.WriteByte(']')
		return b.Bytes()
	case map[string]Cell:
		var b bytes.Buffer
		b.WriteByte('{')
		first := true
		for k, e := range v {
			if !first {
				b.WriteByte(',')
			}
			first = false
			b.Write(jsonString(k))
			b.WriteByte(':')
			b.Write(encodeCell(e))
		}
		b.WriteByte('}')
		return b.Bytes()
	default:
		return []byte("null")
	}
}

func jsonString(s string) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return bytes.TrimRight(buf.Bytes(), "\n")
}

func buildRows(headers []string, objs [][]kv) [][]Cell {
	rows := make([][]Cell, len(objs))
	for i, pairs := range objs {
		m := make(map[string]Cell, len(pairs))
		for _, p := range pairs {
			m[p.k] = p.v
		}
		row := make([]Cell, len(headers))
		for j, h := range headers {
			if v, ok := m[h]; ok {
				row[j] = v
			}
		}
		rows[i] = row
	}
	return rows
}

func numberCell(n json.Number) Cell {
	s := n.String()
	if !strings.ContainsAny(s, ".eE") {
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i
		}
		// A pure integer that overflows int64 is preserved verbatim so it
		// round-trips exactly instead of losing precision through float64.
		return RawNumber(s)
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return RawNumber(s)
}

func readValue(dec *json.Decoder) (Cell, error) {
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch v := t.(type) {
	case json.Delim:
		switch v {
		case '{':
			return readObjectMap(dec)
		case '[':
			return readArray(dec)
		default:
			return nil, fmt.Errorf("unexpected token %q", v)
		}
	case json.Number:
		return numberCell(v), nil
	case string:
		return v, nil
	case bool:
		return v, nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected token %T", t)
	}
}

func readArray(dec *json.Decoder) ([]Cell, error) {
	var arr []Cell
	for dec.More() {
		v, err := readValue(dec)
		if err != nil {
			return nil, err
		}
		arr = append(arr, v)
	}
	if _, err := dec.Token(); err != nil { // closing ]
		return nil, err
	}
	return arr, nil
}

func readObjectMap(dec *json.Decoder) (map[string]Cell, error) {
	m := map[string]Cell{}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := kt.(string)
		if !ok {
			return nil, errors.New("object key is not a string")
		}
		v, err := readValue(dec)
		if err != nil {
			return nil, err
		}
		m[key] = v
	}
	if _, err := dec.Token(); err != nil { // closing }
		return nil, err
	}
	return m, nil
}

// readObjectPairs reads an object's key/value pairs in order; the opening { has
// already been consumed.
func readObjectPairs(dec *json.Decoder) ([]kv, error) {
	var pairs []kv
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := kt.(string)
		if !ok {
			return nil, errors.New("object key is not a string")
		}
		v, err := readValue(dec)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, kv{key, v})
	}
	if _, err := dec.Token(); err != nil { // closing }
		return nil, err
	}
	return pairs, nil
}

func parseJSON(b []byte) ([]string, [][]Cell, JSONTop, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()

	tok, err := dec.Token()
	if err != nil {
		return nil, nil, 0, fmt.Errorf("invalid JSON: %w", err)
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil, nil, 0, errors.New(`JSON must be an array of objects or {"data":[...]}`)
	}

	top := TopArray
	if delim == '{' {
		top = TopDataObject
		kt, err := dec.Token()
		if err != nil {
			return nil, nil, 0, err
		}
		if key, _ := kt.(string); key != "data" {
			return nil, nil, 0, errors.New(`unsupported top-level object (expected {"data":[...]})`)
		}
		at, err := dec.Token()
		if err != nil {
			return nil, nil, 0, err
		}
		if d, ok := at.(json.Delim); !ok || d != '[' {
			return nil, nil, 0, errors.New(`"data" must be an array`)
		}
	} else if delim != '[' {
		return nil, nil, 0, errors.New(`JSON must be an array of objects or {"data":[...]}`)
	}

	var headers []string
	seen := map[string]bool{}
	var objs [][]kv
	for dec.More() {
		ot, err := dec.Token()
		if err != nil {
			return nil, nil, 0, err
		}
		if d, ok := ot.(json.Delim); !ok || d != '{' {
			return nil, nil, 0, errors.New("array element is not an object")
		}
		pairs, err := readObjectPairs(dec)
		if err != nil {
			return nil, nil, 0, err
		}
		for _, p := range pairs {
			if !seen[p.k] {
				seen[p.k] = true
				headers = append(headers, p.k)
			}
		}
		objs = append(objs, pairs)
	}
	if len(objs) == 0 {
		return nil, nil, 0, errors.New("JSON has no objects")
	}
	return headers, buildRows(headers, objs), top, nil
}

func parseJSONL(b []byte) ([]string, [][]Cell, error) {
	var headers []string
	seen := map[string]bool{}
	var objs [][]kv

	for _, line := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "//") {
			return nil, nil, errors.New("JSONL must not contain comment lines")
		}
		if strings.HasPrefix(t, "[") {
			return nil, nil, errors.New("input looks like a JSON array, not JSONL")
		}
		dec := json.NewDecoder(strings.NewReader(t))
		dec.UseNumber()
		ot, err := dec.Token()
		if err != nil {
			return nil, nil, fmt.Errorf("invalid JSONL line: %w", err)
		}
		if d, ok := ot.(json.Delim); !ok || d != '{' {
			return nil, nil, errors.New("each JSONL line must be an object")
		}
		pairs, err := readObjectPairs(dec)
		if err != nil {
			return nil, nil, err
		}
		for _, p := range pairs {
			if !seen[p.k] {
				seen[p.k] = true
				headers = append(headers, p.k)
			}
		}
		objs = append(objs, pairs)
	}
	if len(objs) == 0 {
		return nil, nil, errors.New("JSONL has no objects")
	}
	return headers, buildRows(headers, objs), nil
}

func emitJSON(w io.Writer, s *Spreadsheet) error {
	// The leading UTF-8 BOM is intentional: it matches the Elixir browser tool's
	// emit/json.ex, which the CLI output must reproduce byte-for-byte.
	if _, err := w.Write(utf8BOM); err != nil {
		return err
	}
	open, closing := "[", "]"
	if s.Meta.TopLevel == TopDataObject {
		open, closing = `{"data":[`, "]}"
	}
	if _, err := io.WriteString(w, open); err != nil {
		return err
	}
	for i, row := range s.Rows {
		if i > 0 {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		if err := writeObject(w, s.Headers, row); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, closing)
	return err
}

func emitJSONL(w io.Writer, s *Spreadsheet) error {
	// The leading UTF-8 BOM is intentional: it matches the Elixir browser tool's
	// emit/jsonl.ex, which the CLI output must reproduce byte-for-byte.
	if _, err := w.Write(utf8BOM); err != nil {
		return err
	}
	for _, row := range s.Rows {
		if err := writeObject(w, s.Headers, row); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func writeObject(w io.Writer, keys []string, vals []Cell) error {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(jsonString(k))
		b.WriteByte(':')
		var v Cell
		if i < len(vals) {
			v = vals[i]
		}
		b.Write(encodeCell(v))
	}
	b.WriteByte('}')
	_, err := w.Write(b.Bytes())
	return err
}
