package api

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/config"
	"gitea.mixdep.ru/mix/wynd/internal/mail"
)

type bootstrapBody struct {
	Token        string `json:"token"`
	InstanceName string `json:"instance_name"`
	Password     string `json:"password"`
	PublicURL    string `json:"public_url"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	SMTPPassword string `json:"smtp_password"`
	From         string `json:"from"`
}

func smtpReady(body bootstrapBody) bool {
	return strings.TrimSpace(body.Host) != "" && strings.TrimSpace(body.From) != "" && body.SMTPPassword != ""
}

type adminLoginBody struct {
	Password string `json:"password"`
}

type createInviteBody struct {
	Kind          string `json:"kind"`
	MaxUses       int    `json:"max_uses"`
	TTLSec        int    `json:"ttl_sec"`
	UnlimitedUses bool   `json:"unlimited_uses"`
	NoExpiry      bool   `json:"no_expiry"`
}

// handleBootstrap выполняет первичную установку в порядке, при котором до
// проверки токена не происходит ничего: токен и флаг bootstrapped → пароль →
// проба несохранённого релея → одна транзакция (админ + SMTP) → public_url →
// проверочное письмо. Сбой письма установку не отменяет: он записан в
// smtp_last_error, ответ 200 с mail_sent:false.
func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	body, ok := bindJSON[bootstrapBody](w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	now := time.Now().UTC()
	// Адрес проверяется строго и до всего остального: раньше установка
	// принимала любую строку с «://», писала её в config.json — и сервер
	// после перезапуска отказывался стартовать, потому что config.Load
	// проверяет адрес строго (аудит 2026-09-22).
	publicURL := s.PublicURL()
	if strings.TrimSpace(body.PublicURL) != "" {
		valid, err := config.ValidatePublicURL(body.PublicURL)
		if err != nil {
			writeError(w, auth.ErrInvalid)
			return
		}
		publicURL = valid
	}
	if err := s.Auth.ConfirmBootstrapToken(ctx, body.Token, s.BootstrapToken, s.clientIP(r), now); err != nil {
		writeError(w, err)
		return
	}
	if err := auth.ValidatePassword(body.Password); err != nil {
		writeError(w, err)
		return
	}
	// Почта при установке необязательна и на публичном адресе: админ входит
	// по паролю и настраивает релей позже в панели, «Проверка» покажет, что
	// письма не уходят. Неполный релей не сохраняется — клиент не шлёт его.
	ready := smtpReady(body)

	var smtpCfg *mail.Config
	if ready {
		port := body.Port
		if port <= 0 {
			port = 587
		}
		cfg := mail.Config{
			Host:     body.Host,
			Port:     port,
			Username: body.Username,
			Password: body.SMTPPassword,
			From:     body.From,
		}
		if err := s.Mail.Probe(ctx, cfg); err != nil {
			writeError(w, err)
			return
		}
		smtpCfg = &cfg
	}

	err := s.Auth.Bootstrap(ctx, auth.BootstrapInput{
		Token:        body.Token,
		InstanceName: body.InstanceName,
		Password:     body.Password,
		ClientIP:     s.clientIP(r),
		Now:          now,
		InTx: func(ctx context.Context, tx *sql.Tx) error {
			if smtpCfg == nil {
				return nil
			}
			return s.Mail.SaveConfigTx(ctx, tx, *smtpCfg)
		},
	}, s.BootstrapToken)
	if err != nil {
		writeError(w, err)
		return
	}

	if strings.TrimSpace(body.PublicURL) != "" {
		if err := config.WritePublicURL(s.DataDir, publicURL); err != nil {
			writeError(w, err)
			return
		}
		s.applyPublicURL(publicURL)
	}

	mailSent := false
	if smtpCfg != nil {
		if err := s.Mail.SendTest(ctx, strings.TrimSpace(body.From)); err != nil {
			log.Printf("api: bootstrap test mail: %v", err)
		} else {
			mailSent = true
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "mail_sent": mailSent})
}

func (s *Server) handleBootstrapSMTPTest(w http.ResponseWriter, r *http.Request) {
	body, ok := bindJSON[bootstrapBody](w, r)
	if !ok {
		return
	}
	if err := s.Auth.ConfirmBootstrapToken(r.Context(), body.Token, s.BootstrapToken, s.clientIP(r), time.Now().UTC()); err != nil {
		writeError(w, err)
		return
	}
	if strings.TrimSpace(body.Host) == "" {
		writeError(w, mail.ErrNotConfigured)
		return
	}
	port := body.Port
	if port <= 0 {
		port = 587
	}
	if err := s.Mail.Probe(r.Context(), mail.Config{
		Host:     body.Host,
		Port:     port,
		Username: body.Username,
		Password: body.SMTPPassword,
		From:     body.From,
	}); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) applyPublicURL(publicURL string) {
	loopback := config.IsLoopback(publicURL)
	s.publicURLMu.Lock()
	s.publicURL = publicURL
	s.publicURLMu.Unlock()
	s.loopback.Store(loopback)
	if s.Auth != nil {
		s.Auth.SetLoopback(loopback)
	}
	if s.Mail != nil {
		s.Mail.SetLoopback(loopback)
	}
}

type passwordBody struct {
	Current string `json:"current"`
	New     string `json:"new"`
}

func (s *Server) handleAdminSetPassword(w http.ResponseWriter, r *http.Request) {
	body, ok := bindJSON[passwordBody](w, r)
	if !ok {
		return
	}
	if err := s.Auth.ChangeAdminPassword(r.Context(), body.Current, body.New, time.Now().UTC()); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	body, ok := bindJSON[adminLoginBody](w, r)
	if !ok {
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
	body, ok := bindJSON[createInviteBody](w, r)
	if !ok {
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
	}
	sess, err := s.Auth.IsAdminSession(r.Context(), bearerToken(r))
	if err != nil {
		writeError(w, err)
		return
	}
	inv, err := s.Auth.CreateServerInvite(r.Context(), auth.CreateServerInviteInput{
		Kind:               kind,
		MaxUses:            maxUses,
		TTL:                ttl,
		UnlimitedUses:      body.UnlimitedUses,
		NoExpiry:           body.NoExpiry,
		CreatedByAccountID: sess.AccountID,
		Now:                time.Now().UTC(),
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

// RequirePaidParticipant blocks circle API when subscription is required but expired.
// Pay routes stay on RequireParticipant so users can submit payment.
func (s *Server) RequirePaidParticipant(next http.HandlerFunc) http.HandlerFunc {
	return s.RequireParticipant(func(w http.ResponseWriter, r *http.Request) {
		if err := s.requirePaidSession(r); err != nil {
			writeError(w, err)
			return
		}
		next(w, r)
	})
}

func (s *Server) requirePaidSession(r *http.Request) error {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		return auth.ErrForbidden
	}
	locked, err := s.Auth.PaymentRequired(r.Context(), sess.AccountID, time.Now().UTC())
	if err != nil {
		return err
	}
	if locked {
		return auth.ErrPaymentRequired
	}
	return nil
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
