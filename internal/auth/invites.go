package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// Потолки на ссылку (аудит 2026-09-22, SEC-9): и число входов, и срок
// задаёт участник, а непроверенные значения давали вечную ссылку с
// неограниченным числом входов.
const (
	maxInviteUses = 100
	maxInviteTTL  = 30 * 24 * time.Hour
)

// CreateInvite creates a circle or server invite link within the use and TTL ceilings.
func (s *Service) CreateInvite(ctx context.Context, in CreateInviteInput) (Invite, error) {
	if in.MaxUses < 1 || in.MaxUses > maxInviteUses {
		return Invite{}, ErrInvalid
	}
	if in.Kind == InviteSingle {
		in.MaxUses = 1
	}
	if in.TTL <= 0 {
		in.TTL = 7 * 24 * time.Hour
	}
	if in.TTL > maxInviteTTL {
		return Invite{}, ErrInvalid
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

// CreateServerInvite creates an invite to the server without a circle.
func (s *Service) CreateServerInvite(ctx context.Context, in CreateServerInviteInput) (Invite, error) {
	return s.CreateInvite(ctx, CreateInviteInput{
		Kind:               in.Kind,
		MaxUses:            in.MaxUses,
		TTL:                in.TTL,
		CreatedByAccountID: in.CreatedByAccountID,
		Now:                in.Now,
	})
}

// AcceptInvite validates an invite link for the email and sends a login code; a personal invite only to its target account.
func (s *Service) AcceptInvite(ctx context.Context, in AcceptInviteInput) error {
	if in.Token == "" {
		return ErrInvalid
	}
	email, err := ParseParticipantEmail(in.Email)
	if err != nil {
		return err
	}
	if email == AdminSentinelEmail {
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
	if err := s.chargeRate(ctx, in.ClientIP, email, when); err != nil {
		return err
	}
	acc, accErr := s.accountByEmail(ctx, email)
	switch {
	case accErr == nil && acc.Blocked:
		return ErrForbidden
	case accErr != nil && accErr != ErrNotFound:
		return accErr
	}
	// Личная ссылка — только своему адресу. Проверка стоит до выдачи кода:
	// иначе чужой адрес получал письмо и упирался в отказ только на Verify
	// (AUTH-4).
	if inv.TargetAccountID != "" {
		if accErr != nil || inv.TargetAccountID != acc.ID {
			return ErrForbidden
		}
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
	var circleID, revoked, createdBy, targetAccount sql.NullString
	var kind string
	var expiresRaw, createdRaw string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, token, circle_id, kind, max_uses, uses, expires_at, revoked_at, created_by_account_id, target_account_id, created_at
		FROM invites WHERE token = ?
	`, token).Scan(&inv.ID, &inv.Token, &circleID, &kind, &inv.MaxUses, &inv.Uses, &expiresRaw, &revoked, &createdBy, &targetAccount, &createdRaw)
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
	if targetAccount.Valid {
		inv.TargetAccountID = targetAccount.String
	}
	inv.CreatedAt, err = parseTime(createdRaw)
	if err != nil {
		return Invite{}, err
	}
	return inv, nil
}

func (s *Service) inviteByID(ctx context.Context, id string) (Invite, error) {
	var inv Invite
	var circleID, revoked, createdBy, targetAccount sql.NullString
	var token, kind string
	var expiresRaw, createdRaw string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, token, circle_id, kind, max_uses, uses, expires_at, revoked_at, created_by_account_id, target_account_id, created_at
		FROM invites WHERE id = ?
	`, id).Scan(&inv.ID, &token, &circleID, &kind, &inv.MaxUses, &inv.Uses, &expiresRaw, &revoked, &createdBy, &targetAccount, &createdRaw)
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
	if targetAccount.Valid {
		inv.TargetAccountID = targetAccount.String
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

// ValidateInvite — та же проверка, что и при использовании ссылки, для
// обработчиков. Просмотр ссылки проверял только отзыв и срок и показывал
// круг по исчерпанной ссылке и на закрытом сервере (SEC-9).
func (s *Service) ValidateInvite(ctx context.Context, inv Invite, when time.Time) error {
	return s.validateInvite(ctx, inv, when)
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

// revokeInvitesCreatedBy отзывает живые ссылки автора и снимает отложенные
// вступления, заведённые по ним (аудит 2026-09-22, SEC-9).
func (s *Service) revokeInvitesCreatedBy(ctx context.Context, accountID, nowRaw string) error {
	if accountID == "" {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := revokeInvitesCreatedByTx(ctx, tx, accountID, "", nowRaw); err != nil {
		return err
	}
	return tx.Commit()
}

// revokeInvitesCreatedByTx отзывает ссылки автора; пустой circleID — все его
// ссылки, иначе только ссылки в этот круг.
func revokeInvitesCreatedByTx(ctx context.Context, tx *sql.Tx, accountID, circleID, nowRaw string) error {
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM pending_circle_joins
		WHERE invite_id IN (
			SELECT id FROM invites
			WHERE created_by_account_id = ?
			  AND (? = '' OR circle_id = ?)
		)
	`, accountID, circleID, circleID); err != nil {
		return fmt.Errorf("clear pending joins of author: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE invites SET revoked_at = ?
		WHERE created_by_account_id = ? AND revoked_at IS NULL
		  AND (? = '' OR circle_id = ?)
	`, nowRaw, accountID, circleID, circleID); err != nil {
		return fmt.Errorf("revoke invites of author: %w", err)
	}
	return nil
}

// RevokeCircleInvitesBy отзывает ссылки, выданные этим участником в этот
// круг. Зовётся после выхода и исключения: ушедший не должен оставлять
// живую ссылку, по которой в круг входят уже без него (SEC-9).
func (s *Service) RevokeCircleInvitesBy(ctx context.Context, circleID, accountID string, now time.Time) error {
	if circleID == "" || accountID == "" {
		return ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := revokeInvitesCreatedByTx(ctx, tx, accountID, circleID, formatTime(now.UTC())); err != nil {
		return err
	}
	return tx.Commit()
}

func newInviteToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
