package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Register starts sign-up without a circle — open mode only — and sends a login code; the rate limit is charged first.
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

	// Лимит проверяется до поиска учётки: иначе перебор адресов не бьёт по
	// лимиту вовсе, потому что счётчик рос только на дошедших до выдачи (AUTH-2).
	if err := s.chargeRate(ctx, in.ClientIP, email, when); err != nil {
		return err
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
			// Открытая регистрация отвечает одинаково на любой адрес. Отказ
			// именно заблокированному превращал endpoint в справочник: кто
			// здесь есть и кого закрыли (аудит 2026-09-22). Письма нет.
			return nil
		}
		flow = FlowLogin
	} else if err != ErrNotFound {
		return err
	}
	return s.issueCode(ctx, issueCodeInput{
		email:    email,
		clientIP: in.ClientIP,
		flow:     flow,
		when:     when,
	})
}

// RequestCode sends a login code to an existing account, charging the rate limit before the account lookup.
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
	// Тот же порядок, что в Register: лимит до поиска учётки (AUTH-2).
	if err := s.chargeRate(ctx, in.ClientIP, email, when); err != nil {
		return err
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
		// В открытом режиме ответ одинаков для любого адреса, поэтому и для
		// заблокированного — 200 без письма. В режимах invite и closed отказ
		// незнакомому адресу и так виден, там остаётся forbidden.
		mode, merr := s.registrationMode(ctx)
		if merr != nil {
			return merr
		}
		if mode == ModeOpen {
			return nil
		}
		return ErrForbidden
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
	`, pendingID, in.email, hashCode(pendingID, code), in.clientIP, string(in.flow), in.inviteID, in.inviteName, formatTime(expires), formatTime(in.when)); err != nil {
		return fmt.Errorf("insert pending code: %w", err)
	}
	// Запрос уже посчитан в chargeRate до выдачи: там попытка засчитывается
	// и неизвестному адресу тоже (AUTH-2).
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.mailer.SendCode(ctx, in.email, code)
}

// Verify обменивает код на сессию. Неудачные попытки считаются по адресу
// обратившегося: код живёт у почты, и знающий чужую почту сжигал её три
// попытки сколько угодно раз, запирая вход хозяину (аудит 2026-09-22, SEC-8).
func (s *Service) Verify(ctx context.Context, in VerifyInput) (VerifyResult, error) {
	when := in.Now.UTC()
	if when.IsZero() {
		when = time.Now().UTC()
	}
	in.Now = when
	key := limiterKey(in.ClientIP)
	allowed, delay := s.verifyLimiter.allow(key, when)
	if !allowed {
		return VerifyResult{}, ErrRateLimited
	}
	if err := throttle(ctx, delay); err != nil {
		return VerifyResult{}, err
	}
	res, err := s.verify(ctx, in)
	switch {
	case err == nil:
		s.verifyLimiter.reset(key)
	case errors.Is(err, ErrInvalid), errors.Is(err, ErrNotFound),
		errors.Is(err, ErrExpired), errors.Is(err, ErrTooManyAttempts),
		errors.Is(err, ErrForbidden):
		s.verifyLimiter.fail(key, when)
	}
	return res, err
}

func (s *Service) verify(ctx context.Context, in VerifyInput) (VerifyResult, error) {
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
	if subtle.ConstantTimeCompare([]byte(hashCode(pendingID, code)), []byte(codeHash)) != 1 {
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
			if inv.TargetAccountID != "" && inv.TargetAccountID != acc.ID {
				return VerifyResult{}, ErrForbidden
			}
			if err := s.consumeInvite(ctx, tx, inv, when); err != nil {
				return VerifyResult{}, err
			}
			if !inv.IsServer() {
				if s.chronicle == nil {
					return VerifyResult{}, fmt.Errorf("chronicle required for circle invite")
				}
				if inviteName == "" {
					// Право доназвать себя живёт сутки и не переживает
					// отзыв ссылки, по которой заведено (SEC-9).
					if err := s.insertPendingCircleJoin(ctx, tx, acc.ID, inv.CircleID, inv.ID, when, when.Add(pendingJoinTTL)); err != nil {
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
	// Код одноразовый: расход проверяется по RowsAffected, иначе два
	// параллельных Verify получали по сессии на один код (AUTH-1).
	spentRes, err := tx.ExecContext(ctx, `DELETE FROM pending_codes WHERE id = ?`, pendingID)
	if err != nil {
		return VerifyResult{}, err
	}
	spent, err := spentRes.RowsAffected()
	if err != nil {
		return VerifyResult{}, err
	}
	if spent == 0 {
		return VerifyResult{}, ErrInvalid
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

// chargeRate caps code requests per client address and per mailbox within
// ipRateLimitWindow. The mailbox cap protects a victim from being flooded
// with codes even when the caller rotates addresses.
// Попытка засчитывается всегда, даже когда адрес неизвестен и ответ — 404:
// иначе перебор чужих адресов вообще не бьёт по лимиту (AUTH-2).
func (s *Service) chargeRate(ctx context.Context, clientIP, email string, when time.Time) error {
	clientIP = limiterKey(clientIP)
	since := formatTime(when.Add(-ipRateLimitWindow))
	// Проверка и запись одним оператором: раздельные SELECT и INSERT
	// пропускали параллельные запросы сверх потолка (аудит 2026-09-22, SEC-8).
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO code_request_log (client_ip, email, requested_at)
		SELECT ?, ?, ?
		WHERE (SELECT COUNT(*) FROM code_request_log WHERE client_ip = ? AND requested_at >= ?) < ?
		  AND (SELECT COUNT(*) FROM code_request_log WHERE email = ? AND requested_at >= ?) < ?
	`, clientIP, email, formatTime(when),
		clientIP, since, ipRateLimit,
		email, since, emailRateLimit)
	if err != nil {
		return fmt.Errorf("log code request: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("log code request: %w", err)
	}
	if n > 0 {
		return nil
	}
	// Строка не легла — потолок выбран. checkRate считает, сколько ждать.
	if err := s.checkRate(ctx, clientIP, email, when); err != nil {
		return err
	}
	return &RateLimitError{RetryAfterSec: 1}
}

func (s *Service) checkRate(ctx context.Context, clientIP, email string, when time.Time) error {
	clientIP = limiterKey(clientIP)
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

// hashCode солит код идентификатором заявки: одинаковые шестизначные коды
// у разных заявок дают разные хеши, и таблица не подсказывает совпадения.
func hashCode(pendingID, code string) string {
	sum := sha256.Sum256([]byte(pendingID + ":" + code))
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
