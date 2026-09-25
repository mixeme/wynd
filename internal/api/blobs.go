package api

import (
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/blob"
)

type createUploadBody struct {
	ExpectedSize int64  `json:"expected_size"`
	MimeType     string `json:"mime_type"`
	Filename     string `json:"filename"`
}

type completeUploadBody struct {
	SHA256 string `json:"sha256"`
}

func (s *Server) handleCreateUpload(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, blob.ErrForbidden)
		return
	}
	var body createUploadBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	session, err := s.Blobs.CreateSession(r.Context(), blob.CreateSessionInput{
		AccountID: sess.AccountID, ExpectedSize: body.ExpectedSize,
		MimeType: body.MimeType, OriginalFilename: body.Filename, Now: time.Now().UTC(),
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":             session.ID,
		"expected_size":  session.ExpectedSize,
		"mime_type":      session.MimeType,
		"received_bytes": session.ReceivedBytes,
		"expires_at":     session.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleUploadStatus(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, blob.ErrForbidden)
		return
	}
	sessionID := r.PathValue("session_id")
	session, err := s.Blobs.SessionStatus(r.Context(), sessionID, sess.AccountID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	w.Header().Set("Upload-Offset", strconv.FormatInt(session.ReceivedBytes, 10))
	w.Header().Set("Upload-Length", strconv.FormatInt(session.ExpectedSize, 10))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleUploadChunk(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, blob.ErrForbidden)
		return
	}
	sessionID := r.PathValue("session_id")
	offset, err := parseUploadOffset(r)
	if err != nil {
		writeError(w, blob.ErrInvalid)
		return
	}
	newOffset, err := s.Blobs.WriteChunk(r.Context(), sessionID, sess.AccountID, offset, r.Body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	w.Header().Set("Upload-Offset", strconv.FormatInt(newOffset, 10))
	writeJSON(w, http.StatusOK, map[string]any{"received_bytes": newOffset})
}

func (s *Server) handleCompleteUpload(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, blob.ErrForbidden)
		return
	}
	sessionID := r.PathValue("session_id")
	var body completeUploadBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	b, err := s.Blobs.CompleteSession(r.Context(), blob.CompleteSessionInput{
		SessionID: sessionID, AccountID: sess.AccountID, SHA256: body.SHA256,
		Now: time.Now().UTC(),
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": b.ID, "sha256": b.SHA256, "size_bytes": b.SizeBytes, "mime_type": b.MimeType,
		"filename": b.OriginalFilename,
	})
}

func (s *Server) handleServeBlob(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, blob.ErrForbidden)
		return
	}
	blobID := r.PathValue("blob_id")
	allowed, err := s.Blobs.CanAccessBlob(r.Context(), sess.AccountID, blobID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if !allowed {
		writeError(w, blob.ErrForbidden)
		return
	}
	info, err := s.Blobs.OpenBlob(r.Context(), blobID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	serveBlobFile(w, info)
}

// serveBlobFile отдаёт файл блоба с заголовками, общими для участнического
// и админского маршрутов. Content-Disposition собирается через
// mime.FormatMediaType: кавычка в имени файла ломала заголовок, а не-ASCII
// уходил сырыми байтами (аудит 2026-09-22). Файл приватный — не кэшировать.
func serveBlobFile(w http.ResponseWriter, info blob.ServeInfo) {
	f, err := os.Open(info.Path)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	defer f.Close()
	disposition := mime.FormatMediaType(info.Disposition, map[string]string{"filename": info.Filename})
	if disposition == "" {
		disposition = info.Disposition
	}
	w.Header().Set("Content-Type", info.MimeType)
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.FormatInt(info.SizeBytes, 10))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}

func parseUploadOffset(r *http.Request) (int64, error) {
	if v := r.Header.Get("Upload-Offset"); v != "" {
		return strconv.ParseInt(v, 10, 64)
	}
	if cr := r.Header.Get("Content-Range"); cr != "" {
		// bytes start-end/total
		if !strings.HasPrefix(cr, "bytes ") {
			return 0, blob.ErrInvalid
		}
		rest := strings.TrimPrefix(cr, "bytes ")
		dash := strings.Index(rest, "-")
		if dash < 0 {
			return 0, blob.ErrInvalid
		}
		return strconv.ParseInt(rest[:dash], 10, 64)
	}
	return 0, nil
}
