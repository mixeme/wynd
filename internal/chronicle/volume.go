package chronicle

import (
	"context"
	"database/sql"
	"time"
)

// VolumeBucket is media bytes aggregated by chronicle period.
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
	VolumeStepDay:   `date(p.created_at)`,
	VolumeStepWeek:  `date(p.created_at, '-6 days', 'weekday 1')`,
	VolumeStepMonth: `strftime('%Y-%m-01', p.created_at)`,
}

// MediaVolumeChart returns media volume for cutoff selection, bucketed by a
// step that fits the circle's age.
func (c *Chronicle) MediaVolumeChart(ctx context.Context, circleID string) ([]VolumeBucket, VolumeStep, error) {
	return c.mediaVolumeChart(ctx, circleID, time.Now().UTC())
}

func (c *Chronicle) mediaVolumeChart(ctx context.Context, circleID string, now time.Time) ([]VolumeBucket, VolumeStep, error) {
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
	rows, err := c.db.QueryContext(ctx, `
		SELECT `+volumePeriodSQL[step]+` AS period,
			COALESCE(SUM(b.size_bytes), 0) AS bytes
		FROM posts p
		JOIN (
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
		) pm ON pm.post_id = p.id
		JOIN blobs b ON b.id = pm.blob_id AND b.status = 'complete'
		WHERE p.circle_id = ? AND p.deleted = 0
		GROUP BY period
		ORDER BY period
	`, circleID)
	if err != nil {
		return nil, step, err
	}
	defer rows.Close()
	// Пустой, а не nil: nil уходил в JSON как null, и экран квоты круга без
	// медиа падал на volume.length (план 42, найдено при проверке SCR-1).
	out := []VolumeBucket{}
	var cumulative int64
	for rows.Next() {
		var b VolumeBucket
		if err := rows.Scan(&b.Period, &b.Bytes); err != nil {
			return nil, step, err
		}
		cumulative += b.Bytes
		b.CumulativeBytes = cumulative
		out = append(out, b)
	}
	return out, step, rows.Err()
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
	cutoff, err := CutoffInstant(cutoffDate)
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

// FreedBytesBeforeCutoff estimates media bytes that would be freed at cutoff.
func (c *Chronicle) FreedBytesBeforeCutoff(ctx context.Context, circleID, cutoffDate string) (int64, error) {
	cutoff, err := CutoffInstant(cutoffDate)
	if err != nil {
		return 0, err
	}
	var total sql.NullInt64
	err = c.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(b.size_bytes), 0)
		FROM posts p
		JOIN (
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
		) pm ON pm.post_id = p.id
		JOIN blobs b ON b.id = pm.blob_id AND b.status = 'complete'
		WHERE p.circle_id = ? AND p.deleted = 0 AND p.created_at < ?
	`, circleID, formatTime(cutoff)).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Int64, nil
}
