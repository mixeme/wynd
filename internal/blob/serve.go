package blob

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ServeInfo is returned for blob download handlers.
type ServeInfo struct {
	Path        string
	MimeType    string
	SizeBytes   int64
	Filename    string
	Disposition string // always "attachment"
}

// OpenBlob returns file path and headers for serving.
func (s *Store) OpenBlob(ctx context.Context, blobID string) (ServeInfo, error) {
	var rel, mime string
	var size int64
	var originalFilename sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT storage_path, mime_type, size_bytes, original_filename FROM blobs
		WHERE id = ? AND status = 'complete'
	`, blobID).Scan(&rel, &mime, &size, &originalFilename)
	if err == sql.ErrNoRows {
		return ServeInfo{}, ErrNotFound
	}
	if err != nil {
		return ServeInfo{}, err
	}
	path := filepath.Join(s.dir, filepath.FromSlash(rel))
	if _, err := os.Stat(path); err != nil {
		return ServeInfo{}, ErrNotFound
	}
	filename := blobID
	if originalFilename.Valid && originalFilename.String != "" {
		filename = originalFilename.String
	}
	if ext, ok := executableExtension(mime); ok {
		mime = "application/octet-stream"
		if filename == blobID {
			filename += ext
		}
	}
	return ServeInfo{
		Path: path, MimeType: mime, SizeBytes: size,
		Filename: filename, Disposition: "attachment",
	}, nil
}

func executableExtension(mime string) (string, bool) {
	m := strings.ToLower(strings.TrimSpace(mime))
	if i := strings.Index(m, ";"); i >= 0 {
		m = strings.TrimSpace(m[:i])
	}
	switch m {
	case "image/svg+xml", "image/svg":
		return ".svg", true
	case "text/html", "application/xhtml+xml":
		return ".html", true
	case "text/javascript", "application/javascript", "application/x-javascript":
		return ".js", true
	}
	return "", false
}

// CanAccessBlob checks read permission via ownership or circle post reference.
func (s *Store) CanAccessBlob(ctx context.Context, accountID, blobID string) (bool, error) {
	var owner string
	err := s.db.QueryRowContext(ctx, `
		SELECT account_id FROM blobs WHERE id = ? AND status = 'complete'
	`, blobID).Scan(&owner)
	if err == sql.ErrNoRows {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	if owner == accountID {
		return true, nil
	}
	var n int
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM post_media pm
		JOIN posts p ON p.id = pm.post_id AND p.deleted = 0
		JOIN memberships m ON m.circle_id = p.circle_id AND m.account_id = ?
		JOIN membership_spans ms ON ms.membership_id = m.id AND ms.can_read = 1
		WHERE pm.blob_id = ?
		  AND p.created_at >= ms.started_at
		  AND (ms.ended_at IS NULL OR p.created_at < ms.ended_at)
	`, accountID, blobID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// AddRef records a reference to keep blob alive.
func (s *Store) AddRef(ctx context.Context, tx *sql.Tx, blobID, refType, refID string) error {
	exec := s.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}
	_, err := exec(ctx, `
		INSERT OR IGNORE INTO blob_refs (blob_id, ref_type, ref_id) VALUES (?, ?, ?)
	`, blobID, refType, refID)
	return err
}

// RemoveRefsFor removes all refs of a type for an id and GCs unreferenced blobs.
func (s *Store) RemoveRefsFor(ctx context.Context, refType, refID string) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT blob_id FROM blob_refs WHERE ref_type = ? AND ref_id = ?
	`, refType, refID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
		DELETE FROM blob_refs WHERE ref_type = ? AND ref_id = ?
	`, refType, refID); err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.gcBlobIfUnreferenced(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) gcBlobIfUnreferenced(ctx context.Context, blobID string) error {
	var n int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM blob_refs WHERE blob_id = ?
	`, blobID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	// Also check post_media and day_covers / identity_names
	if err := s.db.QueryRowContext(ctx, `
		SELECT (
			(SELECT COUNT(*) FROM post_media WHERE blob_id = ?) +
			(SELECT COUNT(*) FROM day_covers WHERE blob_id = ? AND deleted = 0) +
			(SELECT COUNT(*) FROM identity_names WHERE avatar_blob_id = ? AND erased_at IS NULL)
		)
	`, blobID, blobID, blobID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	var rel string
	err := s.db.QueryRowContext(ctx, `
		SELECT storage_path FROM blobs WHERE id = ?
	`, blobID).Scan(&rel)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	path := filepath.Join(s.dir, filepath.FromSlash(rel))
	_ = os.Remove(path)
	_, err = s.db.ExecContext(ctx, `DELETE FROM blobs WHERE id = ?`, blobID)
	return err
}

// ReleaseBlobs GCs blobs after post_media rows were removed.
func (s *Store) ReleaseBlobs(ctx context.Context, blobIDs []string) error {
	for _, id := range blobIDs {
		if err := s.gcBlobIfUnreferenced(ctx, id); err != nil {
			return fmt.Errorf("gc blob %s: %w", id, err)
		}
	}
	return nil
}
