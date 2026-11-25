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

func (s *Service) createSession(ctx context.Context, tx *sql.Tx, accountID string, kind SessionKind, when time.Time, device string) (Session, error) {
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
		INSERT INTO sessions (token_hash, account_id, kind, expires_at, created_at, device, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, hashSessionToken(token), accountID, string(kind), formatTime(expires), created, device, created); err != nil {
		return Session{}, fmt.Errorf("insert session: %w", err)
	}
	return Session{
		Token:     token,
		AccountID: accountID,
		Kind:      kind,
		ExpiresAt: expires,
		CreatedAt: when,
		Device:    device,
		LastSeen:  when,
	}, nil
}

// SessionByToken finds an unexpired session of the given kind by the hash of its token.
func (s *Service) SessionByToken(ctx context.Context, token string, kind SessionKind) (Session, error) {
	var sess Session
	var kindRaw, expiresRaw, createdRaw, lastSeenRaw string
	err := s.db.QueryRowContext(ctx, `
		SELECT account_id, kind, expires_at, created_at, device, last_seen_at
		FROM sessions WHERE token_hash = ? AND kind = ?
	`, hashSessionToken(token), string(kind)).Scan(&sess.AccountID, &kindRaw, &expiresRaw, &createdRaw, &sess.Device, &lastSeenRaw)
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
	sess.LastSeen, err = parseTime(lastSeenRaw)
	if err != nil {
		return Session{}, err
	}
	if !time.Now().UTC().Before(sess.ExpiresAt) {
		return Session{}, ErrExpired
	}
	return sess, nil
}

// SessionID — короткий открытый идентификатор сессии для списка и отзыва:
// префикс хеша токена. Хеш 256-битного случайного токена сам по себе
// доступ не даёт, но наружу целиком его всё равно не отдаём (план 72).
func SessionID(token string) string {
	return hashSessionToken(token)[:16]
}

// TouchSession записывает активность: last_seen_at и скользящий
// expires_at = «сейчас» + срок (план 72). Пишет не чаще раза в час на
// сессию; пустая метка устройство не затирает.
func (s *Service) TouchSession(ctx context.Context, token, device string) error {
	hash := hashSessionToken(token)
	now := time.Now().UTC()
	s.touchMu.Lock()
	if last, ok := s.touchSeen[hash]; ok && now.Sub(last) < sessionTouchEvery {
		s.touchMu.Unlock()
		return nil
	}
	s.touchSeen[hash] = now
	s.touchMu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		UPDATE sessions
		SET last_seen_at = ?, expires_at = ?,
		    device = CASE WHEN ? = '' THEN device ELSE ? END
		WHERE token_hash = ?
	`, formatTime(now), formatTime(now.Add(sessionTTL)), device, device, hash)
	if err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

// ListSessions — все сессии учётной записи данного вида, свежая активность
// сверху. Токены не возвращаются.
func (s *Service) ListSessions(ctx context.Context, accountID string, kind SessionKind) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT token_hash, kind, expires_at, created_at, device, last_seen_at
		FROM sessions WHERE account_id = ? AND kind = ?
		ORDER BY last_seen_at DESC, created_at DESC
	`, accountID, string(kind))
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	out := make([]Session, 0, 4)
	for rows.Next() {
		var sess Session
		var kindRaw, expiresRaw, createdRaw, lastSeenRaw string
		if err := rows.Scan(&sess.Token, &kindRaw, &expiresRaw, &createdRaw, &sess.Device, &lastSeenRaw); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sess.Kind = SessionKind(kindRaw)
		if sess.ExpiresAt, err = parseTime(expiresRaw); err != nil {
			return nil, err
		}
		if sess.CreatedAt, err = parseTime(createdRaw); err != nil {
			return nil, err
		}
		if sess.LastSeen, err = parseTime(lastSeenRaw); err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	return out, nil
}

// RevokeSessionByID deletes the account's session by its open id (SessionID).
func (s *Service) RevokeSessionByID(ctx context.Context, accountID string, kind SessionKind, id string) error {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM sessions
		WHERE account_id = ? AND kind = ? AND substr(token_hash, 1, ?) = ?
	`, accountID, string(kind), len(id), id)
	if err != nil {
		return fmt.Errorf("revoke session by id: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke session by id: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
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
