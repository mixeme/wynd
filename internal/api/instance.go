package api

import (
	"net/http"

	"gitea.mixdep.ru/mix/wynd/internal/version"
)

func (s *Server) handleInstance(w http.ResponseWriter, r *http.Request) {
	info, err := s.Auth.Instance(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	delivery := "mail"
	if info.Loopback {
		if s.Mail == nil {
			delivery = "log"
		} else {
			ok, err := s.Mail.Configured(r.Context())
			if err != nil || !ok {
				delivery = "log"
			}
		}
	}
	out := map[string]any{
		"name":              info.Name,
		"version":           info.Version,
		"registration_mode": info.RegistrationMode,
		"loopback":          info.Loopback,
		"bootstrapped":      info.Bootstrapped,
		"code_delivery":     delivery,
		// AGPL §13: клиент показывает ссылку на исходники этого сервера,
		// а не адрес, зашитый в сборку клиента (LIC-2).
		"source_url": version.SourceURL,
	}
	if s.Blobs != nil {
		if cs, err := s.Blobs.LoadCompressionSettings(r.Context()); err == nil {
			out["compression"] = cs
		}
	}
	if s.Push != nil {
		if pub, err := s.Push.PublicKey(r.Context()); err == nil && pub != "" {
			out["vapid_public_key"] = pub
		}
	}
	writeJSON(w, http.StatusOK, out)
}
