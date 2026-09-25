package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/uid"
)

const payRequestRefType = "pay_request"

// Потолки свободного текста оплаты и срока продления (аудит 2026-09-22): без
// них комментарий заявки ограничивал только 1 МиБ тела, а days без верхней
// границы уводил дату за 9999 год, где она перестаёт разбираться.
const (
	MaxPayCommentChars    = 2000
	MaxPayRequisitesChars = 4000
	MaxPayDonateTextChars = 4000
	MaxPayGrantDays       = 3660
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
}

// PayRequest is a subscription payment proof awaiting review.
type PayRequest struct {
	ID              string  `json:"id"`
	AccountID       string  `json:"account_id"`
	AccountEmail    string  `json:"account_email,omitempty"`
	BlobID          string  `json:"blob_id,omitempty"`
	BlobFilename    string  `json:"blob_filename,omitempty"`
	BlobSizeBytes   int64   `json:"blob_size_bytes,omitempty"`
	BlobDeleted     bool    `json:"blob_deleted,omitempty"`
	Comment         *string `json:"comment,omitempty"`
	Status          string  `json:"status"`
	CreatedAt       string  `json:"created_at"`
	ResolvedAt      *string `json:"resolved_at,omitempty"`
	ExpiresAt       *string `json:"expires_at,omitempty"`
	SubscriptionEnd *string `json:"subscription_expires_at,omitempty"`
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
	return PaySubscriptionSettings{
		Required: row.SubscriptionRequired, RemindDays: row.SubscriptionRemindDays,
	}, nil
}

// SetPaySubscriptionSettings turns the subscription gate on or off; reminders go 1, 3 or 7 days ahead.
func (s *Service) SetPaySubscriptionSettings(ctx context.Context, in PaySubscriptionSettings) error {
	if in.RemindDays != 1 && in.RemindDays != 3 && in.RemindDays != 7 {
		return ErrInvalid
	}
	required := 0
	if in.Required {
		required = 1
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE instance_settings SET
			pay_subscription_required = ?, pay_subscription_remind_days = ?, updated_at = ?
		WHERE id = 1
	`, required, in.RemindDays, formatTime(time.Now().UTC()))
	return err
}

// ListPendingPayRequests returns the payment requests waiting for the admin.
func (s *Service) ListPendingPayRequests(ctx context.Context) ([]PayRequest, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT pr.id, pr.account_id, a.email, pr.blob_id, b.original_filename, b.size_bytes,
			pr.blob_deleted, pr.comment, pr.status, pr.created_at, pr.resolved_at, a.subscription_expires_at
		FROM pay_requests pr
		JOIN accounts a ON a.id = pr.account_id
		LEFT JOIN blobs b ON b.id = pr.blob_id
		WHERE pr.status = 'pending' AND a.deleted_at IS NULL
		ORDER BY pr.created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list pay requests: %w", err)
	}
	defer rows.Close()
	var out []PayRequest
	for rows.Next() {
		item, err := scanPayRequestRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// PayRequestByID returns one payment request with its account and screenshot.
func (s *Service) PayRequestByID(ctx context.Context, id string) (PayRequest, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT pr.id, pr.account_id, a.email, pr.blob_id, b.original_filename, b.size_bytes,
			pr.blob_deleted, pr.comment, pr.status, pr.created_at, pr.resolved_at, a.subscription_expires_at
		FROM pay_requests pr
		JOIN accounts a ON a.id = pr.account_id
		LEFT JOIN blobs b ON b.id = pr.blob_id
		WHERE pr.id = ?
	`, id)
	item, err := scanPayRequestRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return PayRequest{}, ErrNotFound
	}
	return item, err
}

func scanPayRequestRow(scanner interface {
	Scan(dest ...any) error
}) (PayRequest, error) {
	var item PayRequest
	var blobID, filename sql.NullString
	var size sql.NullInt64
	var blobDeleted int
	var comment, resolved, expires sql.NullString
	if err := scanner.Scan(&item.ID, &item.AccountID, &item.AccountEmail, &blobID, &filename, &size,
		&blobDeleted, &comment, &item.Status, &item.CreatedAt, &resolved, &expires); err != nil {
		return PayRequest{}, err
	}
	if blobID.Valid {
		item.BlobID = blobID.String
	}
	if filename.Valid {
		item.BlobFilename = filename.String
	}
	if size.Valid {
		item.BlobSizeBytes = size.Int64
	}
	item.BlobDeleted = blobDeleted != 0
	if comment.Valid && comment.String != "" {
		item.Comment = &comment.String
	}
	if resolved.Valid {
		item.ResolvedAt = &resolved.String
	}
	if expires.Valid {
		item.SubscriptionEnd = &expires.String
	}
	return item, nil
}

