package blob

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const chunkOverhead = 1024 * 1024 // 1 MiB recommended client chunk size

// maxMimeTypeLen bounds the client-supplied MIME string stored per blob.
const maxMimeTypeLen = 255

// Blob is a completed uploaded file.
type Blob struct {
	ID               string
	AccountID        string
	SHA256           string
	SizeBytes        int64
	MimeType         string
	OriginalFilename string
	Status           string
	CreatedAt        time.Time
}

// Session is an in-progress upload.
type Session struct {
	ID               string
	AccountID        string
	ExpectedSize     int64
	MimeType         string
	OriginalFilename string
	ReceivedBytes    int64
	ExpiresAt        time.Time
}

type CreateSessionInput struct {
	AccountID        string
	ExpectedSize     int64
	MimeType         string
	OriginalFilename string
	Now              time.Time
}

// CreateSession starts a chunked upload.
func (s *Store) CreateSession(ctx context.Context, in CreateSessionInput) (Session, error) {
	if in.AccountID == "" || in.ExpectedSize <= 0 || in.MimeType == "" {
		return Session{}, ErrInvalid
	}
	if len(in.MimeType) > maxMimeTypeLen {
		return Session{}, ErrInvalid
	}
	cs, err := s.LoadCompressionSettings(ctx)
	if err != nil {
		return Session{}, err
	}
	if in.ExpectedSize > cs.AttachmentMaxBytes {
		return Session{}, ErrInvalid
	}
	if err := s.CheckMediaQuota(ctx, "", in.ExpectedSize); err != nil {
		return Session{}, err
	}
	now := utcOrNow(in.Now)
	id, err := newID()
	if err != nil {
		return Session{}, err
	}
	filename := sanitizeFilename(in.OriginalFilename)
	expires := now.Add(uploadSessionTTL * time.Second)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO upload_sessions (id, account_id, expected_size, mime_type, original_filename, received_bytes, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?)
	`, id, in.AccountID, in.ExpectedSize, in.MimeType, nullableString(filename), formatTime(expires), formatTime(now))
	if err != nil {
		return Session{}, err
	}
	if err := os.MkdirAll(filepath.Join(s.dir, ".uploads"), 0o750); err != nil {
		return Session{}, err
	}
	f, err := os.OpenFile(s.uploadPartPath(id), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return Session{}, err
	}
	return Session{
		ID: id, AccountID: in.AccountID, ExpectedSize: in.ExpectedSize,
		MimeType: in.MimeType, OriginalFilename: filename, ReceivedBytes: 0, ExpiresAt: expires,
	}, f.Close()
}

func (s *Store) uploadPartPath(sessionID string) string {
	return filepath.Join(s.dir, ".uploads", sessionID+".part")
}

func (s *Store) loadSession(ctx context.Context, sessionID, accountID string) (Session, error) {
	var sess Session
	var expires, created string
	var filename sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, expected_size, mime_type, original_filename, received_bytes, expires_at
		FROM upload_sessions WHERE id = ?
	`, sessionID).Scan(&sess.ID, &sess.AccountID, &sess.ExpectedSize, &sess.MimeType,
		&filename, &sess.ReceivedBytes, &expires)
	if err == sql.ErrNoRows {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	if accountID != "" && sess.AccountID != accountID {
		return Session{}, ErrForbidden
	}
	sess.ExpiresAt, err = parseTime(expires)
	if err != nil {
		return Session{}, err
	}
	if time.Now().UTC().After(sess.ExpiresAt) {
		return Session{}, ErrExpired
	}
	if filename.Valid {
		sess.OriginalFilename = filename.String
	}
	_ = created
	return sess, nil
}

// ErrExpired is returned when an upload session has expired.

// SessionStatus returns resume offset for HEAD.
func (s *Store) SessionStatus(ctx context.Context, sessionID, accountID string) (Session, error) {
	return s.loadSession(ctx, sessionID, accountID)
}

