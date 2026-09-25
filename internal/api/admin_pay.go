package api

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

func (s *Server) handleAdminPayHub(w http.ResponseWriter, r *http.Request) {
	settings, err := s.Auth.PayHubSettings(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"requisites":    settings.Requisites,
		"donate":        map[string]any{"show": settings.DonateShow, "until": settings.DonateUntil},
		"subscription":  map[string]any{"required": settings.SubscriptionRequired, "pending_count": settings.PendingRequestCount},
	})
}

func (s *Server) handleAdminSetPayRequisites(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Requisites string `json:"requisites"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if err := s.Auth.SetPayRequisites(r.Context(), body.Requisites); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminPayDonate(w http.ResponseWriter, r *http.Request) {
	settings, err := s.Auth.PayDonateSettings(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleAdminSetPayDonate(w http.ResponseWriter, r *http.Request) {
	var body auth.PayDonateSettings
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if err := s.Auth.SetPayDonateSettings(r.Context(), body); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminPaySubscription(w http.ResponseWriter, r *http.Request) {
	settings, err := s.Auth.PaySubscriptionSettings(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleAdminSetPaySubscription(w http.ResponseWriter, r *http.Request) {
	var body auth.PaySubscriptionSettings
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if err := s.Auth.SetPaySubscriptionSettings(r.Context(), body); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminPayRequests(w http.ResponseWriter, r *http.Request) {
	items, err := s.Auth.ListPendingPayRequests(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"requests": items})
}

func (s *Server) handleAdminPayRequestByID(w http.ResponseWriter, r *http.Request) {
	item, err := s.Auth.PayRequestByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleAdminApprovePayRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Days      int  `json:"days"`
		Unlimited bool `json:"unlimited"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if err := s.Auth.ApprovePayRequest(r.Context(), r.PathValue("id"), body.Days, body.Unlimited); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminPayAccounts(w http.ResponseWriter, r *http.Request) {
	items, err := s.Auth.ListPayAccounts(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": items})
}

func (s *Server) handleAdminPayAccountByID(w http.ResponseWriter, r *http.Request) {
	item, err := s.Auth.PayAccountByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleAdminGrantPayAccount(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Days      int  `json:"days"`
		Unlimited bool `json:"unlimited"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if err := s.Auth.GrantPayAccount(r.Context(), r.PathValue("id"), body.Days, body.Unlimited); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminRejectPayRequest(w http.ResponseWriter, r *http.Request) {
	blobsDir := filepath.Join(s.DataDir, "blobs")
	if err := s.Auth.RejectPayRequest(r.Context(), r.PathValue("id"), blobsDir); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminPayBlob(w http.ResponseWriter, r *http.Request) {
	blobID := r.PathValue("blob_id")
	allowed, err := s.Auth.PayBlobAllowedForAdmin(r.Context(), blobID)
	if err != nil {
		writeError(w, err)
		return
	}
	if !allowed {
		writeError(w, auth.ErrForbidden)
		return
	}
	info, err := s.Blobs.OpenBlob(r.Context(), blobID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	f, err := os.Open(info.Path)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", info.MimeType)
	w.Header().Set("Content-Disposition", "inline; filename=\""+info.Filename+"\"")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.FormatInt(info.SizeBytes, 10))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}
