package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

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
		INSERT INTO sessions (token, account_id, kind, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, token, accountID, string(kind), formatTime(expires), created); err != nil {
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

func (s *Service) SessionByToken(ctx context.Context, token string, kind SessionKind) (Session, error) {
	var sess Session
	var kindRaw, expiresRaw, createdRaw string
	err := s.db.QueryRowContext(ctx, `
		SELECT token, account_id, kind, expires_at, created_at
		FROM sessions WHERE token = ? AND kind = ?
	`, token, string(kind)).Scan(&sess.Token, &sess.AccountID, &kindRaw, &expiresRaw, &createdRaw)
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

func (s *Service) RevokeSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	return err
}

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

func (s *Service) IsAdminSession(ctx context.Context, token string) (Session, error) {
	return s.SessionByToken(ctx, token, SessionAdmin)
}

func RejectAdminJournal(kind SessionKind) error {
	if kind == SessionAdmin {
		return ErrForbidden
	}
	return nil
}
