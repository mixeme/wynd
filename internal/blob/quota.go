package blob

import (
	"context"
	"database/sql"
)

// CompressionSettings are client-side compression thresholds (no server-side ffmpeg).
type CompressionSettings struct {
	PhotoMaxPx         int   `json:"photo_max_px"`
	PhotoQuality       int   `json:"photo_quality"`
	VideoMaxHeight     int   `json:"video_max_height"`
	VideoBitrateKbps   int   `json:"video_bitrate_kbps"`
	AttachmentMaxBytes int64 `json:"attachment_max_bytes"`
}

// LoadCompressionSettings reads instance compression thresholds.
func (s *Store) LoadCompressionSettings(ctx context.Context) (CompressionSettings, error) {
	var cs CompressionSettings
	err := s.db.QueryRowContext(ctx, `
		SELECT compress_photo_max_px, compress_photo_quality,
			compress_video_max_height, compress_video_bitrate_kbps,
			compress_attachment_max_bytes
		FROM instance_settings WHERE id = 1
	`).Scan(&cs.PhotoMaxPx, &cs.PhotoQuality, &cs.VideoMaxHeight,
		&cs.VideoBitrateKbps, &cs.AttachmentMaxBytes)
	if err != nil {
		return CompressionSettings{}, err
	}
	return cs, nil
}

func (s *Store) instanceQuotaBytes(ctx context.Context) (int64, error) {
	var q int64
	err := s.db.QueryRowContext(ctx, `
		SELECT storage_quota_bytes FROM instance_settings WHERE id = 1
	`).Scan(&q)
	return q, err
}

func (s *Store) DefaultCircleQuotaBytes(ctx context.Context) (sql.NullInt64, error) {
	var q sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT default_circle_quota_bytes FROM instance_settings WHERE id = 1
	`).Scan(&q)
	return q, err
}

func (s *Store) SetDefaultCircleQuotaBytes(ctx context.Context, quota *int64) error {
	if quota == nil {
		_, err := s.db.ExecContext(ctx, `
			UPDATE instance_settings SET default_circle_quota_bytes = NULL WHERE id = 1
		`)
		return err
	}
	if *quota < 1 {
		return ErrInvalid
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE instance_settings SET default_circle_quota_bytes = ? WHERE id = 1
	`, *quota)
	return err
}

func (s *Store) effectiveCircleQuotaBytes(ctx context.Context, circleID string) (sql.NullInt64, error) {
	var quotaCustom int
	var ownQuota sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT quota_custom, quota_bytes FROM circles WHERE id = ?
	`, circleID).Scan(&quotaCustom, &ownQuota)
	if err == sql.ErrNoRows {
		return sql.NullInt64{}, ErrNotFound
	}
	if err != nil {
		return sql.NullInt64{}, err
	}
	if quotaCustom == 1 {
		return ownQuota, nil
	}
	var defaultQuota sql.NullInt64
	err = s.db.QueryRowContext(ctx, `
		SELECT default_circle_quota_bytes FROM instance_settings WHERE id = 1
	`).Scan(&defaultQuota)
	return defaultQuota, err
}

func (s *Store) circleQuotaBytes(ctx context.Context, circleID string) (sql.NullInt64, error) {
	return s.effectiveCircleQuotaBytes(ctx, circleID)
}

func (s *Store) usedBytes(ctx context.Context) (int64, error) {
	var used sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(size_bytes), 0) FROM blobs WHERE status = 'complete'
	`).Scan(&used)
	if err != nil {
		return 0, err
	}
	return used.Int64, nil
}

