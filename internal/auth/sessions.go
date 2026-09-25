package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// hashSessionToken — то, что лежит в БД вместо самого токена: бэкап и дамп
// больше не содержат живых Bearer на 30 дней вперёд (AUTH-4).
func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("session token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func (s *Service) createSession(ctx context.Context, tx *sql.Tx, accountID string, kind SessionKind, when time.Time) (Session, error) {
	token, err := newSessionToken()
	if err != nil {
		return Session{}, err
	}
	var ttl time.Duration
	switch kind {
	case SessionAdmin:
		ttl = adminSessionTTL
	default:
		ttl = sessionTTL
	}
	expires := when.Add(ttl)
	created := formatTime(when)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sessions (token_hash, account_id, kind, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, hashSessionToken(token), accountID, string(kind), formatTime(expires), created); err != nil {
		return Session{}, fmt.Errorf("insert session: %w", err)
	}
	return Session{
		Token:     token,
		AccountID: accountID,
		Kind:      kind,
		ExpiresAt: expires,
		CreatedAt: when,
	}, nil
}

// SessionByToken finds an unexpired session of the given kind by the hash of its token.
func (s *Service) SessionByToken(ctx context.Context, token string, kind SessionKind) (Session, error) {
	var sess Session
	var kindRaw, expiresRaw, createdRaw string
	err := s.db.QueryRowContext(ctx, `
		SELECT account_id, kind, expires_at, created_at
		FROM sessions WHERE token_hash = ? AND kind = ?
	`, hashSessionToken(token), string(kind)).Scan(&sess.AccountID, &kindRaw, &expiresRaw, &createdRaw)
	sess.Token = token
	if err == sql.ErrNoRows {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("session by token: %w", err)
	}
	sess.Kind = SessionKind(kindRaw)
	sess.ExpiresAt, err = parseTime(expiresRaw)
	if err != nil {
		return Session{}, err
	}
	sess.CreatedAt, err = parseTime(createdRaw)
	if err != nil {
		return Session{}, err
	}
	if !time.Now().UTC().Before(sess.ExpiresAt) {
		return Session{}, ErrExpired
	}
	return sess, nil
}

// RevokeSession deletes the session with this token.
func (s *Service) RevokeSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, hashSessionToken(token))
	return err
}

// IsParticipantSession resolves a participant session and refuses a blocked account.
func (s *Service) IsParticipantSession(ctx context.Context, token string) (Session, error) {
	sess, err := s.SessionByToken(ctx, token, SessionParticipant)
	if err != nil {
		return Session{}, err
	}
	acc, err := s.AccountByID(ctx, sess.AccountID)
	if err != nil {
		return Session{}, err
	}
	if acc.Blocked {
		return Session{}, ErrForbidden
	}
	return sess, nil
}

// IsAdminSession resolves an admin session.
func (s *Service) IsAdminSession(ctx context.Context, token string) (Session, error) {
	return s.SessionByToken(ctx, token, SessionAdmin)
}

// RejectAdminJournal forbids the journal to an admin session: the panel never reads circles.
func RejectAdminJournal(kind SessionKind) error {
	if kind == SessionAdmin {
		return ErrForbidden
	}
	return nil
}
