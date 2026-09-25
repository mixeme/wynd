package api

import (
	"net/http"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

func (s *Server) handlePayStatus(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, auth.ErrForbidden)
		return
	}
	status, err := s.Auth.PayStatus(r.Context(), sess.AccountID, time.Now().UTC())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleCreatePayRequest(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, auth.ErrForbidden)
		return
	}
	var body struct {
		BlobID  string `json:"blob_id"`
		Comment string `json:"comment"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	id, err := s.Auth.CreatePayRequest(r.Context(), sess.AccountID, body.BlobID, body.Comment)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (s *Server) handleDismissPayBanner(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, auth.ErrForbidden)
		return
	}
	if err := s.Auth.DismissPayBanner(r.Context(), sess.AccountID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
