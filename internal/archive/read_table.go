package archive

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type rawFullTable struct{ Columns, Rows json.RawMessage }

// Retain only primitive cells and check shape while reading, before retaining
// rows. Objects and arrays supplied as cells never become arbitrary Go maps.
func decodeFullTable(raw rawFullTable, maxRows int) (tableJSON, error) {
	var t tableJSON
	columns := json.NewDecoder(bytes.NewReader(raw.Columns))
	if token, err := columns.Token(); err != nil || token != json.Delim('[') {
		return t, fmt.Errorf("archive: invalid columns")
	}
	for columns.More() {
		if len(t.Columns) >= 128 {
			return t, fmt.Errorf("archive: too many columns")
		}
		var col string
		if err := columns.Decode(&col); err != nil {
			return t, err
		}
		t.Columns = append(t.Columns, col)
	}
	if _, err := columns.Token(); err != nil {
		return t, err
	}
	// Empty exported tables used null rather than [].
	if bytes.Equal(bytes.TrimSpace(raw.Rows), []byte("null")) {
		return t, nil
	}
	rows := json.NewDecoder(bytes.NewReader(raw.Rows))
	if token, err := rows.Token(); err != nil || token != json.Delim('[') {
		return t, fmt.Errorf("archive: invalid rows")
	}
	for rows.More() {
		if len(t.Rows) >= maxRows || len(t.Columns) == 0 {
			return t, fmt.Errorf("archive: row budget or shape")
		}
		if token, err := rows.Token(); err != nil || token != json.Delim('[') {
			return t, fmt.Errorf("archive: invalid row")
		}
		row := make([]any, 0, len(t.Columns))
		for rows.More() {
			if len(row) >= len(t.Columns) {
				return t, fmt.Errorf("archive: row too long")
			}
			var value json.RawMessage
			if err := rows.Decode(&value); err != nil {
				return t, err
			}
			value = bytes.TrimSpace(value)
			if len(value) == 0 || value[0] == '{' || value[0] == '[' {
				return t, fmt.Errorf("archive: nonprimitive cell")
			}
			var cell any
			if err := json.Unmarshal(value, &cell); err != nil {
				return t, err
			}
			row = append(row, cell)
		}
		if _, err := rows.Token(); err != nil {
			return t, err
		}
		if len(row) != len(t.Columns) {
			return t, fmt.Errorf("archive: row too short")
		}
		t.Rows = append(t.Rows, row)
	}
	if _, err := rows.Token(); err != nil {
		return t, err
	}
	var extra any
	if err := rows.Decode(&extra); err != io.EOF {
		return t, fmt.Errorf("archive: trailing row data")
	}
	return t, nil
}
