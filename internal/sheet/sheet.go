package sheet

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// UnsupportedFormatError is returned for an unrecognized input or output file
// extension. The CLI maps it to a usage exit code.
type UnsupportedFormatError struct {
	Ext    string
	Output bool
}

func (e *UnsupportedFormatError) Error() string {
	kind := "file"
	if e.Output {
		kind = "output file"
	}
	return fmt.Sprintf("unsupported %s extension %q (supported: .csv .tsv .json .jsonl .xlsx)", kind, e.Ext)
}

// extFormat maps a file extension to a Format.
func extFormat(path string) (Format, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv":
		return CSV, true
	case ".tsv":
		return TSV, true
	case ".json":
		return JSON, true
	case ".jsonl", ".ndjson":
		return JSONL, true
	case ".xlsx", ".xlsm", ".xls":
		return XLSX, true
	}
	return 0, false
}

// Parse reads and parses a spreadsheet file, dispatching by extension.
func Parse(path string) (*Spreadsheet, error) {
	format, ok := extFormat(path)
	if !ok {
		return nil, &UnsupportedFormatError{Ext: strings.ToLower(filepath.Ext(path))}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	switch format {
	case CSV:
		return parseDelimited(b, CSV, 0)
	case TSV:
		return parseDelimited(b, TSV, '\t')
	case JSON:
		text, enc, bom, err := DetectAndDecode(b)
		if err != nil {
			return nil, err
		}
		headers, rows, top, err := parseJSON([]byte(text))
		if err != nil {
			return nil, err
		}
		return &Spreadsheet{Format: JSON, Headers: headers, Rows: rows, Encoding: enc, BOM: bom, Meta: Meta{TopLevel: top}}, nil
	case JSONL:
		text, enc, bom, err := DetectAndDecode(b)
		if err != nil {
			return nil, err
		}
		headers, rows, err := parseJSONL([]byte(text))
		if err != nil {
			return nil, err
		}
		return &Spreadsheet{Format: JSONL, Headers: headers, Rows: rows, Encoding: enc, BOM: bom}, nil
	case XLSX:
		return parseXLSX(b)
	default:
		return nil, fmt.Errorf("unsupported format")
	}
}

// ResolveOutput picks the output format from the -o file extension, or mirrors
// the input format when no output path is given. .xls/.xlsm always emit .xlsx.
func ResolveOutput(inputFormat Format, outputPath string) (Format, error) {
	if outputPath == "" || outputPath == "-" {
		return inputFormat, nil
	}
	f, ok := extFormat(outputPath)
	if !ok {
		return 0, &UnsupportedFormatError{Ext: strings.ToLower(filepath.Ext(outputPath)), Output: true}
	}
	return f, nil
}

// WriteTo emits the spreadsheet in its Format.
func WriteTo(w io.Writer, s *Spreadsheet) error {
	switch s.Format {
	case CSV, TSV:
		return emitDelimited(w, s)
	case JSON:
		return emitJSON(w, s)
	case JSONL:
		return emitJSONL(w, s)
	case XLSX:
		return emitXLSX(w, s)
	default:
		return fmt.Errorf("cannot emit unknown format")
	}
}

// WriteFile writes the spreadsheet to a file path.
func WriteFile(path string, s *Spreadsheet) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := WriteTo(f, s); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
