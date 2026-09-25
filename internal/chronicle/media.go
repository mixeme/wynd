package chronicle

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// MediaKind distinguishes photos, videos and attachments.
type MediaKind string

const (
	MediaPhoto       MediaKind = "photo"
	MediaVideo       MediaKind = "video"
	MediaAttachment  MediaKind = "attachment"
)

// MediaInput is client-provided media metadata linked to a post.
type MediaInput struct {
	BlobID     string
	Kind       MediaKind
	CapturedAt *time.Time
	GeoLat     *float64
	GeoLng     *float64
	IsCover    bool
}

// PostMedia is a row linking a blob to a post.
type PostMedia struct {
	ID               string
	PostID           string
	BlobID           string
	Kind             MediaKind
	SortOrder        int
	CapturedAt       *time.Time
	GeoLat           *float64
	GeoLng           *float64
	IsCover          bool
	OriginalFilename string
	SizeBytes        int64
}

// AttachMedia links uploaded blobs to a post. Caller must validate blob ownership.
func (c *Chronicle) AttachMedia(ctx context.Context, postID string, items []MediaInput) error {
	if len(items) == 0 {
		return nil
	}
	post, err := c.loadPost(ctx, c.db, postID)
	if err != nil {
		return err
	}
	if post.Deleted {
		return ErrInvalid
	}
	coverSet := false
	for i, item := range items {
		if item.BlobID == "" || item.Kind == "" {
			return ErrInvalid
		}
		if item.Kind != MediaPhoto && item.Kind != MediaVideo && item.Kind != MediaAttachment {
			return ErrInvalid
		}
		if item.IsCover {
			if item.Kind == MediaAttachment {
				return ErrInvalid
			}
			if coverSet {
				return ErrInvalid
			}
			coverSet = true
		}
		id, err := newID()
		if err != nil {
			return err
		}
		_, err = c.db.ExecContext(ctx, `
			INSERT INTO post_media (id, post_id, blob_id, kind, sort_order, captured_at, geo_lat, geo_lng, is_cover)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, id, postID, item.BlobID, string(item.Kind), i,
			formatCaptured(item.CapturedAt), nullableFloat(item.GeoLat), nullableFloat(item.GeoLng),
			boolToInt(item.IsCover))
		if err != nil {
			return err
		}
	}
	return nil
}

func nullableFloat(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

// PostMediaBlobIDs returns blob ids attached to a post.
func (c *Chronicle) PostMediaBlobIDs(ctx context.Context, postID string) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT blob_id FROM post_media WHERE post_id = ?`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DeletePostMedia removes media rows for a post.
func (c *Chronicle) DeletePostMedia(ctx context.Context, postID string) error {
	_, err := c.db.ExecContext(ctx, `DELETE FROM post_media WHERE post_id = ?`, postID)
	return err
}

// BlobOnPost checks blob belongs to post and is a visual medium (not attachment).
func (c *Chronicle) BlobOnPost(ctx context.Context, postID, blobID string) (bool, error) {
	var kind string
	err := c.db.QueryRowContext(ctx, `
		SELECT kind FROM post_media WHERE post_id = ? AND blob_id = ?
	`, postID, blobID).Scan(&kind)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return kind == string(MediaPhoto) || kind == string(MediaVideo), nil
}

// ListPostMedia returns media metadata for API responses.
func (c *Chronicle) ListPostMedia(ctx context.Context, postID string) ([]PostMedia, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT pm.id, pm.post_id, pm.blob_id, pm.kind, pm.sort_order, pm.captured_at, pm.geo_lat, pm.geo_lng, pm.is_cover,
			COALESCE(b.original_filename, ''), b.size_bytes
		FROM post_media pm
		JOIN blobs b ON b.id = pm.blob_id
		WHERE pm.post_id = ? ORDER BY sort_order
	`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PostMedia
	for rows.Next() {
		var m PostMedia
		var captured sql.NullString
		var lat, lng sql.NullFloat64
		var cover int
		if err := rows.Scan(&m.ID, &m.PostID, &m.BlobID, &m.Kind, &m.SortOrder,
			&captured, &lat, &lng, &cover, &m.OriginalFilename, &m.SizeBytes); err != nil {
			return nil, err
		}
		if captured.Valid {
			t, _ := parseTime(captured.String)
			m.CapturedAt = &t
		}
		if lat.Valid {
			v := lat.Float64
			m.GeoLat = &v
		}
		if lng.Valid {
			v := lng.Float64
			m.GeoLng = &v
		}
		m.IsCover = cover == 1
		out = append(out, m)
	}
	return out, rows.Err()
}

// MediaSummary is a compact media descriptor for JSON.
type MediaSummary struct {
	BlobID     string   `json:"blob_id"`
	Kind       string   `json:"kind"`
	CapturedAt *string  `json:"captured_at,omitempty"`
	GeoLat     *float64 `json:"geo_lat,omitempty"`
	GeoLng     *float64 `json:"geo_lng,omitempty"`
	IsCover    bool     `json:"is_cover"`
	Filename   *string  `json:"filename,omitempty"`
	SizeBytes  *int64   `json:"size_bytes,omitempty"`
}

// SummarizeMedia converts PostMedia rows for API output.
func SummarizeMedia(items []PostMedia) []MediaSummary {
	out := make([]MediaSummary, len(items))
	for i, m := range items {
		s := MediaSummary{
			BlobID: m.BlobID, Kind: string(m.Kind), IsCover: m.IsCover,
			GeoLat: m.GeoLat, GeoLng: m.GeoLng,
		}
		if m.CapturedAt != nil {
			raw := m.CapturedAt.UTC().Format(time.RFC3339)
			s.CapturedAt = &raw
		}
		if m.OriginalFilename != "" {
			name := m.OriginalFilename
			s.Filename = &name
		}
		if m.SizeBytes > 0 {
			size := m.SizeBytes
			s.SizeBytes = &size
		}
		out[i] = s
	}
	return out
}

// SetPostCover marks one photo or video blob as the post cover within the author's edit window.
func (c *Chronicle) SetPostCover(ctx context.Context, circleID, accountID, postID, blobID string, now time.Time) error {
	if blobID == "" {
		return ErrInvalid
	}
	now = utcOrNow(now)
	post, err := c.loadPost(ctx, c.db, postID)
	if err != nil {
		return err
	}
	if post.CircleID != circleID {
		return ErrNotFound
	}
	if post.Deleted {
		return ErrInvalid
	}
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return err
	}
	if mem.IdentityID != post.IdentityID {
		return ErrForbidden
	}
	if !post.EditWindow.CanEdit(post.CreatedAt, now) {
		return ErrForbidden
	}
	okBlob, err := c.BlobOnPost(ctx, postID, blobID)
	if err != nil {
		return err
	}
	if !okBlob {
		return ErrInvalid
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `UPDATE post_media SET is_cover = 0 WHERE post_id = ?`, postID); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE post_media SET is_cover = 1 WHERE post_id = ? AND blob_id = ?
	`, postID, blobID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrInvalid
	}
	return tx.Commit()
}

// ValidateMediaKinds ensures at least one rule: attachments are not covers.
func ValidateMediaKinds(items []MediaInput) error {
	for _, item := range items {
		if item.IsCover && item.Kind == MediaAttachment {
			return fmt.Errorf("attachment cannot be cover")
		}
	}
	return nil
}
