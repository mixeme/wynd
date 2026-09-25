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
}

// UpdateIdentity changes display name and/or avatar for the actor in a circle.
func (c *Chronicle) UpdateIdentity(ctx context.Context, circleID, accountID string, in UpdateIdentityInput, now time.Time) error {
	if in.Name == nil && in.AvatarBlobID == nil {
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
	if in.AvatarBlobID == nil {
		return nil
	}
	return c.setIdentityAvatar(ctx, mem.IdentityID, *in.AvatarBlobID, now)
}

func (c *Chronicle) setIdentityAvatar(ctx context.Context, identityID, blobID string, now time.Time) error {
	blobID = strings.TrimSpace(blobID)
	var arg any
	if blobID == "" {
		arg = nil
	} else {
		arg = blobID
	}
	var nameRowID string
	err := c.db.QueryRowContext(ctx, `
		SELECT id FROM identity_names
		WHERE identity_id = ? AND erased_at IS NULL
		ORDER BY effective_at DESC LIMIT 1
	`, identityID).Scan(&nameRowID)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	_, err = c.db.ExecContext(ctx, `UPDATE identity_names SET avatar_blob_id = ? WHERE id = ?`, arg, nameRowID)
	return err
}
