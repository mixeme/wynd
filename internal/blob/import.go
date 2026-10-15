package blob

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ImportInput — файл круга, который сервер кладёт сам, без загрузки частями:
// перенос круга с другого сервера (план 53). Сумма и длина — из blobs.json
// архива; не сошлись — ErrInvalid, файл не ложится.
type ImportInput struct {
	AccountID        string
	SHA256           string
	SizeBytes        int64
	MimeType         string
	OriginalFilename string
	Now              time.Time
}

// ImportBlob пишет файл из r в хранилище от имени учётки. Предел вложения
// здесь не действует: файл уже жил в круге на другом сервере; потолок
// сервера — действует.
func (s *Store) ImportBlob(ctx context.Context, in ImportInput, r io.Reader) (Blob, error) {
	if in.AccountID == "" || in.SizeBytes < 0 || in.SHA256 == "" {
		return Blob{}, ErrInvalid
	}
	mime := in.MimeType
	if mime == "" || len(mime) > maxMimeTypeLen {
		mime = "application/octet-stream"
	}
	if err := s.CheckMediaQuota(ctx, "", in.SizeBytes); err != nil {
		return Blob{}, err
	}
	blobID, err := newID()
	if err != nil {
		return Blob{}, err
	}
	if err := os.MkdirAll(filepath.Join(s.dir, ".uploads"), 0o750); err != nil {
		return Blob{}, err
	}
	part := filepath.Join(s.dir, ".uploads", "import-"+blobID+".part")
	f, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return Blob{}, err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(r, in.SizeBytes+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && (n != in.SizeBytes || !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), in.SHA256)) {
		err = ErrInvalid
	}
	if err != nil {
		_ = os.Remove(part)
		return Blob{}, err
	}
	sum := hex.EncodeToString(h.Sum(nil))
	rel := storageRelPath(blobID)
	if err := os.MkdirAll(filepath.Join(s.dir, rel[:2]), 0o750); err != nil {
		_ = os.Remove(part)
		return Blob{}, err
	}
	now := utcOrNow(in.Now)
	filename := sanitizeFilename(in.OriginalFilename)
	// Тот же порядок, что у загрузки (UPL-1): строка pending, файл на место,
	// complete.
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO blobs (id, account_id, sha256, size_bytes, mime_type, original_filename, storage_path, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', ?)
	`, blobID, in.AccountID, sum, in.SizeBytes, mime, nullableString(filename), rel, formatTime(now)); err != nil {
		_ = os.Remove(part)
		return Blob{}, err
	}
	dest := filepath.Join(s.dir, rel)
	if err := os.Rename(part, dest); err != nil {
		if copyErr := copyFile(part, dest); copyErr != nil {
			_, _ = s.db.ExecContext(ctx, `DELETE FROM blobs WHERE id = ? AND status = 'pending'`, blobID)
			_ = os.Remove(part)
			return Blob{}, copyErr
		}
		_ = os.Remove(part)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE blobs SET status = 'complete' WHERE id = ?`, blobID); err != nil {
		return Blob{}, err
	}
	return Blob{
		ID: blobID, AccountID: in.AccountID, SHA256: sum, SizeBytes: in.SizeBytes,
		MimeType: mime, OriginalFilename: filename, Status: "complete", CreatedAt: now,
	}, nil
}
