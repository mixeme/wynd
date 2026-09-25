package api

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
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
	writeJSON(w, http.StatusOK, map[string]any{
		"proto":            requestScheme(r),
		"peer_loopback":    peerLoopback,
		"body_probe_bytes": s.bodyProbeBytes(r.Context()),
	})
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
	limit := int64(104857600)
	if s.Blobs != nil {
		if cs, err := s.Blobs.LoadCompressionSettings(r.Context()); err == nil && cs.AttachmentMaxBytes > 0 {
			limit = cs.AttachmentMaxBytes
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if _, err := io.Copy(io.Discard, r.Body); err != nil {
		writeError(w, errBodyTooLarge)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