// CreatePayRequest files a participant's payment request with a screenshot; one pending request per account.
func (s *Service) CreatePayRequest(ctx context.Context, accountID, blobID, comment string) (string, error) {
	if accountID == "" || blobID == "" {
		return "", ErrInvalid
	}
	comment = strings.TrimSpace(comment)
	if utf8.RuneCountInString(comment) > MaxPayCommentChars {
		return "", ErrTooLong
	}
	settings, err := s.loadPaySettingsRow(ctx)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(settings.Requisites) == "" {
		return "", ErrInvalid
	}
	if !settings.SubscriptionRequired {
		return "", ErrInvalid
	}
	id, err := uid.NewID()
	if err != nil {
		return "", err
	}
	now := formatTime(time.Now().UTC())
	// Предусловия — внутри той же транзакции (BEGIN IMMEDIATE через DSN):
	// два параллельных POST давали две pending-заявки, и админ мог утвердить
	// обе (аудит 2026-09-22). Индекс idx_pay_requests_one_pending — страховка.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var pending int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pay_requests WHERE account_id = ? AND status = 'pending'
	`, accountID).Scan(&pending); err != nil {
		return "", err
	}
	if pending > 0 {
		return "", ErrConflict
	}
	var owner string
	err = tx.QueryRowContext(ctx, `
		SELECT account_id FROM blobs WHERE id = ? AND status = 'complete'
	`, blobID).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if owner != accountID {
		return "", ErrForbidden
	}
	// Файл скриншота отклонённой или истёкшей заявки удалён, а строка blobs
	// осталась complete: повторная заявка с тем же блобом приходила админу
	// без картинки (план 42, PAY-2).
	var fileGone int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pay_requests WHERE blob_id = ? AND blob_deleted = 1
	`, blobID).Scan(&fileGone); err != nil {
		return "", err
	}
	if fileGone > 0 {
		return "", ErrInvalid
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO pay_requests (id, account_id, blob_id, comment, status, created_at)
		VALUES (?, ?, ?, NULLIF(?, ''), 'pending', ?)
	`, id, accountID, blobID, comment, now); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return "", ErrConflict
		}
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO blob_refs (blob_id, ref_type, ref_id) VALUES (?, ?, ?)
	`, blobID, payRequestRefType, id); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
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

