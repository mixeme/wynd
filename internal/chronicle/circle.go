package chronicle

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// CreateCircle creates a circle with the owner as first member.
func (c *Chronicle) CreateCircle(ctx context.Context, in CreateCircleInput) (Circle, Identity, Membership, error) {
	if in.Name == "" || in.OwnerAccountID == "" || in.OwnerName == "" {
		return Circle{}, Identity{}, Membership{}, ErrInvalid
	}
	if checkLen(in.Name, MaxNameChars) != nil || checkLen(in.OwnerName, MaxNameChars) != nil {
		return Circle{}, Identity{}, Membership{}, ErrTooLong
	}
	now := in.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	circleID, err := newID()
	if err != nil {
		return Circle{}, Identity{}, Membership{}, err
	}
	identityID, err := newID()
	if err != nil {
		return Circle{}, Identity{}, Membership{}, err
	}
	nameID, err := newID()
	if err != nil {
		return Circle{}, Identity{}, Membership{}, err
	}
	membershipID, err := newID()
	if err != nil {
		return Circle{}, Identity{}, Membership{}, err
	}
	spanID, err := newID()
	if err != nil {
		return Circle{}, Identity{}, Membership{}, err
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return Circle{}, Identity{}, Membership{}, err
	}
	defer func() { _ = tx.Rollback() }()

	color, err := normalizeCircleColor(in.Color)
	if err != nil {
		return Circle{}, Identity{}, Membership{}, err
	}

	created := formatTime(now)
	ew := editWindowToSQL(in.EditWindow)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO circles (id, name, owner_account_id, color, edit_window_sec, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, circleID, in.Name, in.OwnerAccountID, color, ew, created, created); err != nil {
		return Circle{}, Identity{}, Membership{}, fmt.Errorf("insert circle: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO identities (id, circle_id, account_id, created_at)
		VALUES (?, ?, ?, ?)
	`, identityID, circleID, in.OwnerAccountID, created); err != nil {
		return Circle{}, Identity{}, Membership{}, fmt.Errorf("insert identity: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO identity_names (id, identity_id, name, effective_at)
		VALUES (?, ?, ?, ?)
	`, nameID, identityID, in.OwnerName, created); err != nil {
		return Circle{}, Identity{}, Membership{}, fmt.Errorf("insert identity name: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO memberships (id, circle_id, account_id, identity_id, can_settings, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, ?, ?, ?)
	`, membershipID, circleID, in.OwnerAccountID, identityID, StatusActive, created, created); err != nil {
		return Circle{}, Identity{}, Membership{}, fmt.Errorf("insert membership: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO membership_spans (id, membership_id, started_at, can_read, can_write)
		VALUES (?, ?, ?, 1, 1)
	`, spanID, membershipID, created); err != nil {
		return Circle{}, Identity{}, Membership{}, fmt.Errorf("insert span: %w", err)
	}

	if _, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID:        circleID,
		eventType:       "circle.created",
		isService:       true,
		actorIdentityID: identityID,
		actorName:       in.OwnerName,
		summary:         summaryCircleCreated(in.Name),
		now:             now,
	}); err != nil {
		return Circle{}, Identity{}, Membership{}, err
	}

	if err := tx.Commit(); err != nil {
		return Circle{}, Identity{}, Membership{}, err
	}

	circle := Circle{
		ID:             circleID,
		Name:           in.Name,
		OwnerAccountID: in.OwnerAccountID,
		Color:          color,
		EditWindow:     in.EditWindow,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	identity := Identity{
		ID:        identityID,
		CircleID:  circleID,
		AccountID: in.OwnerAccountID,
		Name:      in.OwnerName,
		CreatedAt: now,
	}
	membership := Membership{
		ID:          membershipID,
		CircleID:    circleID,
		AccountID:   in.OwnerAccountID,
		IdentityID:  identityID,
		CanSettings: true,
		Status:      StatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return circle, identity, membership, nil
}

// SetEditWindow changes the circle default edit window (service event).
func (c *Chronicle) SetEditWindow(ctx context.Context, circleID, actorAccountID string, window EditWindow, now time.Time) error {
	mem, err := c.membership(ctx, c.db, circleID, actorAccountID)
	if err != nil {
		return err
	}
	if mem.Status != StatusActive {
		return ErrForbidden
	}
	if !mem.CanSettings {
		owner, err := c.circleOwner(ctx, c.db, circleID)
		if err != nil {
			return err
		}
		if mem.AccountID != owner {
			return ErrForbidden
		}
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	ew := editWindowToSQL(window)
	if _, err := tx.ExecContext(ctx, `
		UPDATE circles SET edit_window_sec = ?, updated_at = ? WHERE id = ?
	`, ew, formatTime(now), circleID); err != nil {
		return fmt.Errorf("update circle edit window: %w", err)
	}

	name, err := c.identityName(ctx, tx, mem.IdentityID)
	if err != nil {
		return err
	}
	if _, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID:        circleID,
		eventType:       "circle.edit_window_changed",
		isService:       true,
		actorIdentityID: mem.IdentityID,
		actorName:       name,
		payload:         map[string]any{"edit_window_sec": window.Seconds},
		summary:         summaryEditWindowChanged(formatEditWindowLabel(window)),
		now:             now,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// TransferOwnership moves circle ownership; caller must be current owner.
func (c *Chronicle) TransferOwnership(ctx context.Context, circleID, ownerAccountID, newOwnerAccountID string, now time.Time) error {
	currentOwner, err := c.circleOwner(ctx, c.db, circleID)
	if err != nil {
		return err
	}
	if currentOwner != ownerAccountID {
		return ErrForbidden
	}
	if newOwnerAccountID == ownerAccountID {
		return ErrInvalid
	}
	newMem, err := c.membership(ctx, c.db, circleID, newOwnerAccountID)
	if err != nil {
		return err
	}
	if newMem.Status != StatusActive {
		return ErrForbidden
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	oldMem, err := c.membership(ctx, tx, circleID, ownerAccountID)
	if err != nil {
		return err
	}
	oldName, err := c.identityName(ctx, tx, oldMem.IdentityID)
	if err != nil {
		return err
	}
	newName, err := c.identityName(ctx, tx, newMem.IdentityID)
	if err != nil {
		return err
	}

	updated := formatTime(now)
	if _, err := tx.ExecContext(ctx, `UPDATE circles SET owner_account_id = ?, updated_at = ? WHERE id = ?`,
		newOwnerAccountID, updated, circleID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE memberships SET can_settings = 0, updated_at = ? WHERE circle_id = ? AND account_id = ?`,
		updated, circleID, ownerAccountID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE memberships SET can_settings = 1, updated_at = ? WHERE circle_id = ? AND account_id = ?`,
		updated, circleID, newOwnerAccountID); err != nil {
		return err
	}

	if _, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID:        circleID,
		eventType:       "owner.transferred",
		isService:       true,
		actorIdentityID: oldMem.IdentityID,
		actorName:       oldName,
		targetID:        newMem.IdentityID,
		summary:         summaryOwnerTransferred(oldName, newName),
		now:             now,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// CircleOwnerAccount returns the owner account id for a circle.
func (c *Chronicle) CircleOwnerAccount(ctx context.Context, circleID string) (string, error) {
	return c.circleOwner(ctx, c.db, circleID)
}

// CircleEditWindow returns the circle default edit window.
func (c *Chronicle) CircleEditWindow(ctx context.Context, circleID string) (EditWindow, error) {
	return c.circleEditWindow(ctx, c.db, circleID)
}

func (c *Chronicle) circleOwner(ctx context.Context, q querier, circleID string) (string, error) {
	var owner string
	err := q.QueryRowContext(ctx, `SELECT owner_account_id FROM circles WHERE id = ?`, circleID).Scan(&owner)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return owner, err
}

// RequireOwner returns ErrForbidden unless accountID owns the circle.
func (c *Chronicle) RequireOwner(ctx context.Context, circleID, accountID string) error {
	owner, err := c.circleOwner(ctx, c.db, circleID)
	if err != nil {
		return err
	}
	if owner != accountID {
		return ErrForbidden
	}
	return nil
}

// RequireSettings returns ErrForbidden unless the account is an active member
// with can_settings (the owner always has it).
func (c *Chronicle) RequireSettings(ctx context.Context, circleID, accountID string) error {
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return err
	}
	if mem.Status != StatusActive || !mem.CanSettings {
		return ErrForbidden
	}
	return nil
}

func (c *Chronicle) circleEditWindow(ctx context.Context, q querier, circleID string) (EditWindow, error) {
	var ew sql.NullInt64
	err := q.QueryRowContext(ctx, `SELECT edit_window_sec FROM circles WHERE id = ?`, circleID).Scan(&ew)
	if err == sql.ErrNoRows {
		return EditWindow{}, ErrNotFound
	}
	if err != nil {
		return EditWindow{}, err
	}
	return editWindowFromSQL(ew)
}

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
