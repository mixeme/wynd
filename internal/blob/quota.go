package blob

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"time"

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

// queryer — общий вид *sql.DB и *sql.Tx для чтения занятого места.
type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// reservationTTL — сколько живёт резервация без подтверждения. Операция
// записи коротка; более старую строку считаем следом аварийного выхода, а не
// занятым местом, и убираем при следующей резервации.
const reservationTTL = 15 * time.Minute

func reservationCutoff() int64 {
	return time.Now().UTC().Add(-reservationTTL).Unix()
}

// reservedBytes — место, уже занятое незавершёнными загрузками: принятые
// байты открытых сессий (.part на диске) и строки blobs в состоянии pending.
// Считаются received_bytes, а не expected_size: иначе участник пустыми
// сессиями без единого байта резервировал всю квоту инстанса на сутки
// (аудит 2026-09-22). Полностью принятая сессия равна своему размеру, поэтому
// перепроверка на завершении (UPL-2) остаётся точной.
// Сюда же входят неподтверждённые резервации операций записи (R3).
func (s *Store) reservedBytes(ctx context.Context) (int64, error) {
	return reservedBytesQ(ctx, s.db)
}

func reservedBytesQ(ctx context.Context, q queryer) (int64, error) {
	var reserved sql.NullInt64
	err := q.QueryRowContext(ctx, `
		SELECT
			(SELECT COALESCE(SUM(received_bytes), 0) FROM upload_sessions) +
			(SELECT COALESCE(SUM(size_bytes), 0) FROM blobs WHERE status = 'pending') +
			(SELECT COALESCE(SUM(instance_bytes), 0) FROM quota_reservations WHERE created_at > ?)
	`, reservationCutoff()).Scan(&reserved)
	if err != nil {
		return 0, err
	}
	return reserved.Int64, nil
}

// usedBytes — занятое на сервере: файлы и текст всех кругов (C42) и полные
// архивы, которые собираются или ждут скачивания (план 51; 9.7: «готовые
// архивы считаются в потолке»). Шкала панели и потолок сервера смотрят на
// одно и то же число; квоты кругов архивов не видят.
const physicalUsedSQL = `
		SELECT (SELECT COALESCE(SUM(size_bytes),0) FROM blob_contents) + (SELECT COALESCE(SUM(size_bytes),0) FROM (SELECT MAX(size_bytes) size_bytes FROM blobs WHERE status='complete' AND content_sha256 IS NULL AND storage_path<>'' GROUP BY storage_path))
		     + (SELECT COALESCE(SUM(text_bytes), 0) FROM circles)
		     + (SELECT COALESCE(SUM(size_bytes), 0) FROM circle_archives WHERE status IN ('building', 'ready'))
		     + (SELECT COALESCE(SUM(COALESCE(length(data),0)+COALESCE(length(layout),0)+length(card)),0) FROM move_snapshots)
		     + (SELECT COALESCE(SUM(length(data)),0) FROM move_snapshot_literals)
	`

func (s *Store) usedBytes(ctx context.Context) (int64, error) {
	return usedBytesQ(ctx, s.db)
}

func usedBytesQ(ctx context.Context, q queryer) (int64, error) {
	var used sql.NullInt64
	err := q.QueryRowContext(ctx, physicalUsedSQL).Scan(&used)
	if err != nil {
		return 0, err
	}
	return used.Int64, nil
}

// circleUsedSQL — занятое кругом col: файлы записей и комментариев (4.21),
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

func circleUsedBytesQ(ctx context.Context, q queryer, circleID string) (int64, error) {
	var used sql.NullInt64
	err := q.QueryRowContext(ctx, `SELECT `+strings.ReplaceAll(circleUsedSQL("?"), "?", "?1"), circleID).Scan(&used)
	if err != nil {
		return 0, err
	}
	return used.Int64, nil
}

