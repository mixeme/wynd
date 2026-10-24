package chronicle

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// VolumeBucket is media and text bytes aggregated by chronicle period.
// Period — первый день столбика («2026-09-29»): касание ставит отсечку на него.
type VolumeBucket struct {
	Period          string `json:"period"`
	Bytes           int64  `json:"bytes"`
	CumulativeBytes int64  `json:"cumulative_bytes"`
}

// VolumeStep — шаг столбиков графика отсечки (план 46, C7).
type VolumeStep string

const (
	VolumeStepDay   VolumeStep = "day"
	VolumeStepWeek  VolumeStep = "week"
	VolumeStepMonth VolumeStep = "month"
)

// volumeStepFor: молодому кругу месячный столбик один — выбирать нечего.
// До двух недель — дни, до трёх месяцев — недели, дальше — месяцы.
func volumeStepFor(age time.Duration) VolumeStep {
	switch {
	case age <= 14*24*time.Hour:
		return VolumeStepDay
	case age <= 92*24*time.Hour:
		return VolumeStepWeek
	}
	return VolumeStepMonth
}

// SQLite: начало столбика по created_at. Неделя — с понедельника.
var volumePeriodSQL = map[VolumeStep]string{
	VolumeStepDay:   `date(p.created_at, %s)`,
	VolumeStepWeek:  `date(p.created_at, %s, '-6 days', 'weekday 1')`,
	VolumeStepMonth: `strftime('%%Y-%%m-01', p.created_at, %s)`,
}

// volumePeriod — столбик по дню устройства, а не UTC (C59, владелец
// 2026-10-24: «человек выбирает так, как видит это у себя»): ко времени
// прибавляется смещение пояса на сейчас. Там, где переводят часы, у записей
// около полуночи в другой половине года столбик может съехать на час.
func volumePeriod(step VolumeStep, offsetSec int) string {
	return fmt.Sprintf(volumePeriodSQL[step], fmt.Sprintf("'%+d seconds'", offsetSec))
}

// postWeightSQL — сколько весит каждая живая запись круга так, как её
// считает квота: файлы записи и её комментариев плюс текст записи, её
// комментариев и откликов по формулам триггеров quota_text_* (C42). Это то,
// что архивация удаляет вместе с записью. Без текста круг из одних
// текстовых записей давал пустой график и не давал выбрать дату (план 46,
// C55, владелец 2026-10-23: «Текст считаем»). Параметры: circle_id ×4.
const postWeightSQL = `
	SELECT p.id AS post_id,
		550 + 3 * length(CAST(COALESCE(p.body, '') AS BLOB)) + length(CAST(p.author_name AS BLOB))
		+ COALESCE(cmt.bytes, 0) + COALESCE(rx.bytes, 0) + COALESCE(md.bytes, 0) AS bytes
	FROM posts p
	LEFT JOIN (
		SELECT post_id, SUM(560 + 3 * length(CAST(COALESCE(body, '') AS BLOB)) + length(CAST(author_name AS BLOB))) AS bytes
		FROM comments WHERE circle_id = ? GROUP BY post_id
	) cmt ON cmt.post_id = p.id
	LEFT JOIN (
		SELECT post_id, SUM(450 + length(CAST(emoji AS BLOB)) + length(CAST(author_name AS BLOB))) AS bytes
		FROM reactions WHERE circle_id = ? GROUP BY post_id
	) rx ON rx.post_id = p.id
	LEFT JOIN (
		SELECT pm.post_id, SUM(b.size_bytes) AS bytes
		FROM (
			SELECT post_id, blob_id FROM post_media
			UNION
			SELECT post_id, audio_cover_blob_id FROM post_media
			 WHERE audio_cover_blob_id IS NOT NULL AND audio_cover_blob_id != ''
			UNION
			SELECT post_id, video_poster_blob_id FROM post_media
			 WHERE video_poster_blob_id IS NOT NULL AND video_poster_blob_id != ''
			UNION
			SELECT cm.post_id, cmm.blob_id FROM comment_media cmm
			 JOIN comments cm ON cm.id = cmm.comment_id AND cm.deleted = 0
		) pm
		JOIN blobs b ON b.id = pm.blob_id AND b.status = 'complete'
		JOIN posts mp ON mp.id = pm.post_id AND mp.circle_id = ?
		GROUP BY pm.post_id
	) md ON md.post_id = p.id
	WHERE p.circle_id = ? AND p.deleted = 0`

