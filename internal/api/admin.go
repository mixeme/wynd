package api

import (
	"context"
	"net/http"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

type bootstrapBody struct {
	Token        string `json:"token"`
	InstanceName string `json:"instance_name"`
	Password     string `json:"password"`
}

type adminLoginBody struct {
	Password string `json:"password"`
}

type createInviteBody struct {
	Kind    string `json:"kind"`
	MaxUses int    `json:"max_uses"`
	TTLDays int    `json:"ttl_days"`
	TTLSec  int    `json:"ttl_sec"`
}

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	var body bootstrapBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	err := s.Auth.Bootstrap(r.Context(), auth.BootstrapInput{
		Token:        body.Token,
		InstanceName: body.InstanceName,
		Password:     body.Password,
		ClientIP:     s.clientIP(r),
		Now:          time.Now().UTC(),
	}, s.BootstrapToken)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	var body adminLoginBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	sess, err := s.Auth.AdminLogin(r.Context(), auth.AdminLoginInput{
		Password: body.Password,
		ClientIP: s.clientIP(r),
		Now:      time.Now().UTC(),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"token":      sess.Token,
		"expires_at": sess.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

// handleAdminLogout revokes the admin session named by the Bearer token.
func (s *Server) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.Auth.RevokeSession(r.Context(), bearerToken(r)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCreateServerInvite(w http.ResponseWriter, r *http.Request) {
	var body createInviteBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	kind := auth.InviteMulti
	if body.Kind == "single" {
		kind = auth.InviteSingle
	}
	maxUses := body.MaxUses
	if maxUses < 1 {
		maxUses = 1
	}
	ttl := 7 * 24 * time.Hour
	if body.TTLSec > 0 {
		ttl = time.Duration(body.TTLSec) * time.Second
	} else if body.TTLDays > 0 {
		ttl = time.Duration(body.TTLDays) * 24 * time.Hour
	}
	inv, err := s.Auth.CreateServerInvite(r.Context(), auth.CreateServerInviteInput{
		Kind:    kind,
		MaxUses: maxUses,
		TTL:     ttl,
		Now:     time.Now().UTC(),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":      inv.Token,
		"expires_at": inv.ExpiresAt.UTC().Format(time.RFC3339),
		"max_uses":   inv.MaxUses,
	})
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return limitBody(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			writeError(w, auth.ErrForbidden)
			return
		}
		if _, err := s.Auth.IsAdminSession(r.Context(), token); err != nil {
			writeError(w, auth.ErrForbidden)
			return
		}
		next(w, r)
	})
}

// RequireParticipant authenticates a participant session and caps the JSON
// body. The upload chunk route uses RequireParticipantStream instead.
func (s *Server) RequireParticipant(next http.HandlerFunc) http.HandlerFunc {
	return limitBody(s.RequireParticipantStream(next))
}

// RequireParticipantStream authenticates without a body cap; the chunk
// writer bounds bytes itself against the upload session size.
func (s *Server) RequireParticipantStream(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			writeError(w, auth.ErrForbidden)
			return
		}
		sess, err := s.Auth.IsParticipantSession(r.Context(), token)
		if err != nil {
			writeError(w, auth.ErrForbidden)
			return
		}
		if err := auth.RejectAdminJournal(sess.Kind); err != nil {
			writeError(w, err)
			return
		}
		ctx := contextWithSession(r.Context(), sess)
		next(w, r.WithContext(ctx))
	}
}

type sessionKey struct{}

func contextWithSession(ctx context.Context, sess auth.Session) context.Context {
	return context.WithValue(ctx, sessionKey{}, sess)
}

// SessionFromContext returns the participant session stored by RequireParticipant.
func SessionFromContext(ctx context.Context) (auth.Session, bool) {
	sess, ok := ctx.Value(sessionKey{}).(auth.Session)
	return sess, ok
}
