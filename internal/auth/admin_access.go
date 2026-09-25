package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// AccountSummary is a row in the admin account list (no identity names).
type AccountSummary struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	CircleCount int    `json:"circle_count"`
	CreatedAt   string `json:"created_at"`
	Blocked     bool   `json:"blocked"`
}

// InviteSummary is a server invite for the admin list.
type InviteSummary struct {
	ID        string  `json:"id"`
	Token     string  `json:"token"`
	Kind      string  `json:"kind"`
	MaxUses   int     `json:"max_uses"`
	Uses      int     `json:"uses"`
	ExpiresAt string  `json:"expires_at"`
	RevokedAt *string `json:"revoked_at,omitempty"`
	CreatedAt string  `json:"created_at"`
}

// SetInstanceName updates the public instance name.
func (s *Service) SetInstanceName(ctx context.Context, name string) error {
	if name == "" {
		return ErrInvalid
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE instance_settings SET name = ?, updated_at = ? WHERE id = 1
	`, name, formatTime(time.Now().UTC()))
	return err
}

// ListAccounts returns participant accounts with circle counts (no identity names).
func (s *Service) ListAccounts(ctx context.Context) ([]AccountSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.email, a.created_at, a.blocked,
			(SELECT COUNT(DISTINCT m.circle_id) FROM memberships m WHERE m.account_id = a.id AND m.status = 'active')
		FROM accounts a
		WHERE a.email != ? AND a.deleted_at IS NULL
		ORDER BY a.created_at DESC
	`, AdminSentinelEmail)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	var out []AccountSummary
	for rows.Next() {
		var row AccountSummary
		var blocked int
		if err := rows.Scan(&row.ID, &row.Email, &row.CreatedAt, &blocked, &row.CircleCount); err != nil {
			return nil, err
		}
		row.Blocked = blocked != 0
		out = append(out, row)
	}
	return out, rows.Err()
}

// SetAccountBlocked closes or reopens login. Circles are left untouched.
// Participant sessions are dropped on block so an open app cannot keep writing.
func (s *Service) SetAccountBlocked(ctx context.Context, id string, blocked bool) error {
	if id == "" {
		return ErrInvalid
	}
	acc, err := s.AccountByID(ctx, id)
	if err != nil {
		return err
	}
	if acc.Email == AdminSentinelEmail {
		return ErrNotFound
	}
	val := 0
	if blocked {
		val = 1
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE accounts SET blocked = ? WHERE id = ?
	`, val, id); err != nil {
		return fmt.Errorf("set blocked: %w", err)
	}
	if !blocked {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `
		DELETE FROM sessions WHERE account_id = ? AND kind = ?
	`, id, SessionParticipant); err != nil {
		return fmt.Errorf("revoke blocked sessions: %w", err)
	}
	return nil
}

// ListServerInvites returns invites without circle_id (server-only).
func (s *Service) ListServerInvites(ctx context.Context) ([]InviteSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, token, kind, max_uses, uses, expires_at, revoked_at, created_at
		FROM invites
		WHERE circle_id IS NULL
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list server invites: %w", err)
	}
	defer rows.Close()

	var out []InviteSummary
	for rows.Next() {
		var row InviteSummary
		var revoked sql.NullString
		if err := rows.Scan(&row.ID, &row.Token, &row.Kind, &row.MaxUses, &row.Uses,
			&row.ExpiresAt, &revoked, &row.CreatedAt); err != nil {
			return nil, err
		}
		if revoked.Valid {
			row.RevokedAt = &revoked.String
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ListCircleInvites returns live invites for a circle.
func (s *Service) ListCircleInvites(ctx context.Context, circleID string, now time.Time) ([]InviteSummary, error) {
	if circleID == "" {
		return nil, ErrInvalid
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, token, kind, max_uses, uses, expires_at, created_at
		FROM invites
		WHERE circle_id = ?
			AND revoked_at IS NULL
			AND uses < max_uses
			AND expires_at > ?
		ORDER BY created_at DESC
	`, circleID, formatTime(now.UTC()))
	if err != nil {
		return nil, fmt.Errorf("list circle invites: %w", err)
	}
	defer rows.Close()

	var out []InviteSummary
	for rows.Next() {
		var row InviteSummary
		if err := rows.Scan(&row.ID, &row.Token, &row.Kind, &row.MaxUses, &row.Uses,
			&row.ExpiresAt, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// RevokeCircleInvite revokes an invite that belongs to the given circle.
func (s *Service) RevokeCircleInvite(ctx context.Context, circleID, inviteID string) error {
	if circleID == "" || inviteID == "" {
		return ErrInvalid
	}
	inv, err := s.inviteByID(ctx, inviteID)
	if err != nil {
		return err
	}
	if inv.CircleID != circleID {
		return ErrNotFound
	}
	return s.RevokeInvite(ctx, inviteID)
}

// RevokeInvite marks an invite as revoked.
func (s *Service) RevokeInvite(ctx context.Context, id string) error {
	if id == "" {
		return ErrInvalid
	}
	now := formatTime(time.Now().UTC())
	res, err := s.db.ExecContext(ctx, `
		UPDATE invites SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL
	`, now, id)
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
	return nil
}
