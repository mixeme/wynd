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
	if err := os.MkdirAll(filepath.Join(s.dir, ".uploads"), 0o750); err != nil {
		return Blob{}, err
	}
	part := filepath.Join(s.dir, ".uploads", "generated-"+blobID+".part")
	defer os.Remove(part)
	if err := writeFileSync(part, data); err != nil {
		return Blob{}, err
	}
	sum := sha256.Sum256(data)
	return s.publishVerified(ctx, Blob{ID: blobID, AccountID: accountID, SHA256: hex.EncodeToString(sum[:]), SizeBytes: int64(len(data)), MimeType: mimeType, OriginalFilename: sanitizeFilename(filename), Status: "complete", CreatedAt: utcOrNow(time.Time{})}, part, "", false)
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
