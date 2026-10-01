package chronicle

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// InviteSettings holds circle invite policy.
type InviteSettings struct {
	InviteWho         string
	InviteKindDefault string
}

// GetInviteSettings returns invite policy for a circle.
func (c *Chronicle) GetInviteSettings(ctx context.Context, circleID string) (InviteSettings, error) {
	var out InviteSettings
	err := c.db.QueryRowContext(ctx, `
		SELECT invite_who, invite_kind_default FROM circles WHERE id = ?
	`, circleID).Scan(&out.InviteWho, &out.InviteKindDefault)
	if err == sql.ErrNoRows {
		return InviteSettings{}, ErrNotFound
	}
	return out, err
}

// SetInviteWho changes who may create circle invites (can_settings required).
func (c *Chronicle) SetInviteWho(ctx context.Context, circleID, actorAccountID, who string, now time.Time) error {
	if err := c.RequireSettings(ctx, circleID, actorAccountID); err != nil {
		return err
	}
	who = strings.TrimSpace(who)
	if who != "all" && who != "owner" {
		return ErrInvalid
	}
	updated := formatTime(now)
	res, err := c.db.ExecContext(ctx, `UPDATE circles SET invite_who = ?, updated_at = ? WHERE id = ?`,
		who, updated, circleID)
	if err != nil {
		return fmt.Errorf("update invite_who: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetInviteKindDefault sets which invite kinds may be created (single only vs multi allowed).
func (c *Chronicle) SetInviteKindDefault(ctx context.Context, circleID, actorAccountID, kind string, now time.Time) error {
	if err := c.RequireSettings(ctx, circleID, actorAccountID); err != nil {
		return err
	}
	kind = strings.TrimSpace(kind)
	if kind != "single" && kind != "multi" {
		return ErrInvalid
	}
	updated := formatTime(now)
	res, err := c.db.ExecContext(ctx, `UPDATE circles SET invite_kind_default = ?, updated_at = ? WHERE id = ?`,
		kind, updated, circleID)
	if err != nil {
		return fmt.Errorf("update invite_kind_default: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetMemberCanSettings toggles settings access for a member (owner only).
// Выдача и снятие пишут служебную строку журнала. Повтор того же значения
// строку не добавляет.
func (c *Chronicle) SetMemberCanSettings(ctx context.Context, circleID, ownerAccountID, targetAccountID string, canSettings bool, now time.Time) error {
	if err := c.RequireOwner(ctx, circleID, ownerAccountID); err != nil {
		return err
	}
	tx, err := c.beginWrite(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	owner, err := c.circleOwner(ctx, tx, circleID)
	if err != nil {
		return err
	}
	if owner != ownerAccountID || targetAccountID == owner {
		return ErrForbidden
	}
	mem, err := c.membership(ctx, tx, circleID, targetAccountID)
	if err != nil {
		return err
	}
	if mem.Status != StatusActive {
		return ErrInvalid
	}
	if mem.CanSettings == canSettings {
		return nil
	}
	ownerMem, err := c.membership(ctx, tx, circleID, ownerAccountID)
	if err != nil {
		return err
	}
	ownerName, err := c.identityName(ctx, tx, ownerMem.IdentityID)
	if err != nil {
		return err
	}
	targetName, err := c.identityName(ctx, tx, mem.IdentityID)
	if err != nil {
		return err
	}
	ownerGender := c.identityGender(ctx, tx, ownerMem.IdentityID)
	val := 0
	eventType := "member.settings_revoked"
	summary := summarySettingsRevoked(ownerGender, ownerName, targetName)
	if canSettings {
		val = 1
		eventType = "member.settings_granted"
		summary = summarySettingsGranted(ownerGender, ownerName, targetName)
	}
	updated := formatTime(now)
	res, err := tx.ExecContext(ctx, `
		UPDATE memberships SET can_settings = ?, updated_at = ? WHERE circle_id = ? AND account_id = ?
	`, val, updated, circleID, targetAccountID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	if _, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID:        circleID,
		eventType:       eventType,
		isService:       true,
		actorIdentityID: ownerMem.IdentityID,
		actorName:       ownerName,
		targetID:        mem.IdentityID,
		summary:         summary,
		now:             now,
	}); err != nil {
		return err
	}
	return tx.Commit()
}
