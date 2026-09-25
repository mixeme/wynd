package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

func (s *Service) CreateInvite(ctx context.Context, in CreateInviteInput) (Invite, error) {
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
	if when.IsZero() {
		when = time.Now().UTC()
	}

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
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO invites (id, token, circle_id, kind, max_uses, uses, expires_at, created_by_account_id, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, NULLIF(?, ''), ?)
	`, id, token, circleID, string(in.Kind), in.MaxUses, formatTime(expires), in.CreatedByAccountID, formatTime(when))
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
		CreatedAt:          when,
	}, nil
}

func (s *Service) CreateServerInvite(ctx context.Context, in CreateServerInviteInput) (Invite, error) {
	return s.CreateInvite(ctx, CreateInviteInput{
		Kind:    in.Kind,
		MaxUses: in.MaxUses,
		TTL:     in.TTL,
		Now:     in.Now,
	})
}

func (s *Service) AcceptInvite(ctx context.Context, in AcceptInviteInput) error {
	email := normalizeEmail(in.Email)
	if email == "" || email == AdminSentinelEmail || in.Token == "" {
		return ErrInvalid
	}
	when := in.Now.UTC()
	if when.IsZero() {
		when = time.Now().UTC()
	}

	inv, err := s.inviteByToken(ctx, in.Token)
	if err != nil {
		return err
	}
	if err := s.validateInvite(ctx, inv, when); err != nil {
		return err
	}
	if err := s.checkRate(ctx, in.ClientIP, email, when); err != nil {
		return err
	}
	if acc, err := s.accountByEmail(ctx, email); err == nil && acc.Blocked {
		return ErrForbidden
	} else if err != nil && err != ErrNotFound {
		return err
	}
	return s.issueCode(ctx, issueCodeInput{
		email:      email,
		clientIP:   in.ClientIP,
		flow:       FlowInvite,
		inviteID:   inv.ID,
		inviteName: in.Name,
		when:       when,
	})
}

// InviteByToken loads an invite by its public token.
func (s *Service) InviteByToken(ctx context.Context, token string) (Invite, error) {
	return s.inviteByToken(ctx, token)
}

func (s *Service) inviteByToken(ctx context.Context, token string) (Invite, error) {
	var inv Invite
	var circleID, revoked, createdBy sql.NullString
	var kind string
	var expiresRaw, createdRaw string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, token, circle_id, kind, max_uses, uses, expires_at, revoked_at, created_by_account_id, created_at
		FROM invites WHERE token = ?
	`, token).Scan(&inv.ID, &inv.Token, &circleID, &kind, &inv.MaxUses, &inv.Uses, &expiresRaw, &revoked, &createdBy, &createdRaw)
	if err == sql.ErrNoRows {
		return Invite{}, ErrNotFound
	}
	if err != nil {
		return Invite{}, fmt.Errorf("invite by token: %w", err)
	}
	if circleID.Valid {
		inv.CircleID = circleID.String
	}
	inv.Kind = InviteKind(kind)
	inv.ExpiresAt, err = parseTime(expiresRaw)
	if err != nil {
		return Invite{}, err
	}
	if revoked.Valid {
		t, err := parseTime(revoked.String)
		if err != nil {
			return Invite{}, err
		}
		inv.RevokedAt = &t
	}
	if createdBy.Valid {
		inv.CreatedByAccountID = createdBy.String
	}
	inv.CreatedAt, err = parseTime(createdRaw)
	if err != nil {
		return Invite{}, err
	}
	return inv, nil
}

func (s *Service) inviteByID(ctx context.Context, id string) (Invite, error) {
	var inv Invite
	var circleID, revoked, createdBy sql.NullString
	var token, kind string
	var expiresRaw, createdRaw string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, token, circle_id, kind, max_uses, uses, expires_at, revoked_at, created_by_account_id, created_at
		FROM invites WHERE id = ?
	`, id).Scan(&inv.ID, &token, &circleID, &kind, &inv.MaxUses, &inv.Uses, &expiresRaw, &revoked, &createdBy, &createdRaw)
	if err == sql.ErrNoRows {
		return Invite{}, ErrNotFound
	}
	if err != nil {
		return Invite{}, fmt.Errorf("invite by id: %w", err)
	}
	inv.Token = token
	if circleID.Valid {
		inv.CircleID = circleID.String
	}
	inv.Kind = InviteKind(kind)
	inv.ExpiresAt, err = parseTime(expiresRaw)
	if err != nil {
		return Invite{}, err
	}
	if revoked.Valid {
		t, err := parseTime(revoked.String)
		if err != nil {
			return Invite{}, err
		}
		inv.RevokedAt = &t
	}
	if createdBy.Valid {
		inv.CreatedByAccountID = createdBy.String
	}
	inv.CreatedAt, err = parseTime(createdRaw)
	if err != nil {
		return Invite{}, err
	}
	return inv, nil
}

func (s *Service) validateInvite(ctx context.Context, inv Invite, when time.Time) error {
	mode, err := s.registrationMode(ctx)
	if err != nil {
		return err
	}
	if mode == ModeClosed {
		return ErrClosed
	}
	if inv.RevokedAt != nil {
		return ErrInvalid
	}
	if !when.Before(inv.ExpiresAt) {
		return ErrExpired
	}
	if inv.Uses >= inv.MaxUses {
		return ErrInvalid
	}
	return nil
}

func (s *Service) consumeInvite(ctx context.Context, tx *sql.Tx, inv Invite, when time.Time) error {
	if err := s.validateInvite(ctx, inv, when); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE invites SET uses = uses + 1 WHERE id = ? AND uses < max_uses
	`, inv.ID)
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

func newInviteToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