// ApprovePayRequest approves a pending request and extends the subscription in one transaction.
func (s *Service) ApprovePayRequest(ctx context.Context, id string, days int, unlimited bool) error {
	if id == "" {
		return ErrInvalid
	}
	if !unlimited && (days < 1 || days > MaxPayGrantDays) {
		return ErrInvalid
	}
	now := time.Now().UTC()
	resolved := formatTime(now)
	// Одна транзакция и guard по статусу: два параллельных approve продлевали
	// срок дважды, а approve поверх reject переписывал отказ (аудит 2026-09-22).
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var accountID string
	err = tx.QueryRowContext(ctx, `
		SELECT account_id FROM pay_requests WHERE id = ? AND status = 'pending'
	`, id).Scan(&accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	var next string
	if unlimited {
		next = subscriptionUnlimitedExpiry()
	} else {
		var current sql.NullString
		if err := tx.QueryRowContext(ctx, `
			SELECT subscription_expires_at FROM accounts WHERE id = ?
		`, accountID).Scan(&current); err != nil {
			return err
		}
		base := subscriptionExtendBase(current, now)
		next = formatTime(base.AddDate(0, 0, days))
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE pay_requests SET status = 'approved', resolved_at = ?
		WHERE id = ? AND status = 'pending'
	`, resolved, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	res, err = tx.ExecContext(ctx, `
		UPDATE accounts SET subscription_expires_at = ?, pay_reminder_sent_for = NULL
		WHERE id = ? AND deleted_at IS NULL
	`, next, accountID)
	if err != nil {
		return err
	}
	// Учётка удалена — утверждать нечего: заявка остаётся pending, а не
	// «утверждённой» без продления (план 42, PAY-5).
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// RejectPayRequest rejects a pending request and deletes its screenshot file at once.
//
// Статус, пометка blob_deleted и снятие ссылки — одна транзакция, файл
// удаляется после коммита: раньше это были три шага, и сбой между ними
// оставлял отклонённую заявку с живым файлом, который потом не удалял никто
// (план 42, PAY-3).
func (s *Service) RejectPayRequest(ctx context.Context, id string, blobsDir string) error {
	now := formatTime(time.Now().UTC())
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var blobID string
	err = tx.QueryRowContext(ctx, `
		SELECT blob_id FROM pay_requests WHERE id = ? AND status = 'pending'
	`, id).Scan(&blobID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE pay_requests SET status = 'rejected', resolved_at = ?
		WHERE id = ? AND status = 'pending'
	`, now, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	var rel string
	if blobID != "" {
		if rel, err = detachPayRequestBlob(ctx, tx, id, blobID); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	removeBlobFile(blobsDir, rel)
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

// PayBlobAllowedForAdmin reports whether the blob is a live payment screenshot, the only blob the panel may open.
func (s *Service) PayBlobAllowedForAdmin(ctx context.Context, blobID string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pay_requests WHERE blob_id = ? AND blob_deleted = 0
	`, blobID).Scan(&n)
	return n > 0, err
}

// CleanupExpiredPayScreenshots deletes files of approved screenshots whose subscription has expired; the blobs row stays for the foreign key.
func (s *Service) CleanupExpiredPayScreenshots(ctx context.Context, blobsDir string, now time.Time) (int, error) {
	nowRaw := formatTime(now.UTC())
	rows, err := s.db.QueryContext(ctx, `
		SELECT pr.id, pr.blob_id
		FROM pay_requests pr
		JOIN accounts a ON a.id = pr.account_id
		WHERE pr.status = 'approved' AND pr.blob_deleted = 0 AND pr.blob_id != ''
		  AND a.subscription_expires_at IS NOT NULL AND a.subscription_expires_at < ?
	`, nowRaw)
	if err != nil {
		return 0, err
	}
	type item struct{ reqID, blobID string }
	var items []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.reqID, &it.blobID); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, it)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, it := range items {
		if err := s.removePayRequestBlob(ctx, blobsDir, it.reqID, it.blobID); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *Service) removePayRequestBlob(ctx context.Context, blobsDir, reqID, blobID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rel, err := detachPayRequestBlob(ctx, tx, reqID, blobID)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	removeBlobFile(blobsDir, rel)
	return nil
}

// detachPayRequestBlob помечает скриншот заявки удалённым и снимает её ссылку
// на блоб. Возвращает путь файла, если он больше никому не нужен; удалять
// файл — после коммита, чтобы откат не оставил строку без файла.
func detachPayRequestBlob(ctx context.Context, tx *sql.Tx, reqID, blobID string) (string, error) {
	if _, err := tx.ExecContext(ctx, `
		UPDATE pay_requests SET blob_deleted = 1 WHERE id = ?
	`, reqID); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM blob_refs WHERE ref_type = ? AND ref_id = ?
	`, payRequestRefType, reqID); err != nil {
		return "", err
	}
	// Строка pay_requests остаётся (внешний ключ на blobs), поэтому спрашиваем
	// про файл: свой скриншот уже помечен blob_deleted = 1 выше.
	needed, err := blob.IsFileNeeded(ctx, tx, blobID)
	if err != nil || needed {
		return "", err
	}
	var rel string
	err = tx.QueryRowContext(ctx, `
		SELECT storage_path FROM blobs WHERE id = ?
	`, blobID).Scan(&rel)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	// Keep the blobs row: pay_requests.blob_id still references it.
	return rel, err
}

func removeBlobFile(blobsDir, rel string) {
	if blobsDir == "" || rel == "" {
		return
	}
	_ = os.Remove(filepath.Join(blobsDir, filepath.FromSlash(rel)))
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
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
	days := int(hours / 24)
	if days < 1 {
		days = 1
	}
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