// MediaVolumeChart returns media volume for cutoff selection, bucketed by a
// step that fits the circle's age.
func (c *Chronicle) MediaVolumeChart(ctx context.Context, circleID string) ([]VolumeBucket, VolumeStep, error) {
	return c.MediaVolumeChartTZ(ctx, circleID, "")
}

// MediaVolumeChartTZ — столбики по дням пояса tz (устройство, C59); пусто —
// пояс круга, его нет — UTC.
func (c *Chronicle) MediaVolumeChartTZ(ctx context.Context, circleID, tz string) ([]VolumeBucket, VolumeStep, error) {
	if tz == "" {
		tz = c.archiveTimeZone(ctx, circleID)
	}
	return c.mediaVolumeChartIn(ctx, circleID, time.Now().UTC(), tz)
}

func (c *Chronicle) mediaVolumeChart(ctx context.Context, circleID string, now time.Time) ([]VolumeBucket, VolumeStep, error) {
	return c.mediaVolumeChartIn(ctx, circleID, now, "")
}

func (c *Chronicle) mediaVolumeChartIn(ctx context.Context, circleID string, now time.Time, tz string) ([]VolumeBucket, VolumeStep, error) {
	offset := 0
	if tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			_, offset = now.In(loc).Zone()
		}
	}
	var first sql.NullString
	if err := c.db.QueryRowContext(ctx, `
		SELECT MIN(created_at) FROM posts WHERE circle_id = ? AND deleted = 0
	`, circleID).Scan(&first); err != nil {
		return nil, VolumeStepMonth, err
	}
	step := VolumeStepMonth
	if first.Valid {
		if t, err := parseTime(first.String); err == nil {
			step = volumeStepFor(now.Sub(t))
		}
	}
	return pickVolume(step, func(s VolumeStep) ([]VolumeBucket, error) {
		return c.volumeBuckets(ctx, circleID, s, offset)
	})
}

// minVolumeBars: меньше трёх столбиков — выбирать дату не из чего (владелец,
// 2026-10-23: круг старше двух недель с записями за одну неделю давал один
// недельный столбик). Тогда шаг мельче: месяц → неделя → день.
const minVolumeBars = 3

func pickVolume(step VolumeStep, buckets func(VolumeStep) ([]VolumeBucket, error)) ([]VolumeBucket, VolumeStep, error) {
	for {
		out, err := buckets(step)
		if err != nil {
			return nil, step, err
		}
		finer, ok := finerVolumeStep[step]
		if len(out) >= minVolumeBars || !ok {
			return out, step, nil
		}
		step = finer
	}
}

var finerVolumeStep = map[VolumeStep]VolumeStep{
	VolumeStepMonth: VolumeStepWeek,
	VolumeStepWeek:  VolumeStepDay,
}

