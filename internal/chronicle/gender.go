package chronicle

import (
	"context"
	"database/sql"
	"strings"
)

// Gender — род участника в круге для строк журнала (план 46, A5).
// Пустой — не выбран: строка без глагола.
type Gender string

const (
	GenderNone   Gender = ""
	GenderMale   Gender = "m"
	GenderFemale Gender = "f"
)

// ParseGender принимает "", "m", "f"; остальное — ErrInvalid.
func ParseGender(raw string) (Gender, error) {
	switch Gender(strings.TrimSpace(raw)) {
	case GenderNone:
		return GenderNone, nil
	case GenderMale:
		return GenderMale, nil
	case GenderFemale:
		return GenderFemale, nil
	}
	return GenderNone, ErrInvalid
}

// past — глагол прошедшего времени по роду: past(g, "вступил", "вступила").
// Для GenderNone вызывающий строит строку без глагола сам.
func past(g Gender, male, female string) string {
	if g == GenderFemale {
		return female
	}
	return male
}

func genderArg(g Gender) any {
	if g == GenderNone {
		return nil
	}
	return string(g)
}

// identityGender — род участия; не выбран или участия нет — GenderNone:
// строка без глагола лучше, чем сорванное событие.
func (c *Chronicle) identityGender(ctx context.Context, q querier, identityID string) Gender {
	var g sql.NullString
	if err := q.QueryRowContext(ctx, `SELECT gender FROM identities WHERE id = ?`, identityID).Scan(&g); err != nil {
		return GenderNone
	}
	return Gender(g.String)
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// setIdentityGender меняет род участия. Строки в ленту не даёт: старые
// строки не переписываются, новые пойдут по выбору.
func (c *Chronicle) setIdentityGender(ctx context.Context, q execer, identityID string, g Gender) error {
	_, err := q.ExecContext(ctx, `UPDATE identities SET gender = ? WHERE id = ?`, genderArg(g), identityID)
	return err
}

// IdentityGender — род участия для экрана «Кто вы в этом круге».
func (c *Chronicle) IdentityGender(ctx context.Context, identityID string) Gender {
	return c.identityGender(ctx, c.db, identityID)
}
