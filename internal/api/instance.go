package api

import (
	"net/http"
)

func (s *Server) handleInstance(w http.ResponseWriter, r *http.Request) {
	info, err := s.Auth.Instance(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := map[string]any{
		"name":              info.Name,
		"version":           info.Version,
		"registration_mode": info.RegistrationMode,
		"loopback":          info.Loopback,
		"bootstrapped":      info.Bootstrapped,
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
