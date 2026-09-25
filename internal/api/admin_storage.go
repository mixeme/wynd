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
	circles, err := s.Blobs.ListStorageCircles(r.Context())
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
		"circles":                    storageCirclesJSON(circles),
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

func storageCirclesJSON(circles []blob.StorageCircle) []storageCircle {
	out := make([]storageCircle, len(circles))
	for i, row := range circles {
		out[i] = storageCircle{
			ID: row.ID, Name: row.Name, Color: row.Color, Posts: row.Posts,
			MediaBytes: row.MediaBytes, QuotaBytes: row.QuotaBytes,
			QuotaCustom: row.QuotaCustom, OwnerEmail: row.OwnerEmail,
		}
	}
	return out
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
	if err := s.Blobs.SetCircleQuotaAdmin(r.Context(), circleID, body.Custom, body.QuotaBytes); err != nil {
		writeError(w, err)
		return
	}
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
