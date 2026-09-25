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

	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/uid"
)

const payRequestRefType = "pay_request"

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

func (s *Service) setAccountSubscriptionExpires(ctx context.Context, accountID, expires string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE accounts SET subscription_expires_at = ?, pay_reminder_sent_for = NULL
		WHERE id = ? AND deleted_at IS NULL
	`, expires, accountID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
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

func (s *Service) PayHubSettings(ctx context.Context) (PaySettings, error) {
	row, err := s.loadPaySettingsRow(ctx)
	if err != nil {
		return PaySettings{}, err
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pay_requests WHERE status = 'pending'
	`).Scan(&row.PendingRequestCount); err != nil {
		return PaySettings{}, err
	}
	return row, nil
}

func (s *Service) SetPayRequisites(ctx context.Context, requisites string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE instance_settings SET pay_requisites = ?, updated_at = ? WHERE id = 1
	`, strings.TrimSpace(requisites), formatTime(time.Now().UTC()))
	return err
}

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

func (s *Service) SetPayDonateSettings(ctx context.Context, in PayDonateSettings) error {
	current, err := s.loadPaySettingsRow(ctx)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(in.Text)
	until := strings.TrimSpace(in.Until)
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

func (s *Service) PaySubscriptionSettings(ctx context.Context) (PaySubscriptionSettings, error) {
	row, err := s.loadPaySettingsRow(ctx)
	if err != nil {
		return PaySubscriptionSettings{}, err
	}
	return PaySubscriptionSettings{
		Required: row.SubscriptionRequired, RemindDays: row.SubscriptionRemindDays,
	}, nil
}

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

func (s *Service) ListPendingPayRequests(ctx context.Context) ([]PayRequest, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT pr.id, pr.account_id, a.email, pr.blob_id, b.original_filename, b.size_bytes,
			pr.blob_deleted, pr.comment, pr.status, pr.created_at, pr.resolved_at, a.subscription_expires_at
		FROM pay_requests pr
		JOIN accounts a ON a.id = pr.account_id
		LEFT JOIN blobs b ON b.id = pr.blob_id
		WHERE pr.status = 'pending'
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

func (s *Service) CreatePayRequest(ctx context.Context, accountID, blobID, comment string) (string, error) {
	if accountID == "" || blobID == "" {
		return "", ErrInvalid
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
	var pending int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pay_requests WHERE account_id = ? AND status = 'pending'
	`, accountID).Scan(&pending); err != nil {
		return "", err
	}
	if pending > 0 {
		return "", ErrConflict
	}
	var owner string
	err = s.db.QueryRowContext(ctx, `
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
	id, err := uid.NewID()
	if err != nil {
		return "", err
	}
	now := formatTime(time.Now().UTC())
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO pay_requests (id, account_id, blob_id, comment, status, created_at)
		VALUES (?, ?, ?, NULLIF(?, ''), 'pending', ?)
	`, id, accountID, blobID, strings.TrimSpace(comment), now); err != nil {
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

func (s *Service) GrantPayAccount(ctx context.Context, accountID string, days int, unlimited bool) error {
	if err := s.requirePaySubscription(ctx); err != nil {
		return err
	}
	if accountID == "" {
		return ErrInvalid
	}
	acc, err := s.PayAccountByID(ctx, accountID)
	if err != nil {
		return err
	}
	var next string
	if unlimited {
		next = subscriptionUnlimitedExpiry()
	} else {
		if days < 1 {
			return ErrInvalid
		}
		var current sql.NullString
		if acc.SubscriptionExpiresAt != nil {
			current = sql.NullString{String: *acc.SubscriptionExpiresAt, Valid: true}
		}
		base := subscriptionExtendBase(current, time.Now().UTC())
		next = formatTime(base.AddDate(0, 0, days))
	}
	return s.setAccountSubscriptionExpires(ctx, accountID, next)
}

func (s *Service) ApprovePayRequest(ctx context.Context, id string, days int, unlimited bool) error {
	if id == "" {
		return ErrInvalid
	}
	if !unlimited && days < 1 {
		return ErrInvalid
	}
	var accountID string
	err := s.db.QueryRowContext(ctx, `
		SELECT account_id FROM pay_requests WHERE id = ? AND status = 'pending'
	`, id).Scan(&accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	var next string
	if unlimited {
		next = subscriptionUnlimitedExpiry()
	} else {
		var current sql.NullString
		if err := s.db.QueryRowContext(ctx, `
			SELECT subscription_expires_at FROM accounts WHERE id = ?
		`, accountID).Scan(&current); err != nil {
			return err
		}
		base := subscriptionExtendBase(current, now)
		next = formatTime(base.AddDate(0, 0, days))
	}
	resolved := formatTime(now)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		UPDATE pay_requests SET status = 'approved', resolved_at = ? WHERE id = ?
	`, resolved, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE accounts SET subscription_expires_at = ?, pay_reminder_sent_for = NULL WHERE id = ?
	`, next, accountID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) RejectPayRequest(ctx context.Context, id string, blobsDir string) error {
	now := formatTime(time.Now().UTC())
	var blobID string
	err := s.db.QueryRowContext(ctx, `
		SELECT blob_id FROM pay_requests WHERE id = ? AND status = 'pending'
	`, id).Scan(&blobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE pay_requests SET status = 'rejected', resolved_at = ?
		WHERE id = ? AND status = 'pending'
	`, now, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	if blobsDir != "" && blobID != "" {
		return removePayRequestBlob(ctx, s.db, blobsDir, id, blobID)
	}
	return nil
}

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

func (s *Service) PayBlobAllowedForAdmin(ctx context.Context, blobID string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pay_requests WHERE blob_id = ? AND blob_deleted = 0
	`, blobID).Scan(&n)
	return n > 0, err
}

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
		if err := removePayRequestBlob(ctx, s.db, blobsDir, it.reqID, it.blobID); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func removePayRequestBlob(ctx context.Context, db *sql.DB, blobsDir, reqID, blobID string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		UPDATE pay_requests SET blob_deleted = 1 WHERE id = ?
	`, reqID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM blob_refs WHERE ref_type = ? AND ref_id = ?
	`, payRequestRefType, reqID); err != nil {
		return err
	}
	// Строка pay_requests остаётся (внешний ключ на blobs), поэтому спрашиваем
	// про файл: свой скриншот уже помечен blob_deleted = 1 выше.
	needed, err := blob.IsFileNeeded(ctx, tx, blobID)
	if err != nil {
		return err
	}
	if !needed {
		var rel string
		err = tx.QueryRowContext(ctx, `
			SELECT storage_path FROM blobs WHERE id = ?
		`, blobID).Scan(&rel)
		if err == nil && blobsDir != "" {
			_ = os.Remove(filepath.Join(blobsDir, filepath.FromSlash(rel)))
		} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		// Keep the blobs row: pay_requests.blob_id still references it.
	}
	return tx.Commit()
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

func (s *Service) MarkPayReminderSent(ctx context.Context, accountID, expiresAt string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE accounts SET pay_reminder_sent_for = ? WHERE id = ?
	`, expiresAt, accountID)
	return err
}
