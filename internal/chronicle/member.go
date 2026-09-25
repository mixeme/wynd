package chronicle

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type dbtx interface {
	querier
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Join adds a member with a fresh visibility span starting at now.
func (c *Chronicle) Join(ctx context.Context, in JoinInput) (Membership, Identity, error) {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return Membership{}, Identity{}, err
	}
	defer func() { _ = tx.Rollback() }()
	mem, ident, err := c.JoinInTx(ctx, tx, in)
	if err != nil {
		return Membership{}, Identity{}, err
	}
	if err := tx.Commit(); err != nil {
		return Membership{}, Identity{}, err
	}
	return mem, ident, nil
}

// JoinInTx joins inside an existing transaction (auth invite verify).
func (c *Chronicle) JoinInTx(ctx context.Context, tx *sql.Tx, in JoinInput) (Membership, Identity, error) {
	if in.CircleID == "" || in.AccountID == "" || in.Name == "" {
		return Membership{}, Identity{}, ErrInvalid
	}
	if err := checkLen(in.Name, MaxNameChars); err != nil {
		return Membership{}, Identity{}, err
	}
	now := utcOrNow(in.Now)

	existing, err := c.membership(ctx, tx, in.CircleID, in.AccountID)
	if err == nil {
		if existing.Status == StatusActive {
			return Membership{}, Identity{}, ErrInvalid
		}
		return c.rejoinTx(ctx, tx, existing, in.Name, now)
	}
	if err != ErrNotFound {
		return Membership{}, Identity{}, err
	}

	identityID, err := newID()
	if err != nil {
		return Membership{}, Identity{}, err
	}
	nameID, err := newID()
	if err != nil {
		return Membership{}, Identity{}, err
	}
	membershipID, err := newID()
	if err != nil {
		return Membership{}, Identity{}, err
	}
	spanID, err := newID()
	if err != nil {
		return Membership{}, Identity{}, err
	}

	created := formatTime(now)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO identities (id, circle_id, account_id, created_at)
		VALUES (?, ?, ?, ?)
	`, identityID, in.CircleID, in.AccountID, created); err != nil {
		return Membership{}, Identity{}, fmt.Errorf("insert identity: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO identity_names (id, identity_id, name, effective_at)
		VALUES (?, ?, ?, ?)
	`, nameID, identityID, in.Name, created); err != nil {
		return Membership{}, Identity{}, fmt.Errorf("insert identity name: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO memberships (id, circle_id, account_id, identity_id, can_settings, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, ?, ?, ?)
	`, membershipID, in.CircleID, in.AccountID, identityID, StatusActive, created, created); err != nil {
		return Membership{}, Identity{}, fmt.Errorf("insert membership: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO membership_spans (id, membership_id, started_at, can_read, can_write)
		VALUES (?, ?, ?, 1, 1)
	`, spanID, membershipID, created); err != nil {
		return Membership{}, Identity{}, fmt.Errorf("insert span: %w", err)
	}

	if _, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID:        in.CircleID,
		eventType:       "member.joined",
		isService:       true,
		actorIdentityID: identityID,
		actorName:       in.Name,
		summary:         summaryMemberJoined(in.Name),
		now:             now,
	}); err != nil {
		return Membership{}, Identity{}, err
	}

	membership := Membership{
		ID:         membershipID,
		CircleID:   in.CircleID,
		AccountID:  in.AccountID,
		IdentityID: identityID,
		Status:     StatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	identity := Identity{
		ID:        identityID,
		CircleID:  in.CircleID,
		AccountID: in.AccountID,
		Name:      in.Name,
		CreatedAt: now,
	}
	return membership, identity, nil
}

