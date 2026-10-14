package api

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/xtime"
)

type defaultQuotaBody struct {
	DefaultCircleQuotaBytes *int64 `json:"default_circle_quota_bytes"`
}

type storageQuotaBody struct {
	QuotaBytes       *int64 `json:"quota_bytes"`
	QuotaDiskPercent *int   `json:"quota_disk_percent"`
}

type compressionBody struct {
	PhotoMaxPx         int   `json:"photo_max_px"`
	PhotoQuality       int   `json:"photo_quality"`
	VideoMaxHeight     int   `json:"video_max_height"`
	VideoBitrateKbps   int   `json:"video_bitrate_kbps"`
	AudioBitrateKbps   int   `json:"audio_bitrate_kbps"`
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
	absQuota, diskPercent, err := s.Blobs.InstanceQuotaSettings(r.Context())
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
		"storage_quota_bytes":        absQuota,
		"storage_quota_disk_percent": diskPercent,
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
	body, ok := bindJSON[storageQuotaBody](w, r)
	if !ok {
		return
	}
	hasBytes := body.QuotaBytes != nil
	hasPercent := body.QuotaDiskPercent != nil
	if hasBytes == hasPercent {
		writeError(w, blob.ErrInvalid)
		return
	}
	if hasPercent {
		if err := s.Blobs.SetInstanceQuotaDiskPercent(r.Context(), *body.QuotaDiskPercent); err != nil {
			writeError(w, err)
			return
		}
	} else {
		if *body.QuotaBytes < 1 {
			writeError(w, blob.ErrInvalid)
			return
		}
		if err := s.Blobs.SetInstanceQuotaBytes(r.Context(), *body.QuotaBytes); err != nil {
			writeError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminSetDefaultQuota(w http.ResponseWriter, r *http.Request) {
	body, ok := bindJSON[defaultQuotaBody](w, r)
	if !ok {
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
	body, ok := bindJSON[circleQuotaBody](w, r)
	if !ok {
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
	body, ok := bindJSON[compressionBody](w, r)
	if !ok {
		return
	}
	cs := blob.CompressionSettings{
		PhotoMaxPx:         body.PhotoMaxPx,
		PhotoQuality:       body.PhotoQuality,
		VideoMaxHeight:     body.VideoMaxHeight,
		VideoBitrateKbps:   body.VideoBitrateKbps,
		AudioBitrateKbps:   body.AudioBitrateKbps,
		AttachmentMaxBytes: body.AttachmentMaxBytes,
	}
	// Панель до 0.18.37 звука не знает и поле не шлёт — оставить, что есть.
	if cs.AudioBitrateKbps == 0 {
		current, err := s.Blobs.LoadCompressionSettings(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		cs.AudioBitrateKbps = current.AudioBitrateKbps
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
		if t, err := xtime.Parse(routineRaw.String); err == nil {
			routineAt = &t
		}
	}
	if backupRaw.Valid && backupRaw.String != "" {
		if t, err := xtime.Parse(backupRaw.String); err == nil {
			backupAt = &t
		}
	}
	return routineAt, backupAt
}

// Заявки на квоту — рядом с остальным хранилищем. Лежали в notify.go,
// потому что когда-то писались в одну волну с уведомлениями (ARC-4).

type quotaResolveBody struct {
	AdminNote string `json:"admin_note"`
}

type quotaRequestBody struct {
	RequestedBytes int64 `json:"requested_bytes"`
}

func (s *Server) handleCreateQuotaRequest(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	circleID := r.PathValue("circle_id")
	if err := s.Chronicle.RequireOwner(r.Context(), circleID, sess.AccountID); err != nil {
		writeError(w, err)
		return
	}
	body, ok := bindJSON[quotaRequestBody](w, r)
	if !ok {
		return
	}
	id, err := s.Blobs.CreateQuotaRequest(r.Context(), circleID, sess.AccountID, body.RequestedBytes)
	if err != nil {
		writeError(w, err)
		return
	}
	// Просьба о меньшем, чем есть, одобряется сама (решение владельца
	// 2026-10-14): место она только возвращает серверу, администратору
	// решать нечего. Меньше занятого — нельзя: круг сразу упёрся бы.
	status := "pending"
	if lower, err := s.Blobs.QuotaRequestLowers(r.Context(), circleID, body.RequestedBytes); err != nil {
		writeError(w, err)
		return
	} else if lower {
		if status, err = s.Blobs.ResolveQuotaRequest(r.Context(), id, true, ""); err != nil {
			writeError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "status": status})
}

func (s *Server) handleAdminQuotaRequests(w http.ResponseWriter, r *http.Request) {
	items, err := s.Blobs.ListPendingQuotaRequests(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	type row struct {
		ID             string  `json:"id"`
		CircleID       string  `json:"circle_id"`
		RequesterID    string  `json:"requester_id"`
		RequesterMail  string  `json:"requester_email"`
		RequestedBytes int64   `json:"requested_bytes"`
		Status         string  `json:"status"`
		AdminNote      *string `json:"admin_note,omitempty"`
		CreatedAt      string  `json:"created_at"`
		ResolvedAt     *string `json:"resolved_at,omitempty"`
	}
	out := make([]row, len(items))
	for i, item := range items {
		out[i] = row{
			ID: item.ID, CircleID: item.CircleID, RequesterID: item.RequesterID,
			RequesterMail: item.RequesterEmail, RequestedBytes: item.RequestedBytes,
			Status: item.Status, AdminNote: item.AdminNote, CreatedAt: item.CreatedAt,
			ResolvedAt: item.ResolvedAt,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"requests": out})
}

func (s *Server) handleAdminApproveQuotaRequest(w http.ResponseWriter, r *http.Request) {
	s.resolveQuotaRequest(w, r, true)
}

func (s *Server) handleAdminRejectQuotaRequest(w http.ResponseWriter, r *http.Request) {
	s.resolveQuotaRequest(w, r, false)
}

func (s *Server) resolveQuotaRequest(w http.ResponseWriter, r *http.Request, approve bool) {
	id := r.PathValue("id")
	var body quotaResolveBody
	_ = readJSON(r, &body)
	req, err := s.Blobs.QuotaRequestByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	var before sql.NullInt64
	if req != nil {
		if before, err = s.Blobs.CircleQuotaBytes(r.Context(), req.CircleID); err != nil {
			writeError(w, err)
			return
		}
	}
	status, err := s.Blobs.ResolveQuotaRequest(r.Context(), id, approve, body.AdminNote)
	if err != nil {
		writeError(w, err)
		return
	}
	if req != nil {
		s.sendQuotaAnswer(*req, before, approve)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

// sendQuotaAnswer пишет владельцу об ответе на просьбу о месте (решение
// владельца 2026-10-13: об ответе узнают письмом). В фоне, как рассылка о
// цикле архивации: администратор не ждёт релея, сбой — строка в журнале.
func (s *Server) sendQuotaAnswer(req blob.QuotaRequest, before sql.NullInt64, approved bool) {
	if s.Mail == nil {
		return
	}
	link := strings.TrimRight(s.PublicURL(), "/") + "/circles/" + req.CircleID + "/quota"
	s.notifyWG.Add(1)
	go func() {
		defer s.notifyWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		name, err := s.Chronicle.CircleName(ctx, req.CircleID)
		if err != nil {
			log.Printf("quota answer mail %s: circle: %v", req.CircleID, err)
			return
		}
		if err := s.Mail.SendQuotaAnswer(ctx, req.RequesterEmail, name, req.RequestedBytes, before, approved, link); err != nil {
			log.Printf("quota answer mail %s to %s: %v", req.CircleID, req.RequesterEmail, err)
		}
	}()
}
