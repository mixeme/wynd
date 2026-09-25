package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func (s *Service) Register(ctx context.Context, in RegisterInput) error {
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

	mode, err := s.registrationMode(ctx)
	if err != nil {
		return err
	}
	if mode != ModeOpen {
		return ErrClosed
	}
	// Open registration answers "code sent" whether or not the address is
	// known, so the endpoint does not reveal who has an account. An existing
	// account simply receives a login code instead of a registration one.
	flow := FlowRegister
	if acc, err := s.accountByEmail(ctx, email); err == nil {
		if acc.Blocked {
			return ErrForbidden
		}
		flow = FlowLogin
	} else if err != ErrNotFound {
		return err
	}
	if err := s.checkRate(ctx, in.ClientIP, email, when); err != nil {
		return err
	}
	return s.issueCode(ctx, issueCodeInput{
		email:    email,
		clientIP: in.ClientIP,
		flow:     flow,
		when:     when,
	})
}

func (s *Service) RequestCode(ctx context.Context, in RequestCodeInput) error {
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
	flow := FlowLogin
	acc, err := s.accountByEmail(ctx, email)
	if err == ErrNotFound {
		// In invite/closed mode the 404 is deliberate (see docs/security-audit).
		// With open registration the answer is uniform: send a register code.
		mode, merr := s.registrationMode(ctx)
		if merr != nil {
			return merr
		}
		if mode != ModeOpen {
			return err
		}
		flow = FlowRegister
	} else if err != nil {
		return err
	} else if acc.Blocked {
		return ErrForbidden
	}
	if err := s.checkRate(ctx, in.ClientIP, email, when); err != nil {
		return err
	}
	return s.issueCode(ctx, issueCodeInput{
		email:    email,
		clientIP: in.ClientIP,
		flow:     flow,
		when:     when,
	})
}

type issueCodeInput struct {
	email      string
	clientIP   string
	flow       Flow
	inviteID   string
	inviteName string
	when       time.Time
}