func (s *Store) circleUsedBytes(ctx context.Context, circleID string) (int64, error) {
	var used sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(b.size_bytes), 0)
		FROM post_media pm
		JOIN posts p ON p.id = pm.post_id AND p.circle_id = ? AND p.deleted = 0
		JOIN blobs b ON b.id = pm.blob_id AND b.status = 'complete'
	`, circleID).Scan(&used)
	if err != nil {
		return 0, err
	}
	return used.Int64, nil
}

// CheckMediaQuota verifies instance and optional circle media quotas.
func (s *Store) CheckMediaQuota(ctx context.Context, circleID string, additionalBytes int64) error {
	if additionalBytes < 0 {
		return ErrInvalid
	}
	instQuota, err := s.instanceQuotaBytes(ctx)
	if err != nil {
		return err
	}
	used, err := s.usedBytes(ctx)
	if err != nil {
		return err
	}
	// Upload sessions are not in usedBytes yet. Attach-time blobs are already
	// complete, so adding their size again would reject a legal attach.
	instanceExtra := additionalBytes
	if circleID != "" {
		instanceExtra = 0
	}
	if used+instanceExtra > instQuota {
		return ErrQuotaExceeded
	}
	if circleID == "" {
		return nil
	}
	cq, err := s.effectiveCircleQuotaBytes(ctx, circleID)
	if err != nil {
		return err
	}
	if !cq.Valid {
		return nil
	}
	cUsed, err := s.circleUsedBytes(ctx, circleID)
	if err != nil {
		return err
	}
	if cUsed+additionalBytes > cq.Int64 {
		return ErrQuotaExceeded
	}
	return nil
}

// UsedBytes returns total size of complete blobs.
func (s *Store) UsedBytes(ctx context.Context) (int64, error) {
	return s.usedBytes(ctx)
}

// InstanceQuotaBytes returns the instance storage quota.
func (s *Store) InstanceQuotaBytes(ctx context.Context) (int64, error) {
	return s.instanceQuotaBytes(ctx)
}

// CircleUsedBytes returns media bytes used by a circle.
func (s *Store) CircleUsedBytes(ctx context.Context, circleID string) (int64, error) {
	return s.circleUsedBytes(ctx, circleID)
}

// CircleQuotaBytes returns optional per-circle quota.
func (s *Store) CircleQuotaBytes(ctx context.Context, circleID string) (sql.NullInt64, error) {
	return s.circleQuotaBytes(ctx, circleID)
}

// SetInstanceQuotaBytes updates the instance storage quota.
func (s *Store) SetInstanceQuotaBytes(ctx context.Context, quota int64) error {
	if quota < 1 {
		return ErrInvalid
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE instance_settings SET storage_quota_bytes = ? WHERE id = 1
	`, quota)
	return err
}

// SaveCompressionSettings updates client compression thresholds.
func (s *Store) SaveCompressionSettings(ctx context.Context, cs CompressionSettings) error {
	if cs.PhotoMaxPx < 1 || cs.PhotoQuality < 1 || cs.VideoMaxHeight < 1 ||
		cs.VideoBitrateKbps < 1 || cs.AttachmentMaxBytes < 1 {
		return ErrInvalid
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE instance_settings SET
			compress_photo_max_px = ?,
			compress_photo_quality = ?,
			compress_video_max_height = ?,
			compress_video_bitrate_kbps = ?,
			compress_attachment_max_bytes = ?
		WHERE id = 1
	`, cs.PhotoMaxPx, cs.PhotoQuality, cs.VideoMaxHeight, cs.VideoBitrateKbps, cs.AttachmentMaxBytes)
	return err
}

// TotalBytesForBlobs sums sizes of complete blobs by id.
func (s *Store) TotalBytesForBlobs(ctx context.Context, blobIDs []string) (int64, error) {
	if len(blobIDs) == 0 {
		return 0, nil
	}
	query := `SELECT COALESCE(SUM(size_bytes), 0) FROM blobs WHERE status = 'complete' AND id IN (`
	args := make([]any, len(blobIDs))
	for i, id := range blobIDs {
		if i > 0 {
			query += ","
		}
		query += "?"
		args[i] = id
	}
	query += ")"
	var total int64
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&total)
	return total, err
}
