package blob

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"time"
)

// PutGenerated кладёт файл, который сделал сам сервер (кадр ролика), блобом
// учётки accountID: её блобом он и должен быть, чтобы пройти те же проверки
// владения и учёта, что и присланный клиентом. Потолок хранилища не
// проверяется: это десятки килобайт к уже принятому ролику.
func (s *Store) PutGenerated(ctx context.Context, accountID, mimeType, filename string, data []byte) (Blob, error) {
	if accountID == "" || len(data) == 0 {
		return Blob{}, ErrInvalid
	}
	blobID, err := newID()
	if err != nil {
		return Blob{}, err
	}
	rel := storageRelPath(blobID)
	if err := os.MkdirAll(filepath.Join(s.dir, rel[:2]), 0o750); err != nil {
		return Blob{}, err
	}
	dest := filepath.Join(s.dir, rel)
	sum := sha256.Sum256(data)
	now := utcOrNow(time.Time{})

	// Порядок как в CompleteSession: строка pending, файл, complete — сбой
	// посередине не оставляет безымянного файла.
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO blobs (id, account_id, sha256, size_bytes, mime_type, original_filename, storage_path, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', ?)
	`, blobID, accountID, hex.EncodeToString(sum[:]), len(data), mimeType, nullableString(filename), rel, formatTime(now)); err != nil {
		return Blob{}, err
	}
	if err := writeFileSync(dest, data); err != nil {
		_ = os.Remove(dest)
		_, _ = s.db.ExecContext(ctx, `DELETE FROM blobs WHERE id = ? AND status = 'pending'`, blobID)
		return Blob{}, err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE blobs SET status = 'complete' WHERE id = ?`, blobID); err != nil {
		return Blob{}, err
	}
	return Blob{
		ID: blobID, AccountID: accountID, SHA256: hex.EncodeToString(sum[:]),
		SizeBytes: int64(len(data)), MimeType: mimeType, OriginalFilename: filename,
		Status: "complete", CreatedAt: now,
	}, nil
}

func writeFileSync(path string, data []byte) error {
	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, bytes.NewReader(data)); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
