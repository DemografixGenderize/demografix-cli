package sheet

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

func parseXLSX(b []byte) (*Spreadsheet, error) {
	f, err := excelize.OpenReader(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("workbook has no sheets")
	}
	name := sheets[0]

	// The formatted grid gives the shape and lets us locate the header row and
	// skip blank rows; typed values are read per cell below.
	grid, err := f.GetRows(name)
	if err != nil {
		return nil, err
	}

	headerRow := -1
	for i, rec := range grid {
		if !allEmptyStr(rec) {
			headerRow = i
			break
		}
	}
	if headerRow < 0 {
		return nil, errors.New("xlsx has no header row")
	}

	headerRec := grid[headerRow]
	headers := make([]string, len(headerRec))
	for i, c := range headerRec {
		h := strings.TrimSpace(c)
		if h == "" {
			return nil, fmt.Errorf("blank header in column %d", i+1)
		}
		headers[i] = h
	}

	var rows [][]Cell
	for ri := headerRow + 1; ri < len(grid); ri++ {
		if allEmptyStr(grid[ri]) {
			continue
		}
		cells := make([]Cell, len(headers))
		for ci := range headers {
			cellName, err := excelize.CoordinatesToCellName(ci+1, ri+1)
			if err != nil {
				return nil, err
			}
			cells[ci] = xlsxCell(f, name, cellName)
		}
		rows = append(rows, cells)
	}

	if len(rows) == 0 {
		return nil, errors.New("xlsx has no data rows")
	}

	return &Spreadsheet{
		Format:   XLSX,
		Headers:  headers,
		Rows:     rows,
		Encoding: EncUTF8,
		BOM:      BOMNone,
		Meta:     Meta{SheetName: name},
	}, nil
}

// xlsxCell reconstructs the native cell type, mirroring the Elixir parser:
// numbers -> int64/float64, booleans -> bool, date-formatted cells -> Date/
// DateTime, errors/empty -> nil, formulas/strings -> their value. Plain numeric
// and date cells carry no `t` attribute, so excelize reports them as
// CellTypeUnset; the default branch reads the raw value and uses the cell's
// number format to tell a date from a number.
func xlsxCell(f *excelize.File, sheet, cell string) Cell {
	t, err := f.GetCellType(sheet, cell)
	if err != nil {
		return nil
	}
	switch t {
	case excelize.CellTypeBool:
		raw, _ := f.GetCellValue(sheet, cell, excelize.Options{RawCellValue: true})
		return raw == "1" || strings.EqualFold(raw, "true")
	case excelize.CellTypeError:
		return nil
	case excelize.CellTypeSharedString, excelize.CellTypeInlineString, excelize.CellTypeFormula:
		v, _ := f.GetCellValue(sheet, cell)
		if v == "" {
			return nil
		}
		return v
	default: // CellTypeNumber, CellTypeDate, CellTypeUnset
		raw, _ := f.GetCellValue(sheet, cell, excelize.Options{RawCellValue: true})
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil
		}
		serial, perr := strconv.ParseFloat(raw, 64)
		if perr != nil {
			// Not numeric after all; fall back to the formatted string.
			v, _ := f.GetCellValue(sheet, cell)
			if v == "" {
				return nil
			}
			return v
		}
		if isDateStyle(f, sheet, cell) {
			if tm, derr := excelize.ExcelDateToTime(serial, false); derr == nil {
				if tm.Hour() == 0 && tm.Minute() == 0 && tm.Second() == 0 && tm.Nanosecond() == 0 {
					return DateCell(tm)
				}
				return DateTimeCell(tm)
			}
		}
		if !strings.ContainsAny(raw, ".eE") {
			if iv, e := strconv.ParseInt(raw, 10, 64); e == nil {
				return iv
			}
		}
		return serial
	}
}

// isDateStyle reports whether the cell's number format is a date/time format
// (built-in ids 14-22 and 45-47, or a custom format with date/time tokens).
func isDateStyle(f *excelize.File, sheet, cell string) bool {
	styleID, err := f.GetCellStyle(sheet, cell)
	if err != nil {
		return false
	}
	st, err := f.GetStyle(styleID)
	if err != nil || st == nil {
		return false
	}
	switch st.NumFmt {
	case 14, 15, 16, 17, 18, 19, 20, 21, 22, 45, 46, 47:
		return true
	}
	if st.CustomNumFmt != nil && strings.ContainsAny(strings.ToLower(*st.CustomNumFmt), "ymdhs") {
		return true
	}
	return false
}

func emitXLSX(w io.Writer, s *Spreadsheet) error {
	const sheetName = "Classified"
	f := excelize.NewFile()
	defer f.Close()

	idx, err := f.NewSheet(sheetName)
	if err != nil {
		return err
	}
	f.SetActiveSheet(idx)
	_ = f.DeleteSheet("Sheet1")

	dateFmt := "yyyy-mm-dd"
	dateTimeFmt := "yyyy-mm-ddThh:mm:ss"
	dateStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: &dateFmt})
	if err != nil {
		return err
	}
	dateTimeStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: &dateTimeFmt})
	if err != nil {
		return err
	}

	sw, err := f.NewStreamWriter(sheetName)
	if err != nil {
		return err
	}

	header := make([]interface{}, len(s.Headers))
	for i, h := range s.Headers {
		header[i] = h
	}
	cell, _ := excelize.CoordinatesToCellName(1, 1)
	if err := sw.SetRow(cell, header); err != nil {
		return err
	}

	for r, row := range s.Rows {
		vals := make([]interface{}, len(s.Headers))
		for i := range s.Headers {
			var c Cell
			if i < len(row) {
				c = row[i]
			}
			vals[i] = xlsxValue(c, dateStyle, dateTimeStyle)
		}
		cell, _ := excelize.CoordinatesToCellName(1, r+2)
		if err := sw.SetRow(cell, vals); err != nil {
			return err
		}
	}

	if err := sw.Flush(); err != nil {
		return err
	}
	if _, err := f.WriteTo(w); err != nil {
		return err
	}
	return nil
}

func xlsxValue(c Cell, dateStyle, dateTimeStyle int) interface{} {
	switch v := c.(type) {
	case nil:
		return nil
	case string:
		return v
	case bool:
		return v
	case int:
		return v
	case int64:
		return v
	case float64:
		return v
	case DateCell:
		return excelize.Cell{StyleID: dateStyle, Value: time.Time(v)}
	case DateTimeCell:
		return excelize.Cell{StyleID: dateTimeStyle, Value: time.Time(v)}
	default:
		return CellToString(c)
	}
}
