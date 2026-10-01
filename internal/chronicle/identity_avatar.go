package chronicle

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// ResolveIdentityAvatar returns current avatar blob id or empty string.
func (c *Chronicle) ResolveIdentityAvatar(ctx context.Context, identityID string) (string, error) {
	var blobID sql.NullString
	err := c.db.QueryRowContext(ctx, `
		SELECT avatar_blob_id FROM identity_names
		WHERE identity_id = ? AND erased_at IS NULL
		ORDER BY effective_at DESC LIMIT 1
	`, identityID).Scan(&blobID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if blobID.Valid {
		return blobID.String, nil
	}
	return "", nil
}

type UpdateIdentityInput struct {
	Name           *string
	AvatarBlobID   *string // nil = omit; pointer to "" = clear
	// Gender — род для строк журнала (A5); nil = не трогать, "" = не выбран.
	Gender *Gender
}

// UpdateIdentity changes display name and/or avatar for the actor in a circle.
func (c *Chronicle) UpdateIdentity(ctx context.Context, circleID, accountID string, in UpdateIdentityInput, now time.Time) error {
	if in.Name == nil && in.AvatarBlobID == nil && in.Gender == nil {
		return ErrInvalid
	}
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return err
	}
	if mem.Status != StatusActive {
		return ErrForbidden
	}
	if in.Name != nil {
		if err := c.RenameIdentity(ctx, circleID, accountID, *in.Name, now); err != nil {
			return err
		}
	}
	if in.Gender != nil {
		if err := c.setIdentityGender(ctx, c.db, mem.IdentityID, *in.Gender); err != nil {
			return err
		}
	}
	if in.AvatarBlobID == nil {
		return nil
	}
	return c.setIdentityAvatar(ctx, circleID, mem.IdentityID, *in.AvatarBlobID, now)
}

// avatarJoinGrace — первое фото в эти минуты после входа — часть вступления
// (1.3 ставит фото сразу за входом), строки в ленте оно не даёт.
const avatarJoinGrace = 10 * time.Minute

// setIdentityAvatar меняет фото и пишет строку в ленту (3.1): «Новое фото:
// Аня», «Фото убрано: Аня». Без глагола — род человека система не знает.
// То же фото и первое фото при вступлении строки не дают.
func (c *Chronicle) setIdentityAvatar(ctx context.Context, circleID, identityID, blobID string, now time.Time) error {
	blobID = strings.TrimSpace(blobID)
	var arg any
	if blobID == "" {
		arg = nil
	} else {
		arg = blobID
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var nameRowID, name string
	var prev sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT id, name, avatar_blob_id FROM identity_names
		WHERE identity_id = ? AND erased_at IS NULL
		ORDER BY effective_at DESC LIMIT 1
	`, identityID).Scan(&nameRowID, &name, &prev)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if prev.String == blobID {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE identity_names SET avatar_blob_id = ? WHERE id = ?`, arg, nameRowID); err != nil {
		return err
	}
	quiet := false
	if prev.String == "" && blobID != "" {
		var created string
		if err := tx.QueryRowContext(ctx, `SELECT created_at FROM identities WHERE id = ?`, identityID).Scan(&created); err != nil {
			return err
		}
		if joined, err := parseTime(created); err == nil && now.Sub(joined) < avatarJoinGrace {
			quiet = true
		}
	}
	if !quiet {
		eventType, summary := "identity.avatar_set", summaryAvatarSet(name)
		if blobID == "" {
			eventType, summary = "identity.avatar_cleared", summaryAvatarCleared(name)
		}
		if _, err := c.appendEvent(ctx, tx, appendEventInput{
			circleID:        circleID,
			eventType:       eventType,
			isService:       true,
			actorIdentityID: identityID,
			actorName:       name,
			summary:         summary,
			now:             now,
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}
