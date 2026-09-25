package auth

// Срок подписки и шлюз: продление, статус участника, напоминания.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// SubscriptionUnlimitedAt is the stored end date for admin-granted lifetime access.
var SubscriptionUnlimitedAt = time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)

func subscriptionUnlimitedExpiry() string {
	return formatTime(SubscriptionUnlimitedAt)
}

func isSubscriptionUnlimitedRaw(raw string) bool {
	if raw == "" {
		return false
	}
	t, err := parseTime(raw)
	if err != nil {
		return false
	}
	return !t.Before(SubscriptionUnlimitedAt)
}

func subscriptionExtendBase(current sql.NullString, now time.Time) time.Time {
	base := now.UTC()
	if !current.Valid || current.String == "" {
		return base
	}
	if isSubscriptionUnlimitedRaw(current.String) {
		return base
	}
	t, err := parseTime(current.String)
	if err != nil {
		return base
	}
	if t.After(base) {
		return t
	}
	return base
}

// PayStatus is the participant payment gate state.
type PayStatus struct {
	Required            bool            `json:"required"`
	ExpiresAt           *string         `json:"expires_at"`
	Expired             bool            `json:"expired"`
	Pending             bool            `json:"pending"`
	PendingAt           *string         `json:"pending_at,omitempty"`
	PendingComment      *string         `json:"pending_comment,omitempty"`
	PendingBlobFilename *string         `json:"pending_blob_filename,omitempty"`
	Requisites          string          `json:"requisites"`
	HasRequisites       bool            `json:"has_requisites"`
	Banner              *PayBannerState `json:"banner"`
	Dismissed           bool            `json:"dismissed"`
	Reminder            bool            `json:"reminder"`
	ReminderDaysLeft    *int            `json:"reminder_days_left,omitempty"`
	InstanceName        string          `json:"instance_name,omitempty"`
}

// PayAccountSummary is a participant row for the admin subscription table.
type PayAccountSummary struct {
	ID                    string  `json:"id"`
	Email                 string  `json:"email"`
	Blocked               bool    `json:"blocked"`
	SubscriptionExpiresAt *string `json:"subscription_expires_at,omitempty"`
}

// PayAccount is one account for grant-without-request.
type PayAccount struct {
	ID                    string  `json:"id"`
	Email                 string  `json:"email"`
	SubscriptionExpiresAt *string `json:"subscription_expires_at,omitempty"`
}

func (s *Service) requirePaySubscription(ctx context.Context) error {
	settings, err := s.loadPaySettingsRow(ctx)
	if err != nil {
		return err
	}
	if !settings.SubscriptionRequired {
		return ErrInvalid
	}
	return nil
}

// ListPayAccounts lists accounts with their subscription term; only while the gate is on.
func (s *Service) ListPayAccounts(ctx context.Context) ([]PayAccountSummary, error) {
	if err := s.requirePaySubscription(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, email, blocked, subscription_expires_at
		FROM accounts
		WHERE email != ? AND deleted_at IS NULL
		ORDER BY email COLLATE NOCASE
	`, AdminSentinelEmail)
	if err != nil {
		return nil, fmt.Errorf("list pay accounts: %w", err)
	}
	defer rows.Close()
	out := []PayAccountSummary{}
	for rows.Next() {
		var row PayAccountSummary
		var blocked int
		var expires sql.NullString
		if err := rows.Scan(&row.ID, &row.Email, &blocked, &expires); err != nil {
			return nil, err
		}
		row.Blocked = blocked != 0
		if expires.Valid && expires.String != "" {
			v := expires.String
			row.SubscriptionExpiresAt = &v
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// PayAccountByID returns one account's subscription term; only while the gate is on.
func (s *Service) PayAccountByID(ctx context.Context, id string) (PayAccount, error) {
	if err := s.requirePaySubscription(ctx); err != nil {
		return PayAccount{}, err
	}
	if id == "" {
		return PayAccount{}, ErrInvalid
	}
	var row PayAccount
	var expires sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, subscription_expires_at
		FROM accounts
		WHERE id = ? AND email != ? AND deleted_at IS NULL
	`, id, AdminSentinelEmail).Scan(&row.ID, &row.Email, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return PayAccount{}, ErrNotFound
	}
	if err != nil {
		return PayAccount{}, err
	}
	if expires.Valid && expires.String != "" {
		v := expires.String
		row.SubscriptionExpiresAt = &v
	}
	return row, nil
}

