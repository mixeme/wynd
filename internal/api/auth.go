package api

import (
	"net/http"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

type emailBody struct {
	Email string `json:"email"`
}

type verifyBody struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type sessionResponse struct {
	Token           string `json:"token"`
	AccountID       string `json:"account_id"`
	Email           string `json:"email"`
	ExpiresAt       string `json:"expires_at"`
	PendingCircleID string `json:"pending_circle_id,omitempty"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	body, ok := bindJSON[emailBody](w, r)
	if !ok {
		return
	}
	err := s.Auth.Register(r.Context(), auth.RegisterInput{
		Email:    body.Email,
		ClientIP: s.clientIP(r),
		Now:      time.Now().UTC(),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "code_sent"})
}

func (s *Server) handleRequestCode(w http.ResponseWriter, r *http.Request) {
	body, ok := bindJSON[emailBody](w, r)
	if !ok {
		return
	}
	err := s.Auth.RequestCode(r.Context(), auth.RequestCodeInput{
		Email:    body.Email,
		ClientIP: s.clientIP(r),
		Now:      time.Now().UTC(),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "code_sent"})
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	body, ok := bindJSON[verifyBody](w, r)
	if !ok {
		return
	}
	res, err := s.Auth.Verify(r.Context(), auth.VerifyInput{
		Email:    body.Email,
		Code:     body.Code,
		ClientIP: s.clientIP(r),
		Now:      time.Now().UTC(),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sessionResponse{
		Token:           res.Session.Token,
		AccountID:       res.Account.ID,
		Email:           res.Account.Email,
		ExpiresAt:       res.Session.ExpiresAt.UTC().Format(time.RFC3339),
		PendingCircleID: res.PendingCircleID,
	})
}

// handleLogout revokes the participant session named by the Bearer token.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.Auth.RevokeSession(r.Context(), bearerToken(r)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
