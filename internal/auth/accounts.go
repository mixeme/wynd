package auth

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func normalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}

func (s *Service) accountByEmail(ctx context.Context, email string) (Account, error) {
	email = normalizeEmail(email)
	var acc Account
	var created string
	var blocked int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, created_at, blocked FROM accounts
		WHERE email = ? AND deleted_at IS NULL
	`, email).Scan(&acc.ID, &acc.Email, &created, &blocked)
	if err == sql.ErrNoRows {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("account by email: %w", err)
	}
	acc.CreatedAt, err = parseTime(created)
	if err != nil {
		return Account{}, err
	}
	acc.Blocked = blocked != 0
	return acc, nil
}

func (s *Service) createAccount(ctx context.Context, tx *sql.Tx, email string, when time.Time) (Account, error) {
	email = normalizeEmail(email)
	id, err := newID()
	if err != nil {
		return Account{}, err
	}
	created := formatTime(when)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO accounts (id, email, created_at) VALUES (?, ?, ?)
	`, id, email, created); err != nil {
		return Account{}, fmt.Errorf("insert account: %w", err)
	}
	return Account{ID: id, Email: email, CreatedAt: when}, nil
}

func (s *Service) AccountByID(ctx context.Context, id string) (Account, error) {
	var acc Account
	var created string
	var blocked int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, created_at, blocked FROM accounts
		WHERE id = ? AND deleted_at IS NULL
	`, id).Scan(&acc.ID, &acc.Email, &created, &blocked)
	if err == sql.ErrNoRows {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("account by id: %w", err)
	}
	acc.CreatedAt, err = parseTime(created)
	if err != nil {
		return Account{}, err
	}
	acc.Blocked = blocked != 0
	return acc, nil
}

// AccountCircle is a circle membership row for the admin account card.
type AccountCircle struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color"`
	Role     string `json:"role"`
	JoinedAt string `json:"joined_at,omitempty"`
}

// AccountDetail is the admin GET /accounts/{id} payload.
type AccountDetail struct {
	ID                    string          `json:"id"`
	Email                 string          `json:"email"`
	CreatedAt             string          `json:"created_at"`
	LastLoginAt           *string         `json:"last_login_at"`
	Blocked               bool            `json:"blocked"`
	OwnsCircle            bool            `json:"owns_circle"`
	SubscriptionRequired  bool            `json:"subscription_required"`
	SubscriptionExpiresAt *string         `json:"subscription_expires_at,omitempty"`
	Circles               []AccountCircle `json:"circles"`
}

func (s *Service) AccountDetail(ctx context.Context, id string) (AccountDetail, error) {
	if id == "" {
		return AccountDetail{}, ErrInvalid
	}
	var detail AccountDetail
	var blocked int
	var lastLogin sql.NullString
	var subscriptionExpires sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, created_at, last_login_at, blocked, subscription_expires_at
		FROM accounts
		WHERE id = ? AND deleted_at IS NULL
	`, id).Scan(&detail.ID, &detail.Email, &detail.CreatedAt, &lastLogin, &blocked, &subscriptionExpires)
	if err == sql.ErrNoRows {
		return AccountDetail{}, ErrNotFound
	}
	if err != nil {
		return AccountDetail{}, fmt.Errorf("account detail: %w", err)
	}
	detail.Blocked = blocked != 0
	detail.Circles = []AccountCircle{}
	if lastLogin.Valid && lastLogin.String != "" {
		detail.LastLoginAt = &lastLogin.String
	}
	if subscriptionExpires.Valid && subscriptionExpires.String != "" {
		v := subscriptionExpires.String
		detail.SubscriptionExpiresAt = &v
	}
	paySettings, err := s.loadPaySettingsRow(ctx)
	if err != nil {
		return AccountDetail{}, err
	}
	detail.SubscriptionRequired = paySettings.SubscriptionRequired
	var owns int
	if err := s.db.QueryRowContext(ctx, `
		SELECT 1 FROM circles WHERE owner_account_id = ? LIMIT 1
	`, id).Scan(&owns); err == nil {
		detail.OwnsCircle = true
	} else if err != sql.ErrNoRows {
		return AccountDetail{}, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.color,
			CASE WHEN c.owner_account_id = ? THEN 'owner' ELSE 'member' END,
			m.created_at
		FROM memberships m
		JOIN circles c ON c.id = m.circle_id
		WHERE m.account_id = ? AND m.status = 'active'
		ORDER BY c.name COLLATE NOCASE
	`, id, id)
	if err != nil {
		return AccountDetail{}, fmt.Errorf("account circles: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var row AccountCircle
		if err := rows.Scan(&row.ID, &row.Name, &row.Color, &row.Role, &row.JoinedAt); err != nil {
			return AccountDetail{}, err
		}
		detail.Circles = append(detail.Circles, row)
	}
	return detail, rows.Err()
}