// GrantPayAccount extends a subscription by days, or makes it unlimited, without a request.
//
// Чтение текущего срока и запись нового — в одной транзакции: двойное
// нажатие «продлить на 30» давало +60, как approve до аудита (план 42, PAY-4).
func (s *Service) GrantPayAccount(ctx context.Context, accountID string, days int, unlimited bool) error {
	if err := s.requirePaySubscription(ctx); err != nil {
		return err
	}
	if accountID == "" {
		return ErrInvalid
	}
	if !unlimited && (days < 1 || days > MaxPayGrantDays) {
		return ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var current sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT subscription_expires_at FROM accounts
		WHERE id = ? AND email != ? AND deleted_at IS NULL
	`, accountID, AdminSentinelEmail).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	next := subscriptionUnlimitedExpiry()
	if !unlimited {
		base := subscriptionExtendBase(current, time.Now().UTC())
		next = formatTime(base.AddDate(0, 0, days))
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE accounts SET subscription_expires_at = ?, pay_reminder_sent_for = NULL WHERE id = ?
	`, next, accountID); err != nil {
		return err
	}
	return tx.Commit()
}

// PayStatus is what the participant's client shows: gate, banner, reminder and pending request.
func (s *Service) PayStatus(ctx context.Context, accountID string, now time.Time) (PayStatus, error) {
	settings, err := s.loadPaySettingsRow(ctx)
	if err != nil {
		return PayStatus{}, err
	}
	_, name, _, err := s.loadInstance(ctx)
	if err != nil {
		return PayStatus{}, err
	}
	var expires sql.NullString
	var dismissedVersion int
	if err := s.db.QueryRowContext(ctx, `
		SELECT subscription_expires_at, pay_donate_dismissed_version FROM accounts WHERE id = ?
	`, accountID).Scan(&expires, &dismissedVersion); err != nil {
		return PayStatus{}, err
	}
	out := PayStatus{
		Required:      settings.SubscriptionRequired,
		Requisites:    settings.Requisites,
		HasRequisites: strings.TrimSpace(settings.Requisites) != "",
		InstanceName:  name,
		Dismissed:     dismissedVersion >= settings.DonateVersion && settings.DonateVersion > 0,
	}
	if expires.Valid && expires.String != "" {
		v := expires.String
		out.ExpiresAt = &v
	}
	out.Expired = settings.SubscriptionRequired && isSubscriptionExpired(out.ExpiresAt, now)
	var pendingComment sql.NullString
	var pendingAt string
	var pendingFilename sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT pr.created_at, pr.comment, b.original_filename
		FROM pay_requests pr
		LEFT JOIN blobs b ON b.id = pr.blob_id
		WHERE pr.account_id = ? AND pr.status = 'pending'
		ORDER BY pr.created_at DESC LIMIT 1
	`, accountID).Scan(&pendingAt, &pendingComment, &pendingFilename)
	if err == nil {
		out.Pending = true
		out.PendingAt = &pendingAt
		if pendingComment.Valid && pendingComment.String != "" {
			out.PendingComment = &pendingComment.String
		}
		if pendingFilename.Valid && pendingFilename.String != "" {
			out.PendingBlobFilename = &pendingFilename.String
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return PayStatus{}, err
	}
	if out.HasRequisites && donateBannerActive(settings, now) {
		out.Banner = &PayBannerState{
			Text: settings.DonateText, Dismissible: settings.DonateDismissible,
		}
	}
	if settings.SubscriptionRequired && !out.Expired && !out.Pending && out.ExpiresAt != nil {
		if days := reminderDaysLeft(*out.ExpiresAt, now, settings.SubscriptionRemindDays); days != nil {
			out.Reminder = true
			out.ReminderDaysLeft = days
		}
	}
	return out, nil
}

// PaymentRequired reports whether the payment gate locks the account out:
// the subscription is required, requisites are set and the term has run out.
//
// Шлюз зовётся на каждом paid-запросе и каждые 2 с в каждом открытом SSE;
// PayStatus для этого делал четыре запроса ради трёх полей (план 42, PAY-1).
// Правило то же, что `required && expired && has_requisites` в PayStatus.
func (s *Service) PaymentRequired(ctx context.Context, accountID string, now time.Time) (bool, error) {
	var required int
	var requisites string
	var expires sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT i.pay_subscription_required, i.pay_requisites, a.subscription_expires_at
		FROM instance_settings i, accounts a
		WHERE i.id = 1 AND a.id = ?
	`, accountID).Scan(&required, &requisites, &expires)
	if err != nil {
		return false, err
	}
	if required == 0 || strings.TrimSpace(requisites) == "" {
		return false, nil
	}
	var expiresAt *string
	if expires.Valid && expires.String != "" {
		expiresAt = &expires.String
	}
	return isSubscriptionExpired(expiresAt, now), nil
}

