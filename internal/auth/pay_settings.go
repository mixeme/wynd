package auth

// Настройки оплаты инстанса: реквизиты, баннер пожертвований, шлюз подписки.

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Потолки свободного текста оплаты и срока продления (аудит 2026-09-22): без
// них комментарий заявки ограничивал только 1 МиБ тела, а days без верхней
// границы уводил дату за 9999 год, где она перестаёт разбираться.
const (
	MaxPayCommentChars    = 2000
	MaxPayRequisitesChars = 4000
	MaxPayDonateTextChars = 4000
	MaxPayGrantDays       = 3660
)

// PaySettings is instance-level payment configuration.
type PaySettings struct {
	Requisites             string `json:"requisites"`
	DonateText             string `json:"text"`
	DonateShow             bool   `json:"show"`
	DonateDismissible      bool   `json:"dismissible"`
	DonateUntil            string `json:"until"`
	DonateVersion          int    `json:"donate_version"`
	SubscriptionRequired   bool   `json:"required"`
	SubscriptionRemindDays int    `json:"remind_days"`
	PendingRequestCount    int    `json:"pending_count,omitempty"`
}

// PayDonateSettings is the donate banner subset.
type PayDonateSettings struct {
	Text        string `json:"text"`
	Show        bool   `json:"show"`
	Dismissible bool   `json:"dismissible"`
	Until       string `json:"until"`
}

// PaySubscriptionSettings is the subscription gate subset.
type PaySubscriptionSettings struct {
	Required   bool `json:"required"`
	RemindDays int  `json:"remind_days"`
	// GrantDays — переходный срок при включении (2026-10-21): всем, у кого
	// сейчас нет подписки, выдаётся столько дней, как «продлить без заявки».
	// 0 — не выдавать. Учитывается только в момент включения.
	GrantDays int `json:"grant_days,omitempty"`
	// GrantUnlimited — вместо дней выдать всем бессрочную подписку.
	GrantUnlimited bool `json:"grant_unlimited,omitempty"`
	// WithoutSubscription — сколько учёток сейчас без подписки (только ответ).
	WithoutSubscription int `json:"without_subscription"`
}

// PayBannerState is an active donate banner for the street.
type PayBannerState struct {
	Text        string `json:"text"`
	Dismissible bool   `json:"dismissible"`
}

func (s *Service) loadPaySettingsRow(ctx context.Context) (PaySettings, error) {
	var row PaySettings
	var show, dismissible, required int
	var until sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT pay_requisites, pay_donate_text, pay_donate_show, pay_donate_dismissible,
			pay_donate_until, pay_donate_version, pay_subscription_required, pay_subscription_remind_days
		FROM instance_settings WHERE id = 1
	`).Scan(&row.Requisites, &row.DonateText, &show, &dismissible, &until, &row.DonateVersion,
		&required, &row.SubscriptionRemindDays)
	if err != nil {
		return PaySettings{}, fmt.Errorf("load pay settings: %w", err)
	}
	row.DonateShow = show != 0
	row.DonateDismissible = dismissible != 0
	row.SubscriptionRequired = required != 0
	if until.Valid {
		row.DonateUntil = until.String
	}
	return row, nil
}

// PayHubSettings returns the payment settings with the number of pending requests, for the admin hub.
func (s *Service) PayHubSettings(ctx context.Context) (PaySettings, error) {
	row, err := s.loadPaySettingsRow(ctx)
	if err != nil {
		return PaySettings{}, err
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pay_requests pr
		JOIN accounts a ON a.id = pr.account_id AND a.deleted_at IS NULL
		WHERE pr.status = 'pending'
	`).Scan(&row.PendingRequestCount); err != nil {
		return PaySettings{}, err
	}
	return row, nil
}

// SetPayRequisites stores the payment details shown to participants.
func (s *Service) SetPayRequisites(ctx context.Context, requisites string) error {
	requisites = strings.TrimSpace(requisites)
	if utf8.RuneCountInString(requisites) > MaxPayRequisitesChars {
		return ErrTooLong
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE instance_settings SET pay_requisites = ?, updated_at = ? WHERE id = 1
	`, requisites, formatTime(time.Now().UTC()))
	return err
}

// PayDonateSettings returns the donation banner settings.
func (s *Service) PayDonateSettings(ctx context.Context) (PayDonateSettings, error) {
	row, err := s.loadPaySettingsRow(ctx)
	if err != nil {
		return PayDonateSettings{}, err
	}
	return PayDonateSettings{
		Text: row.DonateText, Show: row.DonateShow,
		Dismissible: row.DonateDismissible, Until: row.DonateUntil,
	}, nil
}

// SetPayDonateSettings stores the donation banner; its version grows only when a shown banner changes or is turned on, so dismissals survive other saves.
func (s *Service) SetPayDonateSettings(ctx context.Context, in PayDonateSettings) error {
	current, err := s.loadPaySettingsRow(ctx)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(in.Text)
	if utf8.RuneCountInString(text) > MaxPayDonateTextChars {
		return ErrTooLong
	}
	until := strings.TrimSpace(in.Until)
	if until != "" {
		if _, ok := parseDonateUntil(until); !ok {
			return ErrInvalid
		}
	}
	show := 0
	if in.Show {
		show = 1
	}
	dismissible := 0
	if in.Dismissible {
		dismissible = 1
	}
	version := current.DonateVersion
	contentChanged := text != current.DonateText ||
		dismissible != boolToInt(current.DonateDismissible) ||
		until != current.DonateUntil
	turnedOn := in.Show && !current.DonateShow
	if in.Show && (contentChanged || turnedOn) {
		version++
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE instance_settings SET
			pay_donate_text = ?, pay_donate_show = ?, pay_donate_dismissible = ?,
			pay_donate_until = NULLIF(?, ''), pay_donate_version = ?, updated_at = ?
		WHERE id = 1
	`, text, show, dismissible, until, version, formatTime(time.Now().UTC()))
	return err
}