// DeleteAccount soft-deletes a participant account. Owner accounts return ErrConflict.
func (s *Service) DeleteAccount(ctx context.Context, id string, now time.Time) error {
	if id == "" {
		return ErrInvalid
	}
	acc, err := s.AccountByID(ctx, id)
	if err != nil {
		return err
	}
	if acc.Email == AdminSentinelEmail {
		return ErrNotFound
	}
	var owns int
	if err := s.db.QueryRowContext(ctx, `
		SELECT 1 FROM circles WHERE owner_account_id = ? LIMIT 1
	`, id).Scan(&owns); err == nil {
		return ErrConflict
	} else if err != sql.ErrNoRows {
		return err
	}
	memRows, err := s.db.QueryContext(ctx, `
		SELECT circle_id FROM memberships WHERE account_id = ? AND status = 'active'
	`, id)
	if err != nil {
		return fmt.Errorf("list memberships: %w", err)
	}
	defer memRows.Close()
	var circleIDs []string
	for memRows.Next() {
		var circleID string
		if err := memRows.Scan(&circleID); err != nil {
			return err
		}
		circleIDs = append(circleIDs, circleID)
	}
	if err := memRows.Err(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if s.chronicle == nil {
		return fmt.Errorf("chronicle required for account delete")
	}
	for _, circleID := range circleIDs {
		if err := s.chronicle.LeaveInTx(ctx, tx, circleID, id, now); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE identities SET account_id = NULL WHERE account_id = ?
	`, id); err != nil {
		return fmt.Errorf("orphan identities: %w", err)
	}
	deletedEmail := fmt.Sprintf("deleted+%s@wynd.local", id)
	deletedAt := formatTime(now.UTC())
	if _, err := tx.ExecContext(ctx, `
		UPDATE accounts SET deleted_at = ?, blocked = 1, email = ? WHERE id = ?
	`, deletedAt, deletedEmail, id); err != nil {
		return fmt.Errorf("soft delete account: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM sessions WHERE account_id = ? AND kind = ?
	`, id, SessionParticipant); err != nil {
		return fmt.Errorf("revoke deleted sessions: %w", err)
	}
	// Хвосты учётки: незавершённые вступления и личные приглашения на неё.
	// Без этого удалённая учётка возвращалась в круг по старой ссылке (AUTH-4).
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM pending_circle_joins WHERE account_id = ?
	`, id); err != nil {
		return fmt.Errorf("clear pending joins: %w", err)
	}
	// Push-подписки привязаны к учётке, не к сессии: без снятия устройство
	// продолжало получать сигналы активности кругов (аудит 2026-09-22).
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM push_subscriptions WHERE account_id = ?
	`, id); err != nil {
		return fmt.Errorf("drop push subscriptions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE invites SET revoked_at = ?
		WHERE target_account_id = ? AND revoked_at IS NULL
	`, deletedAt, id); err != nil {
		return fmt.Errorf("revoke personal invites: %w", err)
	}
	return tx.Commit()
}
