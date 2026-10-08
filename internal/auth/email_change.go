package auth

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// RequestEmailChangeInput — вошедший участник называет новую почту.
type RequestEmailChangeInput struct {
	AccountID string
	Email     string
	ClientIP  string
	Now       time.Time
}

// ConfirmEmailChangeInput — код с новой почты. SessionToken — сессия, с
// которой пришёл запрос: она остаётся, остальные закрываются.
type ConfirmEmailChangeInput struct {
	AccountID    string
	Code         string
	SessionToken string
	ClientIP     string
	Now          time.Time
}

// EmailChange — что на что сменилось; обработчику — для писем.
type EmailChange struct {
	AccountID string
	OldEmail  string
	NewEmail  string
}

// RequestEmailChange шлёт код на новую почту. До ввода кода ничего не
// меняется: вход остаётся по прежней. Занятый адрес — ErrConflict: два входа
// в один не сливаются.
func (s *Service) RequestEmailChange(ctx context.Context, in RequestEmailChangeInput) error {
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
	acc, err := s.AccountByID(ctx, in.AccountID)
	if err != nil {
		return err
	}
	if acc.Email == AdminSentinelEmail {
		return ErrForbidden
	}
	if normalizeEmail(acc.Email) == email {
		return ErrInvalid
	}
	// Лимит до поиска адреса, как у входа: иначе вошедший перебирал бы,
	// чьи почты есть на сервере, без счёта (AUTH-2).
	if err := s.chargeRate(ctx, in.ClientIP, email, when); err != nil {
		return err
	}
	if _, err := s.accountByEmail(ctx, email); err == nil {
		return ErrConflict
	} else if err != ErrNotFound {
		return err
	}
	return s.issueCode(ctx, issueCodeInput{
		email:     email,
		clientIP:  in.ClientIP,
		flow:      FlowEmailChange,
		accountID: acc.ID,
		when:      when,
	})
}

// ConfirmEmailChange меняет почту по коду с нового адреса. Неудачи считаются
// по адресу обратившегося тем же счётчиком, что у входа (SEC-8).
func (s *Service) ConfirmEmailChange(ctx context.Context, in ConfirmEmailChangeInput) (EmailChange, error) {
	when := in.Now.UTC()
	if when.IsZero() {
		when = time.Now().UTC()
	}
	in.Now = when
	key := limiterKey(in.ClientIP)
	allowed, delay := s.verifyLimiter.allow(key, when)
	if !allowed {
		return EmailChange{}, ErrRateLimited
	}
	if err := throttle(ctx, delay); err != nil {
		return EmailChange{}, err
	}
	res, err := s.confirmEmailChange(ctx, in)
	switch {
	case err == nil:
		s.verifyLimiter.reset(key)
	case errors.Is(err, ErrInvalid), errors.Is(err, ErrNotFound),
		errors.Is(err, ErrExpired), errors.Is(err, ErrTooManyAttempts):
		s.verifyLimiter.fail(key, when)
	}
	return res, err
}

func (s *Service) confirmEmailChange(ctx context.Context, in ConfirmEmailChangeInput) (EmailChange, error) {
	code := digitsOnly(in.Code)
	if in.AccountID == "" || len(code) != 6 {
		return EmailChange{}, ErrInvalid
	}
	var pendingID, email, expiresRaw, codeHash string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, expires_at, code_hash
		FROM pending_codes
		WHERE account_id = ? AND flow = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, in.AccountID, string(FlowEmailChange)).Scan(&pendingID, &email, &expiresRaw, &codeHash)
	if err == sql.ErrNoRows {
		return EmailChange{}, ErrNotFound
	}
	if err != nil {
		return EmailChange{}, fmt.Errorf("load pending change code: %w", err)
	}
	expires, err := parseTime(expiresRaw)
	if err != nil {
		return EmailChange{}, err
	}
	if !in.Now.Before(expires) {
		return EmailChange{}, ErrExpired
	}
	claimed, err := s.claimAttempt(ctx, pendingID)
	if err != nil {
		return EmailChange{}, err
	}
	if !claimed {
		return EmailChange{}, ErrTooManyAttempts
	}
	if subtle.ConstantTimeCompare([]byte(hashCode(pendingID, code)), []byte(codeHash)) != 1 {
		return EmailChange{}, ErrInvalid
	}
	acc, err := s.AccountByID(ctx, in.AccountID)
	if err != nil {
		return EmailChange{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return EmailChange{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// Код одноразовый — как у входа (AUTH-1).
	spentRes, err := tx.ExecContext(ctx, `DELETE FROM pending_codes WHERE id = ?`, pendingID)
	if err != nil {
		return EmailChange{}, err
	}
	if spent, err := spentRes.RowsAffected(); err != nil {
		return EmailChange{}, err
	} else if spent == 0 {
		return EmailChange{}, ErrInvalid
	}
	if err := setAccountEmailTx(ctx, tx, acc, email); err != nil {
		return EmailChange{}, err
	}
	// Это устройство остаётся вошедшим, остальные попросят войти заново уже
	// новой почтой (кадр 7.14).
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM sessions WHERE account_id = ? AND kind = ? AND token_hash != ?
	`, acc.ID, string(SessionParticipant), hashSessionToken(in.SessionToken)); err != nil {
		return EmailChange{}, fmt.Errorf("revoke other sessions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return EmailChange{}, err
	}
	return EmailChange{AccountID: acc.ID, OldEmail: acc.Email, NewEmail: email}, nil
}

// AdminSetAccountEmail — запасной путь: человек потерял ящик и вышел со всех
// устройств. Без кода; все сессии и push-подписки учётки закрываются, войти
// можно только кодом на новую почту (кадр 9.11).
func (s *Service) AdminSetAccountEmail(ctx context.Context, id, rawEmail string) (EmailChange, error) {
	if id == "" {
		return EmailChange{}, ErrInvalid
	}
	email, err := ParseParticipantEmail(rawEmail)
	if err != nil {
		return EmailChange{}, err
	}
	if email == AdminSentinelEmail {
		return EmailChange{}, ErrInvalid
	}
	acc, err := s.AccountByID(ctx, id)
	if err != nil {
		return EmailChange{}, err
	}
	if acc.Email == AdminSentinelEmail {
		return EmailChange{}, ErrNotFound
	}
	if normalizeEmail(acc.Email) == email {
		return EmailChange{}, ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return EmailChange{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := setAccountEmailTx(ctx, tx, acc, email); err != nil {
		return EmailChange{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM sessions WHERE account_id = ? AND kind = ?
	`, acc.ID, string(SessionParticipant)); err != nil {
		return EmailChange{}, fmt.Errorf("revoke sessions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM push_subscriptions WHERE account_id = ?
	`, acc.ID); err != nil {
		return EmailChange{}, fmt.Errorf("drop push subscriptions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return EmailChange{}, err
	}
	return EmailChange{AccountID: acc.ID, OldEmail: acc.Email, NewEmail: email}, nil
}

// setAccountEmailTx переписывает адрес и гасит коды, выданные на оба адреса
// и самой учётке: код входа на прежнюю почту после смены никого не впускает.
func setAccountEmailTx(ctx context.Context, tx *sql.Tx, acc Account, email string) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE accounts SET email = ? WHERE id = ? AND deleted_at IS NULL
	`, email, acc.ID); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrConflict
		}
		return fmt.Errorf("set account email: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM pending_codes WHERE email IN (?, ?) OR account_id = ?
	`, acc.Email, email, acc.ID); err != nil {
		return fmt.Errorf("purge codes after email change: %w", err)
	}
	return nil
}
