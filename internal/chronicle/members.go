package chronicle

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// MemberRow is a circle participant for settings UI.
type MemberRow struct {
	AccountID   string           `json:"account_id"`
	IdentityID  string           `json:"identity_id"`
	Name        string           `json:"name"`
	Status      MembershipStatus `json:"status"`
	CanSettings bool             `json:"can_settings"`
	IsOwner     bool             `json:"is_owner"`
	JoinedAt    time.Time        `json:"joined_at"`
	CanRead     bool             `json:"can_read"`
	CanWrite    bool             `json:"can_write"`
}

// MembershipForAccount returns the caller's membership row.
func (c *Chronicle) MembershipForAccount(ctx context.Context, circleID, accountID string) (Membership, error) {
	return c.membership(ctx, c.db, circleID, accountID)
}

// ListMembers returns all memberships for a circle (active and left).
func (c *Chronicle) ListMembers(ctx context.Context, circleID, actorAccountID string) ([]MemberRow, error) {
	if err := c.requireReader(ctx, circleID, actorAccountID); err != nil {
		return nil, err
	}
	owner, err := c.circleOwner(ctx, c.db, circleID)
	if err != nil {
		return nil, err
	}
	rows, err := c.db.QueryContext(ctx, `
		SELECT m.account_id, m.identity_id, m.can_settings, m.status, m.created_at
		FROM memberships m
		WHERE m.circle_id = ?
		ORDER BY m.created_at
	`, circleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MemberRow
	for rows.Next() {
		var row MemberRow
		var status string
		var created string
		if err := rows.Scan(&row.AccountID, &row.IdentityID, &row.CanSettings, &status, &created); err != nil {
			return nil, err
		}
		row.Status = MembershipStatus(status)
		row.IsOwner = row.AccountID == owner
		row.JoinedAt, err = parseTime(created)
		if err != nil {
			return nil, err
		}
		row.Name, err = c.identityName(ctx, c.db, row.IdentityID)
		if err != nil {
			return nil, err
		}
		row.CanRead, row.CanWrite, err = c.memberAccessNow(ctx, circleID, row.AccountID)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (c *Chronicle) memberAccessNow(ctx context.Context, circleID, accountID string) (canRead, canWrite bool, err error) {
	if accountID == "" {
		return false, false, nil
	}
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return false, false, err
	}
	if mem.Status == StatusGone {
		return false, false, nil
	}
	spans, err := c.openSpans(ctx, mem.ID)
	if err != nil {
		return false, false, err
	}
	for _, sp := range spans {
		if sp.CanRead {
			canRead = true
		}
		if sp.CanWrite {
			canWrite = true
		}
	}
	return canRead, canWrite, nil
}

// IdentityNameHistory is a past name for an identity.
type IdentityNameHistory struct {
	Name        string    `json:"name"`
	EffectiveAt time.Time `json:"effective_at"`
}

// ListIdentityNames returns name history newest-first (excluding erased).
func (c *Chronicle) ListIdentityNames(ctx context.Context, identityID string) ([]IdentityNameHistory, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT name, effective_at FROM identity_names
		WHERE identity_id = ? AND erased_at IS NULL
		ORDER BY effective_at DESC
	`, identityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IdentityNameHistory
	for rows.Next() {
		var row IdentityNameHistory
		var effective string
		if err := rows.Scan(&row.Name, &effective); err != nil {
			return nil, err
		}
		row.EffectiveAt, err = parseTime(effective)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// RenameIdentity sets a new display name for the actor in a circle.
func (c *Chronicle) RenameIdentity(ctx context.Context, circleID, accountID, name string, now time.Time) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrInvalid
	}
	if err := checkLen(name, MaxNameChars); err != nil {
		return err
	}
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return err
	}
	if mem.Status != StatusActive {
		return ErrForbidden
	}
	oldName, err := c.identityName(ctx, c.db, mem.IdentityID)
	if err != nil {
		return err
	}
	if oldName == name {
		return nil
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := c.addIdentityName(ctx, tx, mem.IdentityID, name, now); err != nil {
		return err
	}
	if _, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID:        circleID,
		eventType:       "identity.renamed",
		isService:       true,
		actorIdentityID: mem.IdentityID,
		actorName:       name,
		payload:         map[string]any{"from": oldName, "to": name},
		summary:         summaryIdentityRenamed(oldName, name),
		now:             now,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// SetCircleName changes the circle title.
func (c *Chronicle) SetCircleName(ctx context.Context, circleID, actorAccountID, name string, now time.Time) error {
	if err := c.RequireSettings(ctx, circleID, actorAccountID); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrInvalid
	}
	if err := checkLen(name, MaxNameChars); err != nil {
		return err
	}
	mem, err := c.membership(ctx, c.db, circleID, actorAccountID)
	if err != nil {
		return err
	}
	actorName, err := c.identityName(ctx, c.db, mem.IdentityID)
	if err != nil {
		return err
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	updated := formatTime(now)
	if _, err := tx.ExecContext(ctx, `UPDATE circles SET name = ?, updated_at = ? WHERE id = ?`,
		name, updated, circleID); err != nil {
		return fmt.Errorf("update circle name: %w", err)
	}
	if _, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID:        circleID,
		eventType:       "circle.renamed",
		isService:       true,
		actorIdentityID: mem.IdentityID,
		actorName:       actorName,
		payload:         map[string]any{"name": name},
		summary:         summaryCircleRenamed(name),
		now:             now,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// SetCircleColor changes the circle accent color (no service event).
func (c *Chronicle) SetCircleColor(ctx context.Context, circleID, actorAccountID, color string, now time.Time) error {
	if err := c.RequireSettings(ctx, circleID, actorAccountID); err != nil {
		return err
	}
	normalized, err := normalizeCircleColor(color)
	if err != nil {
		return err
	}
	updated := formatTime(now)
	res, err := c.db.ExecContext(ctx, `UPDATE circles SET color = ?, updated_at = ? WHERE id = ?`,
		normalized, updated, circleID)
	if err != nil {
		return fmt.Errorf("update circle color: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CircleColor returns the circle accent color token.
func (c *Chronicle) CircleColor(ctx context.Context, circleID string) (string, error) {
	var color string
	err := c.db.QueryRowContext(ctx, `SELECT color FROM circles WHERE id = ?`, circleID).Scan(&color)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return color, nil
}

// DeleteCircle removes a circle and all its data. Owner only; name must match.
func (c *Chronicle) DeleteCircle(ctx context.Context, circleID, ownerAccountID, confirmName string, now time.Time) error {
	if err := c.RequireOwner(ctx, circleID, ownerAccountID); err != nil {
		return err
	}
	var name string
	err := c.db.QueryRowContext(ctx, `SELECT name FROM circles WHERE id = ?`, circleID).Scan(&name)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if strings.TrimSpace(confirmName) != name {
		return ErrInvalid
	}
	_, err = c.db.ExecContext(ctx, `DELETE FROM circles WHERE id = ?`, circleID)
	return err
}

// PostAuthorAccountID resolves the post author's account in a circle.
func (c *Chronicle) PostAuthorAccountID(ctx context.Context, circleID, postID string) (string, error) {
	post, err := c.loadPost(ctx, c.db, postID)
	if err != nil {
		return "", err
	}
	if post.CircleID != circleID || post.Deleted {
		return "", ErrInvalid
	}
	var accountID string
	err = c.db.QueryRowContext(ctx, `
		SELECT account_id FROM memberships
		WHERE circle_id = ? AND identity_id = ? AND status = 'active'
	`, circleID, post.IdentityID).Scan(&accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return accountID, nil
}
