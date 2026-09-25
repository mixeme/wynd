package api

import (
	"net/http"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/config"
)

type accessBody struct {
	Name             string  `json:"name"`
	RegistrationMode string  `json:"registration_mode"`
	PublicURL        *string `json:"public_url"`
}

func (s *Server) handleAdminAccess(w http.ResponseWriter, r *http.Request) {
	info, err := s.Auth.Instance(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":              info.Name,
		"registration_mode": info.RegistrationMode,
		"public_url":        s.PublicURL,
	})
}

func (s *Server) handleAdminSetAccess(w http.ResponseWriter, r *http.Request) {
	var body accessBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if body.Name != "" {
		if err := s.Auth.SetInstanceName(r.Context(), body.Name); err != nil {
			writeError(w, err)
			return
		}
	}
	if body.RegistrationMode != "" {
		mode := auth.RegistrationMode(body.RegistrationMode)
		if err := s.Auth.SetRegistrationMode(r.Context(), mode); err != nil {
			writeError(w, err)
			return
		}
	}
	if body.PublicURL != nil {
		url := config.NormalizePublicURL(*body.PublicURL)
		if url == "" {
			url = config.DefaultPublicURL
		}
		if err := config.WritePublicURL(s.DataDir, url); err != nil {
			writeError(w, err)
			return
		}
		s.applyPublicURL(url)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminAccounts(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Auth.ListAccounts(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": rows})
}

func (s *Server) handleAdminGetAccount(w http.ResponseWriter, r *http.Request) {
	detail, err := s.Auth.AccountDetail(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminDeleteAccount(w http.ResponseWriter, r *http.Request) {
	if err := s.Auth.DeleteAccount(r.Context(), r.PathValue("id"), time.Now().UTC()); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminBlockAccount(w http.ResponseWriter, r *http.Request) {
	if err := s.Auth.SetAccountBlocked(r.Context(), r.PathValue("id"), true); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminUnblockAccount(w http.ResponseWriter, r *http.Request) {
	if err := s.Auth.SetAccountBlocked(r.Context(), r.PathValue("id"), false); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminListInvites(w http.ResponseWriter, r *http.Request) {
	invites, err := s.Auth.ListServerInvites(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"invites": invites})
}

func (s *Server) handleAdminRevokeInvite(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.Auth.RevokeInvite(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
