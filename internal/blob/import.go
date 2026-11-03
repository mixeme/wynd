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
	defer os.Remove(part)
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(r, in.SizeBytes+1))
	if err == nil {
		err = f.Sync()
	}
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
	return s.publishVerified(ctx, Blob{
		ID: blobID, AccountID: in.AccountID, SHA256: sum, SizeBytes: in.SizeBytes, MimeType: mime,
		OriginalFilename: sanitizeFilename(in.OriginalFilename), Status: "complete", CreatedAt: utcOrNow(in.Now),
	}, part, "", true)
}
