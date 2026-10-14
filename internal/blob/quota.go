package blob

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"

	"gitea.mixdep.ru/mix/wynd/internal/check"
)

// CompressionSettings are client-side compression thresholds (photos, video and audio; no server ffmpeg).
type CompressionSettings struct {
	PhotoMaxPx         int   `json:"photo_max_px"`
	PhotoQuality       int   `json:"photo_quality"`
	VideoMaxHeight     int   `json:"video_max_height"`
	VideoBitrateKbps   int   `json:"video_bitrate_kbps"`
	AudioBitrateKbps   int   `json:"audio_bitrate_kbps"`
	AttachmentMaxBytes int64 `json:"attachment_max_bytes"`
}

// LoadCompressionSettings reads instance compression thresholds.
func (s *Store) LoadCompressionSettings(ctx context.Context) (CompressionSettings, error) {
	var cs CompressionSettings
	err := s.db.QueryRowContext(ctx, `
		SELECT compress_photo_max_px, compress_photo_quality,
			compress_video_max_height, compress_video_bitrate_kbps,
			compress_audio_bitrate_kbps, compress_attachment_max_bytes
		FROM instance_settings WHERE id = 1
	`).Scan(&cs.PhotoMaxPx, &cs.PhotoQuality, &cs.VideoMaxHeight,
		&cs.VideoBitrateKbps, &cs.AudioBitrateKbps, &cs.AttachmentMaxBytes)
	if err != nil {
		return CompressionSettings{}, err
	}
	return cs, nil
}

func (s *Store) instanceQuotaSettings(ctx context.Context) (absolute int64, diskPercent sql.NullInt64, err error) {
	err = s.db.QueryRowContext(ctx, `
		SELECT storage_quota_bytes, storage_quota_disk_percent FROM instance_settings WHERE id = 1
	`).Scan(&absolute, &diskPercent)
	return absolute, diskPercent, err
}

func (s *Store) effectiveInstanceQuotaBytes(ctx context.Context) (int64, error) {
	absolute, diskPercent, err := s.instanceQuotaSettings(ctx)
	if err != nil {
		return 0, err
	}
	if !diskPercent.Valid {
		return absolute, nil
	}
	pct := diskPercent.Int64
	if pct < 1 || pct > 100 {
		return 0, ErrInvalid
	}
	dataDir := filepath.Dir(s.dir)
	_, total, err := check.DiskUsage(dataDir)
	if err != nil {
		return 0, err
	}
	if total == 0 {
		return 0, ErrInvalid
	}
	return int64(total) * pct / 100, nil
}

// DefaultCircleQuotaBytes returns the instance default circle quota; NULL means no ceiling.
func (s *Store) DefaultCircleQuotaBytes(ctx context.Context) (sql.NullInt64, error) {
	var q sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT default_circle_quota_bytes FROM instance_settings WHERE id = 1
	`).Scan(&q)
	return q, err
}

// SetDefaultCircleQuotaBytes sets the default circle quota; nil removes it. Circles with their own quota are not touched.
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

// reservedBytes — место, уже занятое незавершёнными загрузками: принятые
// байты открытых сессий (.part на диске) и строки blobs в состоянии pending.
// Считаются received_bytes, а не expected_size: иначе участник пустыми
// сессиями без единого байта резервировал всю квоту инстанса на сутки
// (аудит 2026-09-22). Полностью принятая сессия равна своему размеру, поэтому
// перепроверка на завершении (UPL-2) остаётся точной.
func (s *Store) reservedBytes(ctx context.Context) (int64, error) {
	var reserved sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COALESCE(SUM(received_bytes), 0) FROM upload_sessions) +
			(SELECT COALESCE(SUM(size_bytes), 0) FROM blobs WHERE status = 'pending')
	`).Scan(&reserved)
	if err != nil {
		return 0, err
	}
	return reserved.Int64, nil
}

