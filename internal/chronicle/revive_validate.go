package chronicle

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
)

// ValidateArchiveTableShape runs before any indexing, including outside the DB layer.
func ValidateArchiveTableShape(name string, t FullArchiveTable) error {
	seen := map[string]bool{}
	for _, col := range t.Columns {
		if col == "" || seen[col] {
			return fmt.Errorf("%w: %s duplicate/empty column", ErrInvalid, name)
		}
		seen[col] = true
	}
	for _, row := range t.Rows {
		if len(row) != len(t.Columns) {
			return fmt.Errorf("%w: %s row length", ErrInvalid, name)
		}
	}
	return nil
}

type archiveColumn struct {
	kind     string
	required bool
	primary  bool
}

// ValidateReviveTables checks the complete input before media import or writes.
// Metadata comes from the current schema; missing columns with defaults remain
// compatible with older archive schemas. Account/blob tables are projections.
func (c *Chronicle) ValidateReviveTables(ctx context.Context, format, schema int, tables map[string]FullArchiveTable) error {
	if !ReviveSchemaKnown(format, schema) {
		return fmt.Errorf("%w: unknown archive schema", ErrInvalid)
	}
	for name, t := range tables {
		if err := ValidateArchiveTableShape(name, t); err != nil {
			return err
		}
	}
	if len(tables["circles"].Rows) != 1 {
		return fmt.Errorf("%w: archive must contain one circle", ErrInvalid)
	}
	allowed := map[string]bool{"blobs": true}
	for _, q := range fullArchiveQueries {
		allowed[q.name] = true
	}
	for name, t := range tables {
		if !allowed[name] {
			return fmt.Errorf("%w: unknown archive table", ErrInvalid)
		}
		rows, err := c.db.QueryContext(ctx, `SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info(?)`, name)
		if err != nil {
			return err
		}
		cols := map[string]archiveColumn{}
		for rows.Next() {
			var col, kind string
			var notnull, pk int
			var def sql.NullString
			if err := rows.Scan(&col, &kind, &notnull, &def, &pk); err != nil {
				rows.Close()
				return err
			}
			cols[col] = archiveColumn{kind: strings.ToUpper(kind), required: notnull != 0 && !def.Valid, primary: pk != 0}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, col := range t.Columns {
			if _, ok := cols[col]; !ok {
				return fmt.Errorf("%w: %s unknown column", ErrInvalid, name)
			}
		}
		required := []string{}
		if name == "accounts" {
			required = []string{"id", "email"}
		} else if name == "blobs" {
			required = []string{"id", "sha256", "size_bytes"}
		} else {
			for col, spec := range cols {
				if spec.required || spec.primary {
					required = append(required, col)
				}
			}
		}
		if len(t.Rows) > 0 {
			for _, col := range required {
				if !slices.Contains(t.Columns, col) {
					return fmt.Errorf("%w: %s missing %s", ErrInvalid, name, col)
				}
			}
		}
		primary := map[string]bool{}
		var blobBytes int64
		for _, row := range t.Rows {
			var key []any
			for i, value := range row {
				col := t.Columns[i]
				spec := cols[col]
				if value == nil {
					if spec.required || spec.primary {
						return fmt.Errorf("%w: %s.%s is null", ErrInvalid, name, col)
					}
					continue
				}
				if spec.kind == "INTEGER" {
					n, ok := archiveInteger(value)
					if !ok {
						return fmt.Errorf("%w: %s.%s is not an integer", ErrInvalid, name, col)
					}
					if name == "blobs" && col == "size_bytes" {
						if n < 0 || n > math.MaxInt64-blobBytes {
							return fmt.Errorf("%w: blob sizes", ErrInvalid)
						}
						blobBytes += n
					}
				} else if spec.kind == "TEXT" {
					v, ok := value.(string)
					if !ok || (spec.primary && v == "") {
						return fmt.Errorf("%w: %s.%s is not text", ErrInvalid, name, col)
					}
					if name == "blobs" && col == "sha256" {
						if b, err := hex.DecodeString(v); err != nil || len(b) != 32 {
							return fmt.Errorf("%w: blob digest", ErrInvalid)
						}
					}
				} else if spec.kind == "REAL" {
					// Дробные столбцы — координаты снимка и рамка кадра
					// (post_media, C81): из архива клиента приходят числами
					// JSON, при отделении на том же сервере — float64 из базы.
					switch value.(type) {
					case float64, float32, int64, int:
					default:
						return fmt.Errorf("%w: %s.%s is not a number", ErrInvalid, name, col)
					}
				} else {
					return fmt.Errorf("%w: unsupported archive column type", ErrInvalid)
				}
				if spec.primary {
					key = append(key, value)
				}
			}
			if len(key) != 0 {
				encoded, _ := json.Marshal(key)
				if primary[string(encoded)] {
					return fmt.Errorf("%w: duplicate archive key", ErrInvalid)
				}
				primary[string(encoded)] = true
			}
		}
	}
	// Foreign keys are grouped, preserving composite relationships (days, etc.).
	for name, t := range tables {
		if name == "accounts" || name == "blobs" {
			continue
		}
		rows, err := c.db.QueryContext(ctx, `SELECT id, "table", "from", "to" FROM pragma_foreign_key_list(?) ORDER BY id, seq`, name)
		if err != nil {
			return err
		}
		type reference struct {
			target   string
			from, to []string
		}
		refs := map[int]*reference{}
		for rows.Next() {
			var id int
			var target, from, to string
			if err := rows.Scan(&id, &target, &from, &to); err != nil {
				rows.Close()
				return err
			}
			if refs[id] == nil {
				refs[id] = &reference{target: target}
			}
			refs[id].from = append(refs[id].from, from)
			refs[id].to = append(refs[id].to, to)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, ref := range refs {
			// References to instance-local bookkeeping are cleared by reviveRow.
			if !allowed[ref.target] {
				continue
			}
			target := tables[ref.target]
			keys := map[string]bool{}
			for _, row := range target.Rows {
				if key, ok := archiveReferenceKey(target.Columns, row, ref.to); ok {
					keys[key] = true
				}
			}
			for _, row := range t.Rows {
				if key, ok := archiveReferenceKey(t.Columns, row, ref.from); ok && !keys[key] {
					return fmt.Errorf("%w: %s broken reference to %s", ErrInvalid, name, ref.target)
				}
			}
		}
	}
	return nil
}

func archiveReferenceKey(columns []string, row []any, names []string) (string, bool) {
	var key []string
	for _, name := range names {
		i := slices.Index(columns, name)
		if i < 0 || row[i] == nil || row[i] == "" {
			return "", false
		}
		key = append(key, fmt.Sprint(row[i]))
	}
	encoded, _ := json.Marshal(key)
	return string(encoded), true
}

func archiveInteger(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) || math.Trunc(n) != n || math.Abs(n) > 1<<53-1 {
			return 0, false
		}
		return int64(n), true
	case json.Number:
		value, err := n.Int64()
		return value, err == nil
	}
	return 0, false
}
