package chronicle

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// MediaKind distinguishes photos, videos and attachments.
type MediaKind string

const (
	MediaPhoto      MediaKind = "photo"
	MediaVideo      MediaKind = "video"
	MediaAttachment MediaKind = "attachment"
)

// MediaInput is client-provided media metadata linked to a post.
type MediaInput struct {
	BlobID     string
	Kind       MediaKind
	CapturedAt *time.Time
	GeoLat     *float64
	GeoLng     *float64
	IsCover    bool
	// AudioArtist, AudioTitle и AudioCoverBlobID — только у вложения-звука.
	// Обложка — отдельный JPEG, не снимок записи.
	AudioArtist      string
	AudioTitle       string
	AudioCoverBlobID string
	// VideoPosterBlobID — JPEG первого кадра ролика. Только у kind=video.
	VideoPosterBlobID string
	// Voice — голосовое, записанное в приложении (C14): лента рисует его
	// волной AudioPeaks (до 64 уровней 0–100) и длительностью.
	Voice           bool
	AudioDurationMs int64
	AudioPeaks      []int
	// Crop — кадр обложки для ленты (4.16); nil — по центру.
	Crop *CoverCrop
}

// CoverCrop — квадрат снимка в долях его ширины и высоты.
type CoverCrop struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// validCoverCrop — кадр внутри снимка и не пустой.
func validCoverCrop(c *CoverCrop) bool {
	const eps = 1e-6
	for _, v := range []float64{c.X, c.Y, c.W, c.H} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return c.X >= -eps && c.Y >= -eps && c.W > 0 && c.H > 0 &&
		c.X+c.W <= 1+eps && c.Y+c.H <= 1+eps
}

// PostMedia is a row linking a blob to a post.
type PostMedia struct {
	ID                string
	PostID            string
	BlobID            string
	Kind              MediaKind
	SortOrder         int
	CapturedAt        *time.Time
	GeoLat            *float64
	GeoLng            *float64
	IsCover           bool
	OriginalFilename  string
	SizeBytes         int64
	MimeType          string
	AudioArtist       string
	AudioTitle        string
	AudioCoverBlobID  string
	VideoPosterBlobID string
	Voice             bool
	AudioDurationMs   int64
	AudioPeaks        []int
	Crop              *CoverCrop
}

// AttachMedia links uploaded blobs to a post. Caller must validate blob ownership.
func (c *Chronicle) AttachMedia(ctx context.Context, postID string, items []MediaInput) error {
	return c.attachMedia(ctx, c.db, postID, items)
}

// AttachMediaInTx is like AttachMedia but uses an existing transaction.
func (c *Chronicle) AttachMediaInTx(ctx context.Context, tx *sql.Tx, postID string, items []MediaInput) error {
	return c.attachMedia(ctx, tx, postID, items)
}

func (c *Chronicle) attachMedia(ctx context.Context, q dbtx, postID string, items []MediaInput) error {
	if len(items) == 0 {
		return nil
	}
	post, err := c.loadPost(ctx, q, postID)
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
		if err := normalizeVoice(&item); err != nil {
			return err
		}
		if err := normalizeAudioTags(&item); err != nil {
			return err
		}
		if err := normalizeVideoPoster(&item); err != nil {
			return err
		}
		if item.Crop != nil && (item.Kind == MediaAttachment || !validCoverCrop(item.Crop)) {
			return ErrInvalid
		}
		items[i] = item
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
		_, err = q.ExecContext(ctx, `
			INSERT INTO post_media (id, post_id, blob_id, kind, sort_order, captured_at, geo_lat, geo_lng, is_cover,
				audio_artist, audio_title, audio_cover_blob_id, crop_x, crop_y, crop_w, crop_h,
				voice, audio_duration_ms, audio_peaks, video_poster_blob_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?,
				?, NULLIF(?, 0), NULLIF(?, ''), NULLIF(?, ''))
		`, id, postID, item.BlobID, string(item.Kind), i,
			formatCaptured(item.CapturedAt), nullableFloat(item.GeoLat), nullableFloat(item.GeoLng),
			boolToInt(item.IsCover), item.AudioArtist, item.AudioTitle, item.AudioCoverBlobID,
			cropArg(item.Crop, 0), cropArg(item.Crop, 1), cropArg(item.Crop, 2), cropArg(item.Crop, 3),
			boolToInt(item.Voice), item.AudioDurationMs, joinPeaks(item.AudioPeaks), item.VideoPosterBlobID)
		if err != nil {
			return err
		}
	}
	return nil
}

func cropArg(c *CoverCrop, i int) any {
	if c == nil {
		return nil
	}
	return [4]float64{c.X, c.Y, c.W, c.H}[i]
}

