package api

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/blob"
)

type defaultQuotaBody struct {
	DefaultCircleQuotaBytes *int64 `json:"default_circle_quota_bytes"`
}

type storageQuotaBody struct {
	QuotaBytes int64 `json:"quota_bytes"`
}

type compressionBody struct {
	PhotoMaxPx         int   `json:"photo_max_px"`
	PhotoQuality       int   `json:"photo_quality"`
	VideoMaxHeight     int   `json:"video_max_height"`
	VideoBitrateKbps   int   `json:"video_bitrate_kbps"`
	AttachmentMaxBytes int64 `json:"attachment_max_bytes"`
}

type circleQuotaBody struct {
	Custom     bool   `json:"custom"`
	QuotaBytes *int64 `json:"quota_bytes"`
}

func (s *Server) handleAdminStorage(w http.ResponseWriter, r *http.Request) {
	used, err := s.Blobs.UsedBytes(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	quota, err := s.Blobs.InstanceQuotaBytes(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	defaultQuota, err := s.Blobs.DefaultCircleQuotaBytes(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	circles, err := s.listStorageCircles(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	var defaultPtr *int64
	if defaultQuota.Valid {
		q := defaultQuota.Int64
		defaultPtr = &q
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"used_bytes":                 used,
		"quota_bytes":                quota,
		"default_circle_quota_bytes": defaultPtr,
		"circles":                    circles,
	})
}

type storageCircle struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Posts       int    `json:"posts"`
	MediaBytes  int64  `json:"media_bytes"`
	QuotaBytes  *int64 `json:"quota_bytes"`
	QuotaCustom bool   `json:"quota_custom"`
	OwnerEmail  string `json:"owner_email"`
}

func (s *Server) listStorageCircles(ctx context.Context) ([]storageCircle, error) {
	rows, err := s.Blobs.DB().QueryContext(ctx, `
		SELECT c.id, c.name, c.color, c.quota_bytes, c.quota_custom, a.email,
			(SELECT COUNT(*) FROM posts p WHERE p.circle_id = c.id AND p.deleted = 0),
			(SELECT COALESCE(SUM(b.size_bytes), 0)
			 FROM post_media pm
			 JOIN posts p ON p.id = pm.post_id AND p.circle_id = c.id AND p.deleted = 0
			 JOIN blobs b ON b.id = pm.blob_id AND b.status = 'complete')
		FROM circles c
		JOIN accounts a ON a.id = c.owner_account_id
		ORDER BY 7 DESC, c.name COLLATE NOCASE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]storageCircle, 0)
	for rows.Next() {
		var row storageCircle
		var quota sql.NullInt64
		var custom int
		if err := rows.Scan(&row.ID, &row.Name, &row.Color, &quota, &custom, &row.OwnerEmail,
			&row.Posts, &row.MediaBytes); err != nil {
			return nil, err
		}
		row.QuotaCustom = custom != 0
		if quota.Valid {
			q := quota.Int64
			row.QuotaBytes = &q
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Server) handleAdminSetStorageQuota(w http.ResponseWriter, r *http.Request) {
	var body storageQuotaBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if body.QuotaBytes < 1 {
		writeError(w, blob.ErrInvalid)
		return
	}
	if err := s.Blobs.SetInstanceQuotaBytes(r.Context(), body.QuotaBytes); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminSetDefaultQuota(w http.ResponseWriter, r *http.Request) {
	var body defaultQuotaBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if err := s.Blobs.SetDefaultCircleQuotaBytes(r.Context(), body.DefaultCircleQuotaBytes); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminSetCircleQuota(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("id")
	var body circleQuotaBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	ctx := r.Context()
	var exists int
	err := s.Blobs.DB().QueryRowContext(ctx, `SELECT 1 FROM circles WHERE id = ?`, circleID).Scan(&exists)
	if err == sql.ErrNoRows {
		writeError(w, blob.ErrNotFound)
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	if !body.Custom {
		if _, err := s.Blobs.DB().ExecContext(ctx, `
			UPDATE circles SET quota_custom = 0 WHERE id = ?
		`, circleID); err != nil {
			writeError(w, err)
			return
		}
	} else if body.QuotaBytes != nil {
		if *body.QuotaBytes < 1 {
			writeError(w, blob.ErrInvalid)
			return
		}
		if _, err := s.Blobs.DB().ExecContext(ctx, `
			UPDATE circles SET quota_custom = 1, quota_bytes = ? WHERE id = ?
		`, *body.QuotaBytes, circleID); err != nil {
			writeError(w, err)
			return
		}
	} else {
		if _, err := s.Blobs.DB().ExecContext(ctx, `
			UPDATE circles SET quota_custom = 1, quota_bytes = NULL WHERE id = ?
		`, circleID); err != nil {
			writeError(w, err)
			return
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, _ = s.Auth.DB().ExecContext(ctx, `
		UPDATE quota_requests SET status = 'approved', resolved_at = ?
		WHERE circle_id = ? AND status = 'pending'
	`, now, circleID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminCompression(w http.ResponseWriter, r *http.Request) {
	cs, err := s.Blobs.LoadCompressionSettings(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cs)
}

func (s *Server) handleAdminSetCompression(w http.ResponseWriter, r *http.Request) {
	var body compressionBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	cs := blob.CompressionSettings{
		PhotoMaxPx:         body.PhotoMaxPx,
		PhotoQuality:       body.PhotoQuality,
		VideoMaxHeight:     body.VideoMaxHeight,
		VideoBitrateKbps:   body.VideoBitrateKbps,
		AttachmentMaxBytes: body.AttachmentMaxBytes,
	}
	if err := s.Blobs.SaveCompressionSettings(r.Context(), cs); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) loadInstanceTimestamps(ctx context.Context) (routineAt, backupAt *time.Time) {
	var routineRaw, backupRaw sql.NullString
	_ = s.Auth.DB().QueryRowContext(ctx, `
		SELECT last_routine_at, last_backup_at FROM instance_settings WHERE id = 1
	`).Scan(&routineRaw, &backupRaw)
	if routineRaw.Valid && routineRaw.String != "" {
		if t, err := time.Parse(time.RFC3339Nano, routineRaw.String); err == nil {
			routineAt = &t
		}
	}
	if backupRaw.Valid && backupRaw.String != "" {
		if t, err := time.Parse(time.RFC3339Nano, backupRaw.String); err == nil {
			backupAt = &t
		}
	}
	return routineAt, backupAt
}