// WriteChunk appends bytes at offset (resume). Offset must match received_bytes.
func (s *Store) WriteChunk(ctx context.Context, sessionID, accountID string, offset int64, r io.Reader) (int64, error) {
	sess, err := s.loadSession(ctx, sessionID, accountID)
	if err != nil {
		return 0, err
	}
	if offset != sess.ReceivedBytes {
		return sess.ReceivedBytes, ErrInvalid
	}
	if offset >= sess.ExpectedSize {
		return sess.ReceivedBytes, ErrInvalid
	}

	path := s.uploadPartPath(sessionID)
	f, err := os.OpenFile(path, os.O_WRONLY, 0o640)
	if err != nil {
		return sess.ReceivedBytes, err
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return sess.ReceivedBytes, err
	}
	remaining := sess.ExpectedSize - offset
	limited := io.LimitReader(r, remaining+chunkOverhead)
	written, err := io.Copy(f, limited)
	if err != nil {
		return sess.ReceivedBytes, err
	}
	if written > remaining {
		return sess.ReceivedBytes, ErrInvalid
	}
	newTotal := offset + written
	_, err = s.db.ExecContext(ctx, `
		UPDATE upload_sessions SET received_bytes = ? WHERE id = ?
	`, newTotal, sessionID)
	if err != nil {
		return sess.ReceivedBytes, err
	}
	return newTotal, nil
}

type CompleteSessionInput struct {
	SessionID string
	AccountID string
	SHA256    string
	Now       time.Time
}

// CompleteSession finalizes upload, verifies size and hash, creates blob row.
func (s *Store) CompleteSession(ctx context.Context, in CompleteSessionInput) (Blob, error) {
	if in.SHA256 == "" {
		return Blob{}, ErrInvalid
	}
	sess, err := s.loadSession(ctx, in.SessionID, in.AccountID)
	if err != nil {
		return Blob{}, err
	}
	if sess.ReceivedBytes != sess.ExpectedSize {
		return Blob{}, ErrIncomplete
	}
	path := s.uploadPartPath(in.SessionID)
	sum, err := fileSHA256(path)
	if err != nil {
		return Blob{}, err
	}
	if !strings.EqualFold(sum, in.SHA256) {
		return Blob{}, ErrInvalid
	}

	blobID, err := newID()
	if err != nil {
		return Blob{}, err
	}
	rel := storageRelPath(blobID)
	destDir := filepath.Join(s.dir, rel[:2])
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return Blob{}, err
	}
	dest := filepath.Join(s.dir, rel)
	if err := copyFile(path, dest); err != nil {
		return Blob{}, err
	}
	_ = os.Remove(path)

	now := utcOrNow(in.Now)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Blob{}, err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO blobs (id, account_id, sha256, size_bytes, mime_type, original_filename, storage_path, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'complete', ?)
	`, blobID, in.AccountID, sum, sess.ExpectedSize, sess.MimeType, nullableString(sess.OriginalFilename), rel, formatTime(now))
	if err != nil {
		return Blob{}, err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM upload_sessions WHERE id = ?`, in.SessionID)
	if err != nil {
		return Blob{}, err
	}
	if err := tx.Commit(); err != nil {
		return Blob{}, err
	}
	return Blob{
		ID: blobID, AccountID: in.AccountID, SHA256: sum,
		SizeBytes: sess.ExpectedSize, MimeType: sess.MimeType,
		OriginalFilename: sess.OriginalFilename,
		Status: "complete", CreatedAt: now,
	}, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// LoadBlob returns metadata for a complete blob.
func (s *Store) LoadBlob(ctx context.Context, blobID string) (Blob, error) {
	var b Blob
	var created string
	var filename sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, sha256, size_bytes, mime_type, original_filename, status, created_at
		FROM blobs WHERE id = ?
	`, blobID).Scan(&b.ID, &b.AccountID, &b.SHA256, &b.SizeBytes, &b.MimeType, &filename, &b.Status, &created)
	if err == sql.ErrNoRows {
		return Blob{}, ErrNotFound
	}
	if err != nil {
		return Blob{}, err
	}
	if filename.Valid {
		b.OriginalFilename = filename.String
	}
	b.CreatedAt, err = parseTime(created)
	return b, err
}

func nullableString(v string) any {
	if v == "" {
		return nil
	}
	return v
}

// ValidateOwnedComplete checks blob exists, is complete, and owned by account.
func (s *Store) ValidateOwnedComplete(ctx context.Context, accountID string, blobIDs []string) error {
	for _, id := range blobIDs {
		b, err := s.LoadBlob(ctx, id)
		if err != nil {
			return err
		}
		if b.AccountID != accountID {
			return ErrForbidden
		}
		if b.Status != "complete" {
			return ErrIncomplete
		}
	}
	return nil
}
