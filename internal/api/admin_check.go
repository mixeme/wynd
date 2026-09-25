package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/check"
	"gitea.mixdep.ru/mix/wynd/internal/jobs"
)

type checkBody struct {
	External *check.ExternalReport `json:"external,omitempty"`
}

func (s *Server) handleAdminCheck(w http.ResponseWriter, r *http.Request) {
	var body checkBody
	if r.ContentLength > 0 {
		_ = readJSON(r, &body)
	}
	external := body.External
	if external != nil && !s.Loopback && strings.HasPrefix(s.PublicURL, "https://") {
		external.RedirectPermanent = check.ProbeHTTPRedirect(s.PublicURL)
	}
	results, err := s.runChecks(r, external)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"checks": results})
}

func (s *Server) runChecks(r *http.Request, external *check.ExternalReport) ([]check.Result, error) {
	ctx := r.Context()
	mailOK, _ := s.Mail.Configured(ctx)
	mailCfg, _ := s.Mail.LoadConfig(ctx)
	_ = s.Push.EnsureKeys(ctx)
	pub, _ := s.Push.PublicKey(ctx)
	routineAt, backupAt := s.loadInstanceTimestamps(ctx)
	return check.RunChecks(ctx, check.Input{
		Loopback:        s.Loopback,
		PublicURL:       s.PublicURL,
		DataDir:         s.DataDir,
		MailConfigured:  mailOK,
		SMTPTestSentAt:  mailCfg.TestSentAt,
		VAPIDConfigured: pub != "",
		LastRoutineAt:   routineAt,
		LastBackupAt:    backupAt,
		External:        external,
		Now:             time.Now().UTC(),
	}), nil
}

func (s *Server) handleAdminRunRoutine(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	counts, err := jobs.RunDailyRoutine(r.Context(), s.Auth.DB(), s.Blobs.Dir(), now)
	if err != nil {
		writeError(w, err)
		return
	}
	archiveCounts, err := jobs.RunArchiveJobs(r.Context(), s.Auth.DB(), s.Chronicle, s.Blobs, s.Mail, s.PublicURL, now)
	if err != nil {
		writeError(w, err)
		return
	}
	payCounts, err := jobs.RunPayJobs(r.Context(), s.Auth, s.Mail, s.Push, s.Blobs.Dir(), now)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"routine": counts,
		"archive": archiveCounts,
		"pay":     payCounts,
	})
}

func (s *Server) handleAdminProxySnippet(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	maxBytes := int64(104857600)
	if s.Blobs != nil {
		if cs, err := s.Blobs.LoadCompressionSettings(r.Context()); err == nil && cs.AttachmentMaxBytes > 0 {
			maxBytes = cs.AttachmentMaxBytes
		}
	}
	var snippet string
	switch kind {
	case "nginx":
		snippet = nginxSnippet(s.ListenAddr, maxBytes)
	case "caddy":
		snippet = caddySnippet(s.PublicURL, s.ListenAddr, maxBytes)
	case "traefik":
		snippet = traefikSnippet(s.ListenAddr, maxBytes)
	default:
		writeError(w, auth.ErrNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"kind": kind, "snippet": snippet})
}

func nginxSnippet(listen string, maxBytes int64) string {
	port := listenPort(listen)
	return `# Wynd reverse proxy (nginx)
location / {
    proxy_pass http://127.0.0.1:` + port + `;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    proxy_buffering off;
    proxy_read_timeout 300s;
    client_max_body_size ` + bodySizeNginx(maxBytes) + `;
}
`
}

func caddySnippet(publicURL, listen string, maxBytes int64) string {
	host := publicHost(publicURL)
	port := listenPort(listen)
	return host + ` {
    header Strict-Transport-Security "max-age=31536000; includeSubDomains"
    reverse_proxy 127.0.0.1:` + port + ` {
        flush_interval -1
        transport http {
            read_timeout 300s
        }
    }
    request_body {
        max_size ` + bodySizeCaddy(maxBytes) + `
    }
}
`
}

func traefikSnippet(listen string, maxBytes int64) string {
	port := listenPort(listen)
	return `# Wynd (Traefik dynamic config)
http:
  services:
    wynd:
      loadBalancer:
        servers:
          - url: "http://127.0.0.1:` + port + `"
  middlewares:
    wynd-headers:
      headers:
        customRequestHeaders:
          X-Forwarded-Proto: "https"
        stsSeconds: 31536000
        stsIncludeSubdomains: true
    wynd-body:
      buffering:
        maxRequestBodyBytes: ` + fmt.Sprintf("%d", maxBytes) + `
`
}

func bodySizeNginx(maxBytes int64) string {
	mb := maxBytes / (1024 * 1024)
	if mb < 1 {
		mb = 1
	}
	return fmt.Sprintf("%dm", mb)
}

func bodySizeCaddy(maxBytes int64) string {
	mb := maxBytes / (1024 * 1024)
	if mb < 1 {
		mb = 1
	}
	return fmt.Sprintf("%dMB", mb)
}

func listenPort(listen string) string {
	for i := len(listen) - 1; i >= 0; i-- {
		if listen[i] == ':' {
			return listen[i+1:]
		}
	}
	return "7676"
}

func publicHost(publicURL string) string {
	u := publicURL
	if i := len(u); i > 0 {
		if j := indexAny(u, "://"); j >= 0 {
			u = u[j+3:]
		}
		if k := indexAny(u, "/"); k >= 0 {
			u = u[:k]
		}
	}
	if u == "" {
		return "example.org"
	}
	return u
}

func indexAny(s, sep string) int {
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}
