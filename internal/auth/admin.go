package auth

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"fmt"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// MinPasswordLen is the shortest admin password Bootstrap, change and SetAdminPassword accept.
const MinPasswordLen = 8

func limiterKey(clientIP string) string {
	if clientIP == "" {
		return "unknown"
	}
	return clientIP
}

func (s *Service) gateBootstrapToken(ctx context.Context, token, expectedToken, clientIP string, when time.Time) error {
	key := limiterKey(clientIP)
	ok, delay := s.loginLimiter.allow(key, when)
	if !ok {
		return ErrRateLimited
	}
	if err := throttle(ctx, delay); err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
		s.loginLimiter.fail(key, when)
		return ErrInvalid
	}
	return nil
}

// ConfirmBootstrapToken checks the one-time install token without completing bootstrap.
func (s *Service) ConfirmBootstrapToken(ctx context.Context, token, expectedToken, clientIP string, now time.Time) error {
	when := now.UTC()
	if when.IsZero() {
		when = time.Now().UTC()
	}
	if err := s.gateBootstrapToken(ctx, token, expectedToken, clientIP, when); err != nil {
		return err
	}
	var bootstrapped int
	if err := s.db.QueryRowContext(ctx, `
		SELECT bootstrapped FROM instance_settings WHERE id = 1
	`).Scan(&bootstrapped); err != nil {
		return fmt.Errorf("load bootstrap state: %w", err)
	}
	if bootstrapped == 1 {
		return ErrInvalid
	}
	return nil
}

// ValidatePassword checks an admin password against the minimum length.
func ValidatePassword(password string) error {
	if password == "" {
		return ErrInvalid
	}
	if utf8.RuneCountInString(password) < MinPasswordLen {
		return ErrWeakPassword
	}
	return nil
}

// Bootstrap performs the first-run setup once: admin password and initial instance settings in one transaction, after the token gate (server-reference, «Первый запуск»).
func (s *Service) Bootstrap(ctx context.Context, in BootstrapInput, expectedToken string) error {
	when := in.Now.UTC()
	if when.IsZero() {
		when = time.Now().UTC()
	}
	if err := s.gateBootstrapToken(ctx, in.Token, expectedToken, in.ClientIP, when); err != nil {
		return err
	}
	if err := ValidatePassword(in.Password); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Флаг bootstrapped проверяется самим UPDATE: чтение до транзакции
	// оставляло окно, в котором два параллельных вызова оба проходили.
	res, err := tx.ExecContext(ctx, `
		UPDATE instance_settings
		SET name = ?, bootstrapped = 1, updated_at = ?
		WHERE id = 1 AND bootstrapped = 0
	`, in.InstanceName, formatTime(when))
	if err != nil {
		return fmt.Errorf("update instance: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update instance: %w", err)
	}
	if n == 0 {
		return ErrInvalid
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO admin_credentials (id, password_hash, updated_at)
		VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET password_hash = excluded.password_hash, updated_at = excluded.updated_at
	`, string(hash), formatTime(when)); err != nil {
		return fmt.Errorf("store admin password: %w", err)
	}
	if in.InTx != nil {
		if err := in.InTx(ctx, tx); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.loginLimiter.reset(limiterKey(in.ClientIP))
	return nil
}

// AdminLogin checks the admin password under the failure limiter and issues an admin session.
func (s *Service) AdminLogin(ctx context.Context, in AdminLoginInput) (Session, error) {
	when := in.Now.UTC()
	if when.IsZero() {
		when = time.Now().UTC()
	}
	if in.Password == "" {
		return Session{}, ErrInvalid
	}
	key := limiterKey(in.ClientIP)
	ok, delay := s.loginLimiter.allow(key, when)
	if !ok {
		return Session{}, ErrRateLimited
	}
	if err := throttle(ctx, delay); err != nil {
		return Session{}, err
	}

	var hash string
	err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM admin_credentials WHERE id = 1`).Scan(&hash)
	if err == sql.ErrNoRows {
		s.loginLimiter.fail(key, when)
		return Session{}, ErrForbidden
	}
	if err != nil {
		return Session{}, fmt.Errorf("load admin credentials: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)); err != nil {
		s.loginLimiter.fail(key, when)
		return Session{}, ErrForbidden
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, err
	}
	defer func() { _ = tx.Rollback() }()

	adminAccountID, err := s.ensureAdminAccount(ctx, tx, when)
	if err != nil {
		return Session{}, err
	}
	sess, err := s.createSession(ctx, tx, adminAccountID, SessionAdmin, when)
	if err != nil {
		return Session{}, err
	}
	if err := tx.Commit(); err != nil {
		return Session{}, err
	}
	s.loginLimiter.reset(key)
	return sess, nil
}

func (s *Service) ensureAdminAccount(ctx context.Context, tx *sql.Tx, when time.Time) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id FROM accounts WHERE email = ?`, AdminSentinelEmail).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	acc, err := s.createAccount(ctx, tx, AdminSentinelEmail, when)
	if err != nil {
		return "", err
	}
	return acc.ID, nil
}

// SetAdminPassword sets a new panel password without the current one and drops all admin sessions.
//
// Только для команды `wynd admin-password` на хосте: кто забыл пароль
// панели, тот держит сервер — ему и менять. Сброс по токену
// (IssueAdminReset) удалён: токен некуда было доставить, у панели нет почты
// (план 42, BKP-4). Колонки reset_token/reset_expires_at остаются — схема
// только растёт.
func (s *Service) SetAdminPassword(ctx context.Context, password string, when time.Time) error {
	if utf8.RuneCountInString(password) < MinPasswordLen {
		return ErrWeakPassword
	}
	if when.IsZero() {
		when = time.Now().UTC()
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_credentials WHERE id = 1`).Scan(&n); err != nil {
		return fmt.Errorf("load admin credentials: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.storeAdminPassword(ctx, string(hash), when)
}

// storeAdminPassword пишет новый хеш и в той же транзакции отзывает админские
// сессии: смена пароля обязана закрывать чужие открытые панели (AUTH-3).
func (s *Service) storeAdminPassword(ctx context.Context, hash string, when time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		UPDATE admin_credentials
		SET password_hash = ?, reset_token = NULL, reset_expires_at = NULL, updated_at = ?
		WHERE id = 1
	`, hash, formatTime(when)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM sessions WHERE kind = ?
	`, string(SessionAdmin)); err != nil {
		return fmt.Errorf("revoke admin sessions: %w", err)
	}
	return tx.Commit()
}

// ChangeAdminPassword verifies the current panel password and stores a new hash.
func (s *Service) ChangeAdminPassword(ctx context.Context, current, next string, when time.Time) error {
	if current == "" || next == "" {
		return ErrInvalid
	}
	if utf8.RuneCountInString(next) < MinPasswordLen {
		return ErrWeakPassword
	}
	if when.IsZero() {
		when = time.Now().UTC()
	}
	var hash string
	err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM admin_credentials WHERE id = 1`).Scan(&hash)
	if err == sql.ErrNoRows {
		return ErrForbidden
	}
	if err != nil {
		return fmt.Errorf("load admin credentials: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(current)); err != nil {
		return ErrForbidden
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.storeAdminPassword(ctx, string(newHash), when)
}