func nullableFloat(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

// MediaBlobIDs lists the file and, when present, its cover art. The cover
// is not a second media row, but quota and ownership checks must see it.
func MediaBlobIDs(items []MediaInput) []string {
	seen := make(map[string]struct{}, len(items)*2)
	var out []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, item := range items {
		add(item.BlobID)
		add(item.AudioCoverBlobID)
		add(item.VideoPosterBlobID)
	}
	return out
}

// MaxVoicePeaks — уровней волны голосового; MaxVoiceDurationMs — запас над
// пределом записи в 15 минут.
const (
	MaxVoicePeaks      = 64
	MaxVoiceDurationMs = 16 * 60 * 1000
)

func normalizeVoice(item *MediaInput) error {
	if !item.Voice {
		item.AudioDurationMs = 0
		item.AudioPeaks = nil
		return nil
	}
	if item.Kind != MediaAttachment || len(item.AudioPeaks) > MaxVoicePeaks {
		return ErrInvalid
	}
	if item.AudioDurationMs < 0 || item.AudioDurationMs > MaxVoiceDurationMs {
		return ErrInvalid
	}
	for _, p := range item.AudioPeaks {
		if p < 0 || p > 100 {
			return ErrInvalid
		}
	}
	return nil
}

func joinPeaks(peaks []int) string {
	parts := make([]string, len(peaks))
	for i, p := range peaks {
		parts[i] = strconv.Itoa(p)
	}
	return strings.Join(parts, ",")
}

func splitPeaks(raw string) []int {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		if n, err := strconv.Atoi(p); err == nil {
			out = append(out, n)
		}
	}
	return out
}

func normalizeAudioTags(item *MediaInput) error {
	item.AudioArtist = strings.TrimSpace(item.AudioArtist)
	item.AudioTitle = strings.TrimSpace(item.AudioTitle)
	item.AudioCoverBlobID = strings.TrimSpace(item.AudioCoverBlobID)
	if item.AudioArtist == "" && item.AudioTitle == "" && item.AudioCoverBlobID == "" {
		return nil
	}
	if item.Kind != MediaAttachment {
		return ErrInvalid
	}
	if item.AudioCoverBlobID != "" && item.AudioCoverBlobID == item.BlobID {
		return ErrInvalid
	}
	if err := checkLen(item.AudioArtist, MaxAudioTagChars); err != nil {
		return err
	}
	return checkLen(item.AudioTitle, MaxAudioTagChars)
}

func normalizeVideoPoster(item *MediaInput) error {
	item.VideoPosterBlobID = strings.TrimSpace(item.VideoPosterBlobID)
	if item.VideoPosterBlobID == "" {
		return nil
	}
	if item.Kind != MediaVideo || item.VideoPosterBlobID == item.BlobID {
		return ErrInvalid
	}
	return nil
}

const postMediaBlobIDsSQL = `
	SELECT blob_id FROM post_media WHERE post_id = ? AND blob_id != ''
	UNION
	SELECT audio_cover_blob_id FROM post_media
	 WHERE post_id = ? AND audio_cover_blob_id IS NOT NULL AND audio_cover_blob_id != ''
	UNION
	SELECT video_poster_blob_id FROM post_media
	 WHERE post_id = ? AND video_poster_blob_id IS NOT NULL AND video_poster_blob_id != ''
`

// PostMediaBlobIDs returns blob ids attached to a post, including audio covers.
func (c *Chronicle) PostMediaBlobIDs(ctx context.Context, postID string) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, postMediaBlobIDsSQL, postID, postID, postID)
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

func (c *Chronicle) deletePostMediaInTx(ctx context.Context, tx *sql.Tx, postID string) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM post_media WHERE post_id = ?`, postID)
	return err
}

func (c *Chronicle) postMediaBlobIDsInTx(ctx context.Context, tx *sql.Tx, postID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, postMediaBlobIDsSQL, postID, postID, postID)
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

// ReplacePostMediaInTx replaces all media on a post. Returns blob ids removed from the post.
func (c *Chronicle) ReplacePostMediaInTx(ctx context.Context, tx *sql.Tx, circleID, postID, entryDate string, items []MediaInput) (removed []string, err error) {
	oldIDs, err := c.postMediaBlobIDsInTx(ctx, tx, postID)
	if err != nil {
		return nil, err
	}
	if err := c.deletePostMediaInTx(ctx, tx, postID); err != nil {
		return nil, err
	}
	if len(items) > 0 {
		if err := c.attachMedia(ctx, tx, postID, items); err != nil {
			return nil, err
		}
	}
	newSet := make(map[string]struct{}, len(items))
	for _, id := range MediaBlobIDs(items) {
		newSet[id] = struct{}{}
	}
	for _, id := range oldIDs {
		if _, ok := newSet[id]; !ok {
			removed = append(removed, id)
			if err := c.reconcileDayCoverAfterBlobRemovedFromPost(ctx, tx, circleID, entryDate, postID, id); err != nil {
				return nil, err
			}
		}
	}
	return removed, nil
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

// DayCoverBlobOnPost checks blob can be a day cover: a photo or video of the
// post, or the cover art of one of its audio attachments.
func (c *Chronicle) DayCoverBlobOnPost(ctx context.Context, postID, blobID string) (bool, error) {
	ok, err := c.BlobOnPost(ctx, postID, blobID)
	if err != nil || ok {
		return ok, err
	}
	var n int
	err = c.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM post_media
		WHERE post_id = ? AND audio_cover_blob_id = ?
	`, postID, blobID).Scan(&n)
	return n > 0, err
}

