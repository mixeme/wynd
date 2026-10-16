package chronicle

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// Раскладка улочки (план 46, A2): закрепления и группы кругов — настройка
// учётки на этом сервере. Сервер знает только свои круги; группы всех
// серверов человека приложение складывает по названию. Событий хроники нет —
// это не сказанное и не структура круга.

// StreetGroup — группа учётки на этом сервере: название и её круги.
type StreetGroup struct {
	Name      string
	CircleIDs []string
}

// AccountPins — когда закреплён каждый закреплённый круг учётки.
func (c *Chronicle) AccountPins(ctx context.Context, accountID string) (map[string]time.Time, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT circle_id, pinned_at FROM circle_pins WHERE account_id = ?`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]time.Time{}
	for rows.Next() {
		var id, at string
		if err := rows.Scan(&id, &at); err != nil {
			return nil, err
		}
		out[id], _ = parseTime(at)
	}
	return out, rows.Err()
}

// AccountGroups — группы учётки по названию; круги внутри — в порядке
// добавления (на улочке их всё равно ставят по свежести).
func (c *Chronicle) AccountGroups(ctx context.Context, accountID string) ([]StreetGroup, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT g.name, i.circle_id
		FROM circle_groups g
		JOIN circle_group_items i ON i.group_id = g.id
		WHERE g.account_id = ?
		ORDER BY g.name, i.added_at, i.circle_id
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StreetGroup
	for rows.Next() {
		var name, circleID string
		if err := rows.Scan(&name, &circleID); err != nil {
			return nil, err
		}
		if len(out) == 0 || out[len(out)-1].Name != name {
			out = append(out, StreetGroup{Name: name})
		}
		out[len(out)-1].CircleIDs = append(out[len(out)-1].CircleIDs, circleID)
	}
	return out, rows.Err()
}

// PinCircle закрепляет круг. Время приходит с устройства: при переносе
// закреплений с устройства на сервер порядок закреплённых сохраняется.
// Закрепить можно круг, который виден на улочке, — и тот, что читаешь,
// выйдя.
func (c *Chronicle) PinCircle(ctx context.Context, circleID, accountID string, at time.Time) error {
	if _, err := c.streetMembership(ctx, circleID, accountID); err != nil {
		return err
	}
	_, err := c.db.ExecContext(ctx, `
		INSERT INTO circle_pins (account_id, circle_id, pinned_at) VALUES (?, ?, ?)
		ON CONFLICT (account_id, circle_id) DO UPDATE SET pinned_at = excluded.pinned_at
	`, accountID, circleID, formatTime(at))
	return err
}

// UnpinCircle снимает закрепление; снятого и не было — не ошибка.
func (c *Chronicle) UnpinCircle(ctx context.Context, circleID, accountID string) error {
	_, err := c.db.ExecContext(ctx, `DELETE FROM circle_pins WHERE account_id = ? AND circle_id = ?`, accountID, circleID)
	return err
}

// PutCircleInGroup кладёт круг в группу по названию: группы нет — заводится,
// круг был в другой — переезжает (а та, опустев, уходит). Класть можно только
// круг, где человек сейчас: вышедший из круга уходит и из групп.
func (c *Chronicle) PutCircleInGroup(ctx context.Context, circleID, accountID, name string, now time.Time) error {
	name, err := groupName(name)
	if err != nil {
		return err
	}
	mem, err := c.streetMembership(ctx, circleID, accountID)
	if err != nil {
		return err
	}
	if mem.Status != StatusActive {
		return ErrForbidden
	}
	tx, err := c.beginWrite(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	groupID, err := ensureGroup(ctx, tx, accountID, name, now)
	if err != nil {
		return err
	}
	var current string
	err = tx.QueryRowContext(ctx, `SELECT group_id FROM circle_group_items WHERE account_id = ? AND circle_id = ?`, accountID, circleID).Scan(&current)
	switch {
	case err == nil && current == groupID:
		return tx.Commit()
	case err == nil:
		if _, err := tx.ExecContext(ctx, `DELETE FROM circle_group_items WHERE account_id = ? AND circle_id = ?`, accountID, circleID); err != nil {
			return err
		}
	case err != sql.ErrNoRows:
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO circle_group_items (group_id, account_id, circle_id, added_at) VALUES (?, ?, ?, ?)
	`, groupID, accountID, circleID, formatTime(now)); err != nil {
		return err
	}
	return tx.Commit()
}

// RemoveCircleFromGroup убирает круг из его группы; опустевшая группа уходит.
func (c *Chronicle) RemoveCircleFromGroup(ctx context.Context, circleID, accountID string) error {
	_, err := c.db.ExecContext(ctx, `DELETE FROM circle_group_items WHERE account_id = ? AND circle_id = ?`, accountID, circleID)
	return err
}

// RenameGroup переименовывает группу — одна запись на сервер. Группа с новым
// названием уже есть — круги переходят в неё: на улочке группы и так
// складываются по названию. Группы нет — не ошибка: на этом сервере её кругов
// уже не осталось.
func (c *Chronicle) RenameGroup(ctx context.Context, accountID, from, to string) error {
	from, err := groupName(from)
	if err != nil {
		return err
	}
	to, err = groupName(to)
	if err != nil {
		return err
	}
	if from == to {
		return nil
	}
	tx, err := c.beginWrite(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var fromID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM circle_groups WHERE account_id = ? AND name = ?`, accountID, from).Scan(&fromID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	var toID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM circle_groups WHERE account_id = ? AND name = ?`, accountID, to).Scan(&toID)
	switch {
	case err == sql.ErrNoRows:
		if _, err := tx.ExecContext(ctx, `UPDATE circle_groups SET name = ? WHERE id = ?`, to, fromID); err != nil {
			return err
		}
	case err != nil:
		return err
	default:
		if _, err := tx.ExecContext(ctx, `UPDATE circle_group_items SET group_id = ? WHERE group_id = ?`, toID, fromID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM circle_groups WHERE id = ?`, fromID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DropGroup удаляет группу; её круги остаются на улочке без группы.
func (c *Chronicle) DropGroup(ctx context.Context, accountID, name string) error {
	name, err := groupName(name)
	if err != nil {
		return err
	}
	_, err = c.db.ExecContext(ctx, `DELETE FROM circle_groups WHERE account_id = ? AND name = ?`, accountID, name)
	return err
}

// streetMembership — круг на улочке человека: участие есть и не ушло совсем.
func (c *Chronicle) streetMembership(ctx context.Context, circleID, accountID string) (Membership, error) {
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return Membership{}, err
	}
	if mem.Status == StatusGone {
		return Membership{}, ErrForbidden
	}
	return mem, nil
}

func ensureGroup(ctx context.Context, tx *sql.Tx, accountID, name string, now time.Time) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id FROM circle_groups WHERE account_id = ? AND name = ?`, accountID, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	if id, err = newID(); err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO circle_groups (id, account_id, name, created_at) VALUES (?, ?, ?, ?)
	`, id, accountID, name, formatTime(now))
	return id, err
}

func groupName(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrInvalid
	}
	if err := checkLen(s, MaxNameChars); err != nil {
		return "", err
	}
	return s, nil
}