func (c *Chronicle) rejoinTx(ctx context.Context, tx *sql.Tx, existing Membership, name string, now time.Time) (Membership, Identity, error) {
	spanID, err := newID()
	if err != nil {
		return Membership{}, Identity{}, err
	}

	updated := formatTime(now)
	if _, err := tx.ExecContext(ctx, `
		UPDATE memberships SET status = ?, updated_at = ? WHERE id = ?
	`, StatusActive, updated, existing.ID); err != nil {
		return Membership{}, Identity{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO membership_spans (id, membership_id, started_at, can_read, can_write)
		VALUES (?, ?, ?, 1, 1)
	`, spanID, existing.ID, updated); err != nil {
		return Membership{}, Identity{}, err
	}

	if name != "" {
		if err := c.addIdentityName(ctx, tx, existing.IdentityID, name, now); err != nil {
			return Membership{}, Identity{}, err
		}
	}
	displayName, err := c.identityName(ctx, tx, existing.IdentityID)
	if err != nil {
		return Membership{}, Identity{}, err
	}

	if _, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID:        existing.CircleID,
		eventType:       "member.joined",
		isService:       true,
		actorIdentityID: existing.IdentityID,
		actorName:       displayName,
		summary:         summaryMemberJoined(displayName),
		now:             now,
	}); err != nil {
		return Membership{}, Identity{}, err
	}

	existing.Status = StatusActive
	existing.UpdatedAt = now
	identity := Identity{
		ID:        existing.IdentityID,
		CircleID:  existing.CircleID,
		AccountID: existing.AccountID,
		Name:      displayName,
	}
	return existing, identity, nil
}

// LeaveWithAccess closes the active span with read access until now.
func (c *Chronicle) LeaveWithAccess(ctx context.Context, circleID, accountID string, now time.Time) error {
	return c.leave(ctx, circleID, accountID, StatusLeftWithAccess, true, now,
		func(ctx context.Context, tx *sql.Tx) error {
			return c.forbidOwnerLeaveTx(ctx, tx, circleID, accountID)
		})
}

// Leave revokes all access immediately.
func (c *Chronicle) Leave(ctx context.Context, circleID, accountID string, now time.Time) error {
	return c.leave(ctx, circleID, accountID, StatusGone, false, now,
		func(ctx context.Context, tx *sql.Tx) error {
			return c.forbidOwnerLeaveTx(ctx, tx, circleID, accountID)
		})
}

// LeaveInTx revokes all access inside an existing transaction (admin account delete).
func (c *Chronicle) LeaveInTx(ctx context.Context, tx *sql.Tx, circleID, accountID string, now time.Time) error {
	return c.leaveInTx(ctx, tx, circleID, accountID, StatusGone, false, now)
}

// forbidOwnerLeaveTx — владелец не уходит из своего круга. Проверка читает
// владельца в той же транзакции, что и запись (QLT-1).
func (c *Chronicle) forbidOwnerLeaveTx(ctx context.Context, tx *sql.Tx, circleID, accountID string) error {
	owner, err := c.circleOwner(ctx, tx, circleID)
	if err != nil {
		return err
	}
	if owner == accountID {
		return ErrForbidden
	}
	return nil
}

// Exclude removes a member without read access.
func (c *Chronicle) Exclude(ctx context.Context, circleID, actorAccountID, targetAccountID string, now time.Time) error {
	return c.leave(ctx, circleID, targetAccountID, StatusGone, false, now, func(ctx context.Context, tx *sql.Tx) error {
		owner, err := c.circleOwner(ctx, tx, circleID)
		if err != nil {
			return err
		}
		if actorAccountID != owner || targetAccountID == owner {
			return ErrForbidden
		}
		return nil
	})
}

// leave закрывает членство. precheck, если задан, выполняется внутри той же
// транзакции — до любой записи.
func (c *Chronicle) leave(ctx context.Context, circleID, accountID string, status MembershipStatus, retainRead bool, now time.Time, precheck func(context.Context, *sql.Tx) error) error {
	tx, err := c.beginWrite(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if precheck != nil {
		if err := precheck(ctx, tx); err != nil {
			return err
		}
	}
	if err := c.leaveInTx(ctx, tx, circleID, accountID, status, retainRead, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (c *Chronicle) leaveInTx(ctx context.Context, tx *sql.Tx, circleID, accountID string, status MembershipStatus, retainRead bool, now time.Time) error {
	mem, err := c.membership(ctx, tx, circleID, accountID)
	if err != nil {
		return err
	}

	updated := formatTime(now)
	switch {
	case mem.Status == StatusActive:
		if err := c.closeOpenSpan(ctx, tx, mem.ID, now, retainRead); err != nil {
			return err
		}
	case mem.Status == StatusLeftWithAccess && status == StatusGone:
		// Вышедший с доступом свой отрезок уже закрыл, поэтому закрывать
		// нечего — исключение отзывает чтение у всех его отрезков. Без этой
		// ветки владелец не мог отозвать доступ иначе как удалив круг (CHR-1).
		if _, err := tx.ExecContext(ctx, `
			UPDATE membership_spans SET can_read = 0 WHERE membership_id = ?
		`, mem.ID); err != nil {
			return err
		}
	default:
		return ErrInvalid
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE memberships SET status = ?, updated_at = ? WHERE id = ?
	`, status, updated, mem.ID); err != nil {
		return err
	}

	name, err := c.identityName(ctx, tx, mem.IdentityID)
	if err != nil {
		return err
	}
	eventType := "member.left"
	if status == StatusLeftWithAccess {
		eventType = "member.left_with_access"
	}
	if _, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID:        circleID,
		eventType:       eventType,
		isService:       true,
		actorIdentityID: mem.IdentityID,
		actorName:       name,
		summary:         summaryMemberLeft(name),
		now:             now,
	}); err != nil {
		return err
	}
	return nil
}

func (c *Chronicle) closeOpenSpan(ctx context.Context, tx dbtx, membershipID string, end time.Time, retainRead bool) error {
	ended := formatTime(end)
	canRead := 0
	if retainRead {
		canRead = 1
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE membership_spans
		SET ended_at = ?, can_write = 0, can_read = ?
		WHERE membership_id = ? AND ended_at IS NULL
	`, ended, canRead, membershipID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrInvalid
	}
	return nil
}

func (c *Chronicle) addIdentityName(ctx context.Context, tx dbtx, identityID, name string, now time.Time) error {
	nameID, err := newID()
	if err != nil {
		return err
	}
	var avatarBlobID sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT avatar_blob_id FROM identity_names
		WHERE identity_id = ? AND erased_at IS NULL
		ORDER BY effective_at DESC LIMIT 1
	`, identityID).Scan(&avatarBlobID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	var avatarArg any
	if avatarBlobID.Valid && avatarBlobID.String != "" {
		avatarArg = avatarBlobID.String
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO identity_names (id, identity_id, name, avatar_blob_id, effective_at)
		VALUES (?, ?, ?, ?, ?)
	`, nameID, identityID, name, avatarArg, formatTime(now))
	return err
}

func (c *Chronicle) identityName(ctx context.Context, q querier, identityID string) (string, error) {
	var name string
	err := q.QueryRowContext(ctx, `
		SELECT name FROM identity_names
		WHERE identity_id = ? AND erased_at IS NULL
		ORDER BY effective_at DESC LIMIT 1
	`, identityID).Scan(&name)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return name, err
}