func (s *Service) issueCode(ctx context.Context, in issueCodeInput) error {
	code, err := randomCode()
	if err != nil {
		return err
	}
	pendingID, err := newID()
	if err != nil {
		return err
	}
	expires := in.when.Add(codeTTL)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM pending_codes WHERE email = ?
	`, in.email); err != nil {
		return fmt.Errorf("purge pending codes: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO pending_codes (id, email, code_hash, attempts, client_ip, flow, invite_id, invite_name, expires_at, created_at)
		VALUES (?, ?, ?, 0, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?)
	`, pendingID, in.email, hashCode(code), in.clientIP, string(in.flow), in.inviteID, in.inviteName, formatTime(expires), formatTime(in.when)); err != nil {
		return fmt.Errorf("insert pending code: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO code_request_log (client_ip, email, requested_at) VALUES (?, ?, ?)
	`, in.clientIP, in.email, formatTime(in.when)); err != nil {
		return fmt.Errorf("log code request: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.mailer.SendCode(ctx, in.email, code)
}

func (s *Service) Verify(ctx context.Context, in VerifyInput) (VerifyResult, error) {
	email := normalizeEmail(in.Email)
	code := digitsOnly(in.Code)
	if email == "" || email == AdminSentinelEmail || len(code) != 6 {
		return VerifyResult{}, ErrInvalid
	}
	when := in.Now.UTC()
	if when.IsZero() {
		when = time.Now().UTC()
	}

	var pendingID, flow, inviteID, inviteName, codeHash string
	var expiresRaw string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, flow, COALESCE(invite_id, ''), COALESCE(invite_name, ''), expires_at, code_hash
		FROM pending_codes
		WHERE email = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, email).Scan(&pendingID, &flow, &inviteID, &inviteName, &expiresRaw, &codeHash)
	if err == sql.ErrNoRows {
		return VerifyResult{}, ErrNotFound
	}
	if err != nil {
		return VerifyResult{}, fmt.Errorf("load pending code: %w", err)
	}
	expires, err := parseTime(expiresRaw)
	if err != nil {
		return VerifyResult{}, err
	}
	if !when.Before(expires) {
		return VerifyResult{}, ErrExpired
	}
	// Claim an attempt atomically before comparing, so parallel requests
	// cannot exceed codeMaxAttempts between a read and a bump.
	claimed, err := s.claimAttempt(ctx, pendingID)
	if err != nil {
		return VerifyResult{}, err
	}
	if !claimed {
		return VerifyResult{}, ErrTooManyAttempts
	}
	if subtle.ConstantTimeCompare([]byte(hashCode(code)), []byte(codeHash)) != 1 {
		return VerifyResult{}, ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return VerifyResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	acc, err := s.accountByEmail(ctx, email)
	switch Flow(flow) {
	case FlowLogin:
		if err != nil {
			return VerifyResult{}, ErrNotFound
		}
		if acc.Blocked {
			return VerifyResult{}, ErrForbidden
		}
	case FlowRegister:
		if err == nil {
			return VerifyResult{}, ErrInvalid
		}
		if err != ErrNotFound {
			return VerifyResult{}, err
		}
		acc, err = s.createAccount(ctx, tx, email, when)
		if err != nil {
			return VerifyResult{}, err
		}
	case FlowInvite:
		if err == ErrNotFound {
			acc, err = s.createAccount(ctx, tx, email, when)
			if err != nil {
				return VerifyResult{}, err
			}
		} else if err != nil {
			return VerifyResult{}, err
		}
		if acc.Blocked {
			return VerifyResult{}, ErrForbidden
		}
		if inviteID != "" {
			inv, err := s.inviteByID(ctx, inviteID)
			if err != nil {
				return VerifyResult{}, err
			}
			if err := s.consumeInvite(ctx, tx, inv, when); err != nil {
				return VerifyResult{}, err
			}
			if !inv.IsServer() {
				if s.chronicle == nil {
					return VerifyResult{}, fmt.Errorf("chronicle required for circle invite")
				}
				if inviteName == "" {
					if err := s.insertPendingCircleJoin(ctx, tx, acc.ID, inv.CircleID, when); err != nil {
						return VerifyResult{}, err
					}
				} else if _, _, err := s.chronicle.JoinInTx(ctx, tx, chronicle.JoinInput{
					CircleID:  inv.CircleID,
					AccountID: acc.ID,
					Name:      inviteName,
					Now:       when,
				}); err != nil {
					return VerifyResult{}, err
				}
			}
		}
	default:
		return VerifyResult{}, ErrInvalid
	}

	sess, err := s.createSession(ctx, tx, acc.ID, SessionParticipant, when)
	if err != nil {
		return VerifyResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM pending_codes WHERE id = ?`, pendingID); err != nil {
		return VerifyResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE accounts SET last_login_at = ? WHERE id = ?
	`, formatTime(when), acc.ID); err != nil {
		return VerifyResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return VerifyResult{}, err
	}

	pendingCircleID := ""
	if Flow(flow) == FlowInvite && inviteID != "" && inviteName == "" {
		inv, err := s.inviteByID(ctx, inviteID)
		if err == nil && !inv.IsServer() {
			pendingCircleID = inv.CircleID
		}
	}
	return VerifyResult{Session: sess, Account: acc, PendingCircleID: pendingCircleID}, nil
}

// claimAttempt increments attempts only while under the cap, in one statement.
func (s *Service) claimAttempt(ctx context.Context, id string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE pending_codes SET attempts = attempts + 1
		WHERE id = ? AND attempts < ?
	`, id, codeMaxAttempts)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// checkRate caps code requests per client address and per mailbox within
// ipRateLimitWindow. The mailbox cap protects a victim from being flooded
// with codes even when the caller rotates addresses.
func (s *Service) checkRate(ctx context.Context, clientIP, email string, when time.Time) error {
	if clientIP == "" {
		clientIP = "unknown"
	}
	since := formatTime(when.Add(-ipRateLimitWindow))
	var byIP, byEmail int
	err := s.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM code_request_log WHERE client_ip = ? AND requested_at >= ?),
			(SELECT COUNT(*) FROM code_request_log WHERE email = ? AND requested_at >= ?)
	`, clientIP, since, email, since).Scan(&byIP, &byEmail)
	if err != nil {
		return fmt.Errorf("rate limit: %w", err)
	}
	if byIP < ipRateLimit && byEmail < emailRateLimit {
		return nil
	}
	retry := 0
	if byIP >= ipRateLimit {
		sec, err := s.secondsUntilRateSlot(ctx, `
			SELECT MIN(requested_at) FROM code_request_log
			WHERE client_ip = ? AND requested_at >= ?
		`, clientIP, since, when)
		if err != nil {
			return err
		}
		retry = sec
	}
	if byEmail >= emailRateLimit {
		sec, err := s.secondsUntilRateSlot(ctx, `
			SELECT MIN(requested_at) FROM code_request_log
			WHERE email = ? AND requested_at >= ?
		`, email, since, when)
		if err != nil {
			return err
		}
		if sec > retry {
			retry = sec
		}
	}
	if retry < 1 {
		retry = 1
	}
	return &RateLimitError{RetryAfterSec: retry}
}

func (s *Service) secondsUntilRateSlot(ctx context.Context, query, arg, since string, when time.Time) (int, error) {
	var oldestRaw sql.NullString
	err := s.db.QueryRowContext(ctx, query, arg, since).Scan(&oldestRaw)
	if err != nil {
		return 0, fmt.Errorf("rate limit oldest: %w", err)
	}
	if !oldestRaw.Valid || oldestRaw.String == "" {
		return 1, nil
	}
	oldest, err := parseTime(oldestRaw.String)
	if err != nil {
		return 0, err
	}
	unlock := oldest.Add(ipRateLimitWindow)
	remaining := unlock.Sub(when)
	sec := int((remaining + time.Second - 1) / time.Second)
	if sec < 1 {
		sec = 1
	}
	return sec, nil
}

func randomCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func hashCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func digitsOnly(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			out = append(out, s[i])
		}
	}
	return string(out)
}