const postMediaSelect = `
	SELECT pm.id, pm.post_id, pm.blob_id, pm.kind, pm.sort_order, pm.captured_at, pm.geo_lat, pm.geo_lng, pm.is_cover,
		COALESCE(b.original_filename, ''), b.size_bytes, COALESCE(b.mime_type, ''),
		COALESCE(pm.audio_artist, ''), COALESCE(pm.audio_title, ''), COALESCE(pm.audio_cover_blob_id, ''),
		pm.crop_x, pm.crop_y, pm.crop_w, pm.crop_h,
		pm.voice, COALESCE(pm.audio_duration_ms, 0), COALESCE(pm.audio_peaks, ''),
		COALESCE(pm.video_poster_blob_id, '')
`

func scanPostMedia(rows *sql.Rows) (PostMedia, error) {
	var m PostMedia
	var captured sql.NullString
	var lat, lng sql.NullFloat64
	var cover int
	var cx, cy, cw, ch sql.NullFloat64
	var voice int
	var peaks string
	if err := rows.Scan(&m.ID, &m.PostID, &m.BlobID, &m.Kind, &m.SortOrder,
		&captured, &lat, &lng, &cover, &m.OriginalFilename, &m.SizeBytes, &m.MimeType,
		&m.AudioArtist, &m.AudioTitle, &m.AudioCoverBlobID, &cx, &cy, &cw, &ch,
		&voice, &m.AudioDurationMs, &peaks, &m.VideoPosterBlobID); err != nil {
		return PostMedia{}, err
	}
	m.Voice = voice != 0
	m.AudioPeaks = splitPeaks(peaks)
	if cx.Valid && cy.Valid && cw.Valid && ch.Valid {
		m.Crop = &CoverCrop{X: cx.Float64, Y: cy.Float64, W: cw.Float64, H: ch.Float64}
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
	return m, nil
}

// ListPostMedia returns media metadata for API responses.
func (c *Chronicle) ListPostMedia(ctx context.Context, postID string) ([]PostMedia, error) {
	rows, err := c.db.QueryContext(ctx, postMediaSelect+`
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
		m, err := scanPostMedia(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MediaSummary is a compact media descriptor for JSON.
type MediaSummary struct {
	BlobID            string     `json:"blob_id"`
	Kind              string     `json:"kind"`
	CapturedAt        *string    `json:"captured_at,omitempty"`
	GeoLat            *float64   `json:"geo_lat,omitempty"`
	GeoLng            *float64   `json:"geo_lng,omitempty"`
	IsCover           bool       `json:"is_cover"`
	Filename          *string    `json:"filename,omitempty"`
	SizeBytes         *int64     `json:"size_bytes,omitempty"`
	MimeType          string     `json:"mime_type,omitempty"`
	AudioArtist       string     `json:"audio_artist,omitempty"`
	AudioTitle        string     `json:"audio_title,omitempty"`
	AudioCoverBlobID  string     `json:"audio_cover_blob_id,omitempty"`
	VideoPosterBlobID string     `json:"video_poster_blob_id,omitempty"`
	Voice             bool       `json:"voice,omitempty"`
	AudioDurationMs   int64      `json:"audio_duration_ms,omitempty"`
	AudioPeaks        []int      `json:"audio_peaks,omitempty"`
	Crop              *CoverCrop `json:"crop,omitempty"`
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
		s.MimeType = m.MimeType
		s.AudioArtist = m.AudioArtist
		s.AudioTitle = m.AudioTitle
		s.AudioCoverBlobID = m.AudioCoverBlobID
		s.VideoPosterBlobID = m.VideoPosterBlobID
		s.Voice = m.Voice
		s.AudioDurationMs = m.AudioDurationMs
		s.AudioPeaks = m.AudioPeaks
		s.Crop = m.Crop
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
	if _, err := c.requireAuthor(ctx, c.db, circleID, accountID, post.IdentityID, now); err != nil {
		return err
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