// usedBytes — занятое на сервере: файлы и текст всех кругов (C42) и полные
// архивы, которые собираются или ждут скачивания (план 51; 9.5: «готовые
// архивы считаются в потолке»). Шкала панели и потолок сервера смотрят на
// одно и то же число; квоты кругов архивов не видят.
func (s *Store) usedBytes(ctx context.Context) (int64, error) {
	var used sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT (SELECT COALESCE(SUM(size_bytes), 0) FROM blobs WHERE status = 'complete')
		     + (SELECT COALESCE(SUM(text_bytes), 0) FROM circles)
		     + (SELECT COALESCE(SUM(size_bytes), 0) FROM circle_archives WHERE status IN ('building', 'ready'))
	`).Scan(&used)
	if err != nil {
		return 0, err
	}
	return used.Int64, nil
}

// circleUsedSQL — занятое кругом col: файлы записей и комментариев (4.29),
// фото участников и текст (C42: «считать надо всё»). Текст ведут триггеры
// базы в circles.text_bytes — замер того, сколько он весит на диске.
func circleUsedSQL(col string) string {
	return `(SELECT COALESCE(SUM(b.size_bytes), 0)
		   FROM post_media pm
		   JOIN posts p ON p.id = pm.post_id AND p.circle_id = ` + col + ` AND p.deleted = 0
		   JOIN blobs b ON b.id = pm.blob_id AND b.status = 'complete')
		+ (SELECT COALESCE(SUM(b.size_bytes), 0)
		   FROM comment_media cmm
		   JOIN comments cm ON cm.id = cmm.comment_id AND cm.circle_id = ` + col + ` AND cm.deleted = 0
		   JOIN blobs b ON b.id = cmm.blob_id AND b.status = 'complete')
		+ (SELECT COALESCE(SUM(b.size_bytes), 0)
		   FROM blobs b
		   WHERE b.status = 'complete' AND b.id IN (
			SELECT n.avatar_blob_id FROM identity_names n
			JOIN identities i ON i.id = n.identity_id AND i.circle_id = ` + col + `
			WHERE n.erased_at IS NULL AND n.avatar_blob_id IS NOT NULL))
		+ (SELECT text_bytes FROM circles tc WHERE tc.id = ` + col + `)`
}

func (s *Store) circleUsedBytes(ctx context.Context, circleID string) (int64, error) {
	var used sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT `+strings.ReplaceAll(circleUsedSQL("?"), "?", "?1"), circleID).Scan(&used)
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
	instQuota, err := s.effectiveInstanceQuotaBytes(ctx)
	if err != nil {
		return err
	}
	used, err := s.usedBytes(ctx)
	if err != nil {
		return err
	}
	// Открытые сессии загрузки занимают место, но в usedBytes не входят:
	// без их учёта N параллельных загрузок превышали потолок в N раз (UPL-2).
	// Учитываются и незавершённые строки blobs (status = 'pending').
	reserved, err := s.reservedBytes(ctx)
	if err != nil {
		return err
	}
	used += reserved
	// Attach-time blobs are already complete, so adding their size again
	// would reject a legal attach.
	instanceExtra := additionalBytes
	if circleID != "" {
		instanceExtra = 0
	}
	if used+instanceExtra > instQuota {
		return ErrInstanceQuotaExceeded
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

// TextCost — сколько места займёт на диске запись, комментарий или название
// дня с текстом text: строка, событие журнала и поиск. Те же постоянные, что у
// счёта в базе (миграция 0033).
func TextCost(text string) int64 {
	return 550 + 400 + 4*int64(len(text))
}

// ReactionCost — место под реакцию и её событие (0033).
const ReactionCost int64 = 450 + 400

// EventCost — строка журнала без текста: снять реакцию, убрать название дня (0033).
const EventCost int64 = 400

// CheckWriteQuota — можно ли писать в круг (C42): квота круга и потолок
// сервера считают и текст. Упёрся круг — ErrQuotaExceeded, упёрся сервер —
// ErrInstanceQuotaExceeded: тогда замолкают все круги, и без своей квоты.
func (s *Store) CheckWriteQuota(ctx context.Context, circleID string, cost int64) error {
	if cost < 0 {
		return ErrInvalid
	}
	instQuota, err := s.effectiveInstanceQuotaBytes(ctx)
	if err != nil {
		return err
	}
	used, err := s.usedBytes(ctx)
	if err != nil {
		return err
	}
	reserved, err := s.reservedBytes(ctx)
	if err != nil {
		return err
	}
	if used+reserved+cost > instQuota {
		return ErrInstanceQuotaExceeded
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
	if cUsed+cost > cq.Int64 {
		return ErrQuotaExceeded
	}
	return nil
}

// RoomBytes — сколько ещё помещается на сервер (план 51, 6.43): до потолка
// сервера за вычетом занятого и начатых загрузок — и не больше, чем свободно
// на самом диске. Меньше нуля не бывает.
func (s *Store) RoomBytes(ctx context.Context) (int64, error) {
	instQuota, err := s.effectiveInstanceQuotaBytes(ctx)
	if err != nil {
		return 0, err
	}
	used, err := s.usedBytes(ctx)
	if err != nil {
		return 0, err
	}
	reserved, err := s.reservedBytes(ctx)
	if err != nil {
		return 0, err
	}
	room := instQuota - used - reserved
	if free, _, err := check.DiskUsage(filepath.Dir(s.dir)); err == nil && int64(free) < room {
		room = int64(free)
	}
	return max(room, 0), nil
}

// UsedBytes returns bytes used on the server: complete blobs and circle text.
func (s *Store) UsedBytes(ctx context.Context) (int64, error) {
	return s.usedBytes(ctx)
}

// InstanceQuotaBytes returns the effective instance storage quota.
func (s *Store) InstanceQuotaBytes(ctx context.Context) (int64, error) {
	return s.effectiveInstanceQuotaBytes(ctx)
}

// InstanceQuotaSettings returns stored absolute bytes and optional disk percent mode.
func (s *Store) InstanceQuotaSettings(ctx context.Context) (absolute int64, diskPercent *int, err error) {
	abs, pct, err := s.instanceQuotaSettings(ctx)
	if err != nil {
		return 0, nil, err
	}
	if !pct.Valid {
		return abs, nil, nil
	}
	p := int(pct.Int64)
	return abs, &p, nil
}

// CircleUsedBytes returns bytes used by a circle: media, member photos and text.
func (s *Store) CircleUsedBytes(ctx context.Context, circleID string) (int64, error) {
	return s.circleUsedBytes(ctx, circleID)
}

// CircleQuotaBytes returns optional per-circle quota.
func (s *Store) CircleQuotaBytes(ctx context.Context, circleID string) (sql.NullInt64, error) {
	return s.circleQuotaBytes(ctx, circleID)
}

// SetInstanceQuotaBytes sets absolute instance quota and clears disk percent mode.
func (s *Store) SetInstanceQuotaBytes(ctx context.Context, quota int64) error {
	if quota < 1 {
		return ErrInvalid
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE instance_settings SET storage_quota_bytes = ?, storage_quota_disk_percent = NULL WHERE id = 1
	`, quota)
	return err
}

// SetInstanceQuotaDiskPercent sets percent-of-data-volume quota and clears absolute mode.
func (s *Store) SetInstanceQuotaDiskPercent(ctx context.Context, percent int) error {
	if percent < 1 || percent > 100 {
		return ErrInvalid
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE instance_settings SET storage_quota_disk_percent = ? WHERE id = 1
	`, percent)
	return err
}

// SaveCompressionSettings updates client compression thresholds.
func (s *Store) SaveCompressionSettings(ctx context.Context, cs CompressionSettings) error {
	if cs.PhotoMaxPx < 1 || cs.PhotoQuality < 1 || cs.VideoMaxHeight < 1 ||
		cs.VideoBitrateKbps < 1 || cs.AudioBitrateKbps < 1 || cs.AttachmentMaxBytes < 1 {
		return ErrInvalid
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE instance_settings SET
			compress_photo_max_px = ?,
			compress_photo_quality = ?,
			compress_video_max_height = ?,
			compress_video_bitrate_kbps = ?,
			compress_audio_bitrate_kbps = ?,
			compress_attachment_max_bytes = ?
		WHERE id = 1
	`, cs.PhotoMaxPx, cs.PhotoQuality, cs.VideoMaxHeight, cs.VideoBitrateKbps, cs.AudioBitrateKbps, cs.AttachmentMaxBytes)
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