func isSubscriptionExpired(expiresAt *string, now time.Time) bool {
	if expiresAt == nil || *expiresAt == "" {
		return true
	}
	if isSubscriptionUnlimitedRaw(*expiresAt) {
		return false
	}
	t, err := parseTime(*expiresAt)
	if err != nil {
		return true
	}
	return !t.After(now.UTC())
}

func reminderDaysLeft(expiresAt string, now time.Time, remindDays int) *int {
	if isSubscriptionUnlimitedRaw(expiresAt) {
		return nil
	}
	t, err := parseTime(expiresAt)
	if err != nil {
		return nil
	}
	hours := t.Sub(now.UTC()).Hours()
	if hours <= 0 {
		return nil
	}
	// Вверх: «остался 1 день» — это последние сутки. Вниз при 47 часах
	// выходило «1 день», и напоминание «за сутки» уходило за двое (PAY-7).
	days := int(math.Ceil(hours / 24))
	if days > remindDays {
		return nil
	}
	return &days
}

// PayReminderCandidate is an account due for a subscription reminder.
type PayReminderCandidate struct {
	AccountID string
	Email     string
	ExpiresAt string
	DaysLeft  int
}

// ListPayReminderCandidates returns accounts whose subscription ends within the reminder window and who were not reminded for this term.
func (s *Service) ListPayReminderCandidates(ctx context.Context, now time.Time) ([]PayReminderCandidate, error) {
	settings, err := s.loadPaySettingsRow(ctx)
	if err != nil {
		return nil, err
	}
	if !settings.SubscriptionRequired {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, email, subscription_expires_at
		FROM accounts
		WHERE email != ?
		  AND deleted_at IS NULL
		  AND blocked = 0
		  AND subscription_expires_at IS NOT NULL
		  AND subscription_expires_at > ?
		  AND (pay_reminder_sent_for IS NULL OR pay_reminder_sent_for != subscription_expires_at)
	`, AdminSentinelEmail, formatTime(now.UTC()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PayReminderCandidate
	for rows.Next() {
		var id, email, expires string
		if err := rows.Scan(&id, &email, &expires); err != nil {
			return nil, err
		}
		if days := reminderDaysLeft(expires, now, settings.SubscriptionRemindDays); days != nil {
			out = append(out, PayReminderCandidate{
				AccountID: id, Email: email, ExpiresAt: expires, DaysLeft: *days,
			})
		}
	}
	return out, rows.Err()
}

// MarkPayReminderSent records that the reminder for this expiry went out.
func (s *Service) MarkPayReminderSent(ctx context.Context, accountID, expiresAt string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE accounts SET pay_reminder_sent_for = ? WHERE id = ?
	`, expiresAt, accountID)
	return err
}