// PaySubscriptionSettings returns whether a subscription is required and how early to remind.
func (s *Service) PaySubscriptionSettings(ctx context.Context) (PaySubscriptionSettings, error) {
	row, err := s.loadPaySettingsRow(ctx)
	if err != nil {
		return PaySubscriptionSettings{}, err
	}
	without, err := s.countWithoutSubscription(ctx, time.Now().UTC())
	if err != nil {
		return PaySubscriptionSettings{}, err
	}
	return PaySubscriptionSettings{
		Required: row.SubscriptionRequired, RemindDays: row.SubscriptionRemindDays,
		WithoutSubscription: without,
	}, nil
}

// SetPaySubscriptionSettings turns the subscription gate on or off; reminders go 1, 3 or 7 days ahead.
func (s *Service) SetPaySubscriptionSettings(ctx context.Context, in PaySubscriptionSettings) error {
	if in.RemindDays != 1 && in.RemindDays != 3 && in.RemindDays != 7 {
		return ErrInvalid
	}
	if in.GrantDays < 0 || in.GrantDays > MaxPayGrantDays {
		return ErrInvalid
	}
	required := 0
	if in.Required {
		required = 1
	}
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var was bool
	if err := tx.QueryRowContext(ctx, `SELECT pay_subscription_required FROM instance_settings WHERE id = 1`).Scan(&was); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE instance_settings SET
			pay_subscription_required = ?, pay_subscription_remind_days = ?, updated_at = ?
		WHERE id = 1
	`, required, in.RemindDays, formatTime(now)); err != nil {
		return err
	}
	// Переходный срок — только при включении: иначе повторное сохранение
	// напоминания снова раздало бы дни тем, у кого подписка кончилась.
	if in.Required && !was && (in.GrantDays > 0 || in.GrantUnlimited) {
		if err := grantWithoutSubscription(ctx, tx, in.GrantDays, in.GrantUnlimited, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// withoutSubscription — учётки без действующей подписки: срока не было или он
// прошёл. Бессрочная и идущая подписки не трогаются.
func withoutSubscription(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, now time.Time) ([]string, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, subscription_expires_at FROM accounts
		WHERE email != ? AND deleted_at IS NULL
	`, AdminSentinelEmail)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		var expires sql.NullString
		if err := rows.Scan(&id, &expires); err != nil {
			return nil, err
		}
		if expires.Valid && expires.String != "" {
			if t, err := parseTime(expires.String); err == nil && t.After(now) {
				continue
			}
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Service) countWithoutSubscription(ctx context.Context, now time.Time) (int, error) {
	ids, err := withoutSubscription(ctx, s.db, now)
	return len(ids), err
}

func grantWithoutSubscription(ctx context.Context, tx *sql.Tx, days int, unlimited bool, now time.Time) error {
	ids, err := withoutSubscription(ctx, tx, now)
	if err != nil {
		return err
	}
	until := formatTime(now.AddDate(0, 0, days))
	if unlimited {
		until = subscriptionUnlimitedExpiry()
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `
			UPDATE accounts SET subscription_expires_at = ?, pay_reminder_sent_for = NULL WHERE id = ?
		`, until, id); err != nil {
			return err
		}
	}
	return nil
}

// DismissPayBanner hides the current donation banner version for the account.
func (s *Service) DismissPayBanner(ctx context.Context, accountID string) error {
	settings, err := s.loadPaySettingsRow(ctx)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE accounts SET pay_donate_dismissed_version = ? WHERE id = ?
	`, settings.DonateVersion, accountID)
	return err
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func donateBannerActive(settings PaySettings, now time.Time) bool {
	if !settings.DonateShow || strings.TrimSpace(settings.DonateText) == "" {
		return false
	}
	if settings.DonateUntil == "" {
		return true
	}
	end, ok := parseDonateUntil(settings.DonateUntil)
	if !ok {
		return true
	}
	return now.UTC().Before(end)
}

func parseDonateUntil(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if t, err := parseTime(raw); err == nil {
		return t, true
	}
	t, err := time.ParseInLocation("2006-01-02", raw, time.UTC)
	if err != nil {
		return time.Time{}, false
	}
	return t.AddDate(0, 0, 1), true
}
