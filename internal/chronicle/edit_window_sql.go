package chronicle

import (
	"database/sql"
	"fmt"
	"time"
)

func editWindowToSQL(w EditWindow) sql.NullInt64 {
	if w.Seconds == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *w.Seconds, Valid: true}
}

func editWindowFromSQL(n sql.NullInt64) (EditWindow, error) {
	if !n.Valid {
		return UnlimitedWindow(), nil
	}
	sec := n.Int64
	return EditWindow{Seconds: &sec}, nil
}

func editableUntilToSQL(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: formatTime(*t), Valid: true}
}

func parseEditableUntil(s sql.NullString) (*time.Time, error) {
	if !s.Valid {
		return nil, nil
	}
	t, err := parseTime(s.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func formatEditWindowLabel(w EditWindow) string {
	if w.IsChronicle() {
		return "летопись"
	}
	if w.IsUnlimited() {
		return "без ограничения"
	}
	sec := *w.Seconds
	if sec%(24*3600) == 0 {
		days := sec / (24 * 3600)
		return fmt.Sprintf("%d сут.", days)
	}
	if sec%3600 == 0 {
		hours := sec / 3600
		return fmt.Sprintf("%d ч", hours)
	}
	return fmt.Sprintf("%d с", sec)
}
