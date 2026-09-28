package auth

// Заявки на оплату со скриншотом: подача, утверждение, отказ, уборка файлов.

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
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
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
