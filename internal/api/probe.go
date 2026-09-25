package api

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/proxy"
)

const maxBrowserBodyProbeBytes = 2 * 1024 * 1024

func (s *Server) bodyProbeBytes(ctx context.Context) int64 {
	limit := int64(maxBrowserBodyProbeBytes)
	if s.Blobs != nil {
		if cs, err := s.Blobs.LoadCompressionSettings(ctx); err == nil && cs.AttachmentMaxBytes > 0 {
			if cs.AttachmentMaxBytes < limit {
				limit = cs.AttachmentMaxBytes
			}
		}
	}
	return limit
}

func requestScheme(r *http.Request) string {
	if proto := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))); proto != "" {
		return proto
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func (s *Server) handleProbe(w http.ResponseWriter, r *http.Request) {
	client := s.clientIP(r)
	ip := net.ParseIP(client)
	peerLoopback := ip != nil && ip.IsLoopback()
	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	xri := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	writeJSON(w, http.StatusOK, map[string]any{
		"proto":                  requestScheme(r),
		"peer_loopback":          peerLoopback,
		"body_probe_bytes":       s.bodyProbeBytes(r.Context()),
		"proxy_read_timeout_sec": proxy.ReadTimeoutSeconds,
		"client_ip":              client,
		"x_forwarded_for":        xff,
		"x_real_ip":              xri,
	})
}

func (s *Server) handleProbeStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, auth.ErrInvalid)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-cache")
	const chunks = 20
	for i := 0; i < chunks; i++ {
		if _, err := w.Write([]byte("x")); err != nil {
			return
		}
		flusher.Flush()
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *Server) handleProbeSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, auth.ErrInvalid)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	fmt.Fprintf(w, "event: probe\ndata: ok\n\n")
	flusher.Flush()
}

func (s *Server) handleProbeBody(w http.ResponseWriter, r *http.Request) {
	// Тот же потолок, что объявлен в GET /probe: аноним лил до
	// attachment_max (100 МиБ) вместо заявленных 2 МиБ (аудит 2026-09-22).
	r.Body = http.MaxBytesReader(w, r.Body, s.bodyProbeBytes(r.Context()))
	if _, err := io.Copy(io.Discard, r.Body); err != nil {
		writeError(w, errBodyTooLarge)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
