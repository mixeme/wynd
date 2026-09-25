package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const memberInviteTTL = 30 * 24 * time.Hour

// CreateMemberInvite issues a one-time circle invite bound to an existing account.
func (s *Service) CreateMemberInvite(ctx context.Context, in CreateMemberInviteInput) (Invite, error) {
	if in.CircleID == "" || in.TargetAccountID == "" || in.CreatedByAccountID == "" {
		return Invite{}, ErrInvalid
	}
	if in.TargetAccountID == in.CreatedByAccountID {
		return Invite{}, ErrInvalid
	}
	when := in.Now.UTC()
	if when.IsZero() {
		when = time.Now().UTC()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Invite{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.revokeLiveMemberInvites(ctx, tx, in.CircleID, in.TargetAccountID, when); err != nil {
		return Invite{}, err
	}

	inv, err := s.createInviteInTx(ctx, tx, CreateInviteInput{
		CircleID:           in.CircleID,
		Kind:               InviteSingle,
		MaxUses:            1,
		TTL:                memberInviteTTL,
		CreatedByAccountID: in.CreatedByAccountID,
		TargetAccountID:    in.TargetAccountID,
		Now:                when,
	})
	if err != nil {
		return Invite{}, err
	}
	// Личное приглашение: право вступить живёт ровно столько, сколько сама
	// ссылка, и снимается вместе с ней (SEC-9).
	if err := s.insertPendingCircleJoin(ctx, tx, in.TargetAccountID, in.CircleID, inv.ID, when, inv.ExpiresAt); err != nil {
		return Invite{}, err
	}
	if err := tx.Commit(); err != nil {
		return Invite{}, err
	}
	return inv, nil
}

func (s *Service) revokeLiveMemberInvites(ctx context.Context, tx *sql.Tx, circleID, targetAccountID string, when time.Time) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE invites SET revoked_at = ?
		WHERE circle_id = ? AND target_account_id = ? AND revoked_at IS NULL
		  AND uses < max_uses AND expires_at > ?
	`, formatTime(when), circleID, targetAccountID, formatTime(when))
	if err != nil {
		return fmt.Errorf("revoke member invites: %w", err)
	}
	return nil
}

func (s *Service) createInviteInTx(ctx context.Context, tx *sql.Tx, in CreateInviteInput) (Invite, error) {
	if in.MaxUses < 1 {
		return Invite{}, ErrInvalid
	}
	if in.Kind == InviteSingle {
		in.MaxUses = 1
	}
	if in.TTL <= 0 {
		in.TTL = 7 * 24 * time.Hour
	}
	when := in.Now.UTC()
	token, err := newInviteToken()
	if err != nil {
		return Invite{}, err
	}
	id, err := newID()
	if err != nil {
		return Invite{}, err
	}
	expires := when.Add(in.TTL)
	var circleID any
	if in.CircleID != "" {
		circleID = in.CircleID
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO invites (id, token, circle_id, kind, max_uses, uses, expires_at, created_by_account_id, target_account_id, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, NULLIF(?, ''), NULLIF(?, ''), ?)
	`, id, token, circleID, string(in.Kind), in.MaxUses, formatTime(expires), in.CreatedByAccountID, in.TargetAccountID, formatTime(when))
	if err != nil {
		return Invite{}, fmt.Errorf("insert invite: %w", err)
	}
	return Invite{
		ID:                 id,
		Token:              token,
		CircleID:           in.CircleID,
		Kind:               in.Kind,
		MaxUses:            in.MaxUses,
		ExpiresAt:          expires,
		CreatedByAccountID: in.CreatedByAccountID,
		TargetAccountID:    in.TargetAccountID,
		CreatedAt:          when,
	}, nil
}

// MemberInviteTargets returns account ids with a live personal invite or pending join in the circle.
func (s *Service) MemberInviteTargets(ctx context.Context, circleID string, now time.Time) (map[string]bool, error) {
	out := map[string]bool{}
	rows, err := s.db.QueryContext(ctx, `
		SELECT account_id FROM pending_circle_joins WHERE circle_id = ?
	`, circleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = s.db.QueryContext(ctx, `
		SELECT target_account_id FROM invites
		WHERE circle_id = ? AND target_account_id IS NOT NULL
		  AND revoked_at IS NULL AND uses < max_uses AND expires_at > ?
	`, circleID, formatTime(now.UTC()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// HasPendingCircleJoin reports whether the account must complete naming for
// the circle. Нулевое now — текущее время.
func (s *Service) HasPendingCircleJoin(ctx context.Context, accountID, circleID string, now time.Time) (bool, error) {
	when := now.UTC()
	if when.IsZero() {
		when = time.Now().UTC()
	}
	return s.hasPendingCircleJoin(ctx, accountID, circleID, when)
}

// ListPendingCircleJoins returns circle ids awaiting join for the account.
func (s *Service) ListPendingCircleJoins(ctx context.Context, accountID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT circle_id FROM pending_circle_joins
		WHERE account_id = ? AND expires_at IS NOT NULL AND expires_at > ?
		ORDER BY created_at
	`, accountID, formatTime(time.Now().UTC()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// consumeMemberInvite удалён (STB-1): он повторно увеличивал uses личной
// ссылки, уже израсходованной в Verify, а оба его выхода возвращали nil —
// охраной он не был.