// circleUsedBytes — занятое кругом вместе с неподтверждёнными резервациями
// его записей (R3). Резервация держится до конца операции: проверка места и
// запись не разделяются.
func (s *Store) circleUsedBytes(ctx context.Context, circleID string) (int64, error) {
	return circleUsedBytesQ(ctx, s.db, circleID)
}

func circleReservedBytesQ(ctx context.Context, q queryer, circleID string) (int64, error) {
	var reserved sql.NullInt64
	err := q.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(circle_bytes), 0) FROM quota_reservations
		WHERE circle_id = ? AND created_at > ?
	`, circleID, reservationCutoff()).Scan(&reserved)
	if err != nil {
		return 0, err
	}
	return reserved.Int64, nil
}

// circleUsageQ — всё занятое кругом: записанное и удержанное.
func circleUsageQ(ctx context.Context, q queryer, circleID string) (int64, error) {
	used, err := circleUsedBytesQ(ctx, q, circleID)
	if err != nil {
		return 0, err
	}
	reserved, err := circleReservedBytesQ(ctx, q, circleID)
	if err != nil {
		return 0, err
	}
	return used + reserved, nil
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

// EventCost — строка журнала без текста: снять реакцию, убрать название дня (0033).
// Стоимость записей с текстом считается в chronicle (PostTextCost и др.).
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

// Reservation — место, удержанное под операцию до её записи. Подтверждение
// записи и освобождение — одна и та же операция: после неё фактическое место
// уже считает usedBytes/circleUsedBytes.
type Reservation struct {
	id string
}

// ID возвращает идентификатор резервации (для журнала и тестов).
func (r *Reservation) ID() string {
	if r == nil {
		return ""
	}
	return r.id
}

// ReserveWrite проверяет квоту и удерживает место одним сериализуемым
// действием (R3). instanceBytes входит в резерв сервера, circleBytes — в
// занятое кругом; файлы, уже завершённые как blob, в instanceBytes не
// добавляются, потому что usedBytes их уже считает.
//
// Транзакция открывается с BEGIN IMMEDIATE (store/sqlite.go): между чтением
// занятого места и вставкой резервации другой писатель не пройдёт.
func (s *Store) ReserveWrite(ctx context.Context, circleID string, instanceBytes, circleBytes int64) (*Reservation, error) {
	if instanceBytes < 0 || circleBytes < 0 {
		return nil, ErrInvalid
	}
	if circleID == "" && circleBytes != 0 {
		return nil, ErrInvalid
	}
	instQuota, err := s.effectiveInstanceQuotaBytes(ctx)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	used, err := usedBytesQ(ctx, tx)
	if err != nil {
		return nil, err
	}
	reserved, err := reservedBytesQ(ctx, tx)
	if err != nil {
		return nil, err
	}
	if used+reserved+instanceBytes > instQuota {
		return nil, ErrInstanceQuotaExceeded
	}
	if circleID != "" {
		cq, err := s.effectiveCircleQuotaBytes(ctx, circleID)
		if err != nil {
			return nil, err
		}
		if cq.Valid {
			cUsed, err := circleUsageQ(ctx, tx, circleID)
			if err != nil {
				return nil, err
			}
			if cUsed+circleBytes > cq.Int64 {
				return nil, ErrQuotaExceeded
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM quota_reservations WHERE created_at <= ?`, reservationCutoff()); err != nil {
		return nil, err
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO quota_reservations (id, circle_id, instance_bytes, circle_bytes, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, id, circleID, instanceBytes, circleBytes, time.Now().UTC().Unix()); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &Reservation{id: id}, nil
}

// ReleaseReservation снимает удержание: операция записана (и её байты уже
// считаются) или не состоялась. Повторное освобождение безопасно.
func (s *Store) ReleaseReservation(ctx context.Context, r *Reservation) error {
	if r == nil || r.id == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM quota_reservations WHERE id = ?`, r.id)
	return err
}

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
