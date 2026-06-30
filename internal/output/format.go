// Package output renders predictions and quota to the terminal (table), or to
// json/jsonl/tsv/csv for machine consumers.
package output

import (
	"fmt"
	"strings"
)

// Format is an output format.
type Format int

const (
	FormatAuto Format = iota
	FormatTable
	FormatJSON
	FormatJSONL
	FormatTSV
	FormatCSV
)

// ParseFormat parses the -o flag value.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return FormatAuto, nil
	case "table":
		return FormatTable, nil
	case "json":
		return FormatJSON, nil
	case "jsonl", "ndjson":
		return FormatJSONL, nil
	case "tsv":
		return FormatTSV, nil
	case "csv":
		return FormatCSV, nil
	default:
		return FormatAuto, fmt.Errorf("unknown output format %q (want table|json|jsonl|tsv|csv)", s)
	}
}

// Resolve turns Auto into table on a TTY and jsonl when piped.
func Resolve(f Format, stdoutIsTTY bool) Format {
	if f != FormatAuto {
		return f
	}
	if stdoutIsTTY {
		return FormatTable
	}
	return FormatJSONL
}