func (c *Chronicle) volumeBuckets(ctx context.Context, circleID string, step VolumeStep, offsetSec int) ([]VolumeBucket, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT `+volumePeriod(step, offsetSec)+` AS period, SUM(w.bytes) AS bytes
		FROM (`+postWeightSQL+`) w
		JOIN posts p ON p.id = w.post_id
		GROUP BY period
		ORDER BY period
	`, circleID, circleID, circleID, circleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// Пустой, а не nil: nil уходил в JSON как null, и экран квоты круга без
	// медиа падал на volume.length (план 42, найдено при проверке SCR-1).
	out := []VolumeBucket{}
	var cumulative int64
	for rows.Next() {
		var b VolumeBucket
		if err := rows.Scan(&b.Period, &b.Bytes); err != nil {
			return nil, err
		}
		cumulative += b.Bytes
		b.CumulativeBytes = cumulative
		out = append(out, b)
	}
	return out, rows.Err()
}

// MedianPostBytes returns median media bytes per post for quota estimates.
func (c *Chronicle) MedianPostBytes(ctx context.Context, circleID string) (int64, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT COALESCE(SUM(b.size_bytes), 0)
		FROM posts p
		LEFT JOIN (
			SELECT post_id, blob_id FROM post_media
			UNION
			SELECT post_id, audio_cover_blob_id FROM post_media
			 WHERE audio_cover_blob_id IS NOT NULL AND audio_cover_blob_id != ''
			UNION
			SELECT post_id, video_poster_blob_id FROM post_media
			 WHERE video_poster_blob_id IS NOT NULL AND video_poster_blob_id != ''
		) pm ON pm.post_id = p.id
		LEFT JOIN blobs b ON b.id = pm.blob_id AND b.status = 'complete'
		WHERE p.circle_id = ? AND p.deleted = 0
		GROUP BY p.id
		HAVING COALESCE(SUM(b.size_bytes), 0) > 0
	`, circleID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var sizes []int64
	for rows.Next() {
		var n int64
		if err := rows.Scan(&n); err != nil {
			return 0, err
		}
		sizes = append(sizes, n)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(sizes) == 0 {
		return 0, nil
	}
	return medianInt64(sizes), nil
}

func medianInt64(vals []int64) int64 {
	n := len(vals)
	if n == 0 {
		return 0
	}
	// insertion sort — small N per circle
	for i := 1; i < n; i++ {
		v := vals[i]
		j := i - 1
		for j >= 0 && vals[j] > v {
			vals[j+1] = vals[j]
			j--
		}
		vals[j+1] = v
	}
	mid := n / 2
	if n%2 == 1 {
		return vals[mid]
	}
	return (vals[mid-1] + vals[mid]) / 2
}

// PostsKeptAtCutoff counts posts the archive cycle leaves in the circle:
// created at or after the cutoff.
func (c *Chronicle) PostsKeptAtCutoff(ctx context.Context, circleID, cutoffDate string) (int, error) {
	return c.PostsKeptAtCutoffTZ(ctx, circleID, cutoffDate, "")
}

// PostsKeptAtCutoffTZ — то же, день отсечки в поясе tz (устройство, C59);
// пусто — пояс круга.
func (c *Chronicle) PostsKeptAtCutoffTZ(ctx context.Context, circleID, cutoffDate, tz string) (int, error) {
	if tz == "" {
		tz = c.archiveTimeZone(ctx, circleID)
	}
	cutoff, err := CutoffInstantIn(cutoffDate, tz)
	if err != nil {
		return 0, err
	}
	var n int
	err = c.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM posts
		WHERE circle_id = ? AND deleted = 0 AND created_at >= ?
	`, circleID, formatTime(cutoff)).Scan(&n)
	return n, err
}

// FreedBytesBeforeCutoff estimates bytes freed at cutoff: media and text, as
// the quota counts them (postWeightSQL).
func (c *Chronicle) FreedBytesBeforeCutoff(ctx context.Context, circleID, cutoffDate string) (int64, error) {
	return c.FreedBytesBeforeCutoffTZ(ctx, circleID, cutoffDate, "")
}

// FreedBytesBeforeCutoffTZ — то же, день отсечки в поясе tz (C59).
func (c *Chronicle) FreedBytesBeforeCutoffTZ(ctx context.Context, circleID, cutoffDate, tz string) (int64, error) {
	if tz == "" {
		tz = c.archiveTimeZone(ctx, circleID)
	}
	cutoff, err := CutoffInstantIn(cutoffDate, tz)
	if err != nil {
		return 0, err
	}
	var total sql.NullInt64
	err = c.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(w.bytes), 0)
		FROM (`+postWeightSQL+`) w
		JOIN posts p ON p.id = w.post_id
		WHERE p.created_at < ?
	`, circleID, circleID, circleID, circleID, formatTime(cutoff)).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Int64, nil
}
