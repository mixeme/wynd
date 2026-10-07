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
	case "text/xml", "application/xml", "text/xsl", "application/xslt+xml":
		// XML с xhtml-namespace браузер отрисовывает как документ со
		// скриптами (аудит 2026-09-22).
		return ".xml", true
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
		WHERE (pm.blob_id = ? OR pm.audio_cover_blob_id = ? OR pm.video_poster_blob_id = ?)
		  AND p.created_at >= ms.started_at
		  AND (ms.ended_at IS NULL OR p.created_at < ms.ended_at)
	`, accountID, blobID, blobID, blobID).Scan(&n)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	// Вложение комментария видит тот, кто видит и запись, и сам комментарий.
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM comment_media cmm
		JOIN comments c ON c.id = cmm.comment_id AND c.deleted = 0
		JOIN posts p ON p.id = c.post_id AND p.deleted = 0
		JOIN memberships m ON m.circle_id = p.circle_id AND m.account_id = ?
		JOIN membership_spans ms ON ms.membership_id = m.id AND ms.can_read = 1
		WHERE cmm.blob_id = ?
		  AND p.created_at >= ms.started_at
		  AND (ms.ended_at IS NULL OR p.created_at < ms.ended_at)
		  AND EXISTS (
		    SELECT 1 FROM membership_spans cs
		    WHERE cs.membership_id = m.id AND cs.can_read = 1
		      AND c.created_at >= cs.started_at
		      AND (cs.ended_at IS NULL OR c.created_at < cs.ended_at)
		  )
	`, accountID, blobID).Scan(&n)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM identity_names inm
		JOIN identities ident ON ident.id = inm.identity_id
		JOIN memberships m ON m.circle_id = ident.circle_id AND m.account_id = ?
		JOIN membership_spans ms ON ms.membership_id = m.id AND ms.can_read = 1
		WHERE inm.avatar_blob_id = ? AND inm.erased_at IS NULL
		  AND (ms.ended_at IS NULL OR ident.created_at < ms.ended_at)
	`, accountID, blobID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *Store) gcBlobIfUnreferenced(ctx context.Context, blobID string) error {
	referenced, err := IsReferenced(ctx, s.db, blobID)
	if err != nil {
		return err
	}
	if referenced {
		return nil
	}
	var rel string
	err = s.db.QueryRowContext(ctx, `
		SELECT storage_path FROM blobs WHERE id = ?
	`, blobID).Scan(&rel)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	// Сначала строка, потом файл (план 42, раздел B «Ссылки на блобы»).
	if _, err := s.db.ExecContext(ctx, `DELETE FROM blobs WHERE id = ?`, blobID); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(s.dir, filepath.FromSlash(rel)))
	return nil
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
