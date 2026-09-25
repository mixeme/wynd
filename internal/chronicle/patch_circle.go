package chronicle

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// PatchCircleInput — набор изменений настроек круга. nil означает «не трогать».
type PatchCircleInput struct {
	Name              *string
	EditWindow        *EditWindow
	Color             *string
	InviteWho         *string
	InviteKindDefault *string
}

// PatchCircle применяет все изменения настроек круга одной транзакцией.
//
// Раньше обработчик звал пять отдельных методов подряд, и каждый открывал
// свою транзакцию: неверный цвет после принятого имени оставлял круг в
// наполовину изменённом состоянии (QLT-3). Теперь значения проверяются до
// первой записи, а сами записи идут вместе.
func (c *Chronicle) PatchCircle(ctx context.Context, circleID, actorAccountID string, in PatchCircleInput, now time.Time) error {
	// 1. Проверка значений — до любого обращения к базе.
	var name string
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
		if name == "" {
			return ErrInvalid
		}
		if err := checkLen(name, MaxNameChars); err != nil {
			return err
		}
	}
	var color string
	if in.Color != nil {
		normalized, err := normalizeCircleColor(*in.Color)
		if err != nil {
			return err
		}
		color = normalized
	}
	var who string
	if in.InviteWho != nil {
		who = strings.TrimSpace(*in.InviteWho)
		if who != "all" && who != "owner" {
			return ErrInvalid
		}
	}
	var kind string
	if in.InviteKindDefault != nil {
		kind = strings.TrimSpace(*in.InviteKindDefault)
		if kind != "single" && kind != "multi" {
			return ErrInvalid
		}
	}
	if in.EditWindow != nil && in.EditWindow.Seconds != nil && *in.EditWindow.Seconds < 0 {
		return ErrInvalid
	}
	if in.Name == nil && in.EditWindow == nil && in.Color == nil &&
		in.InviteWho == nil && in.InviteKindDefault == nil {
		return nil
	}

	now = utcOrNow(now)
	tx, err := c.beginWrite(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// 2. Права — в той же транзакции (QLT-1).
	mem, err := c.requireSettingsTx(ctx, tx, circleID, actorAccountID)
	if err != nil {
		return err
	}
	updated := formatTime(now)

	if in.Name != nil {
		actorName, err := c.identityName(ctx, tx, mem.IdentityID)
		if err != nil {
			return err
		}
		if err := c.updateCircleField(ctx, tx, circleID, "name", name, updated); err != nil {
			return err
		}
		if _, err := c.appendEvent(ctx, tx, appendEventInput{
			circleID: circleID, eventType: "circle.renamed", isService: true,
			actorIdentityID: mem.IdentityID, actorName: actorName,
			payload: map[string]any{"name": name},
			summary: summaryCircleRenamed(name), now: now,
		}); err != nil {
			return err
		}
	}
	if in.EditWindow != nil {
		actorName, err := c.identityName(ctx, tx, mem.IdentityID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE circles SET edit_window_sec = ?, updated_at = ? WHERE id = ?
		`, editWindowToSQL(*in.EditWindow), updated, circleID); err != nil {
			return fmt.Errorf("update edit window: %w", err)
		}
		if _, err := c.appendEvent(ctx, tx, appendEventInput{
			circleID: circleID, eventType: "circle.edit_window_changed", isService: true,
			actorIdentityID: mem.IdentityID, actorName: actorName,
			payload: map[string]any{"edit_window_sec": in.EditWindow.Seconds},
			summary: summaryEditWindowChanged(formatEditWindowLabel(*in.EditWindow)), now: now,
		}); err != nil {
			return err
		}
	}
	if in.Color != nil {
		if err := c.updateCircleField(ctx, tx, circleID, "color", color, updated); err != nil {
			return err
		}
	}
	if in.InviteWho != nil {
		if err := c.updateCircleField(ctx, tx, circleID, "invite_who", who, updated); err != nil {
			return err
		}
	}
	if in.InviteKindDefault != nil {
		if err := c.updateCircleField(ctx, tx, circleID, "invite_kind_default", kind, updated); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// updateCircleField пишет одну колонку круга. Имя колонки — из списка выше,
// не из запроса.
func (c *Chronicle) updateCircleField(ctx context.Context, tx *sql.Tx, circleID, column, value, updated string) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE circles SET `+column+` = ?, updated_at = ? WHERE id = ?`, value, updated, circleID)
	if err != nil {
		return fmt.Errorf("update circle %s: %w", column, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
