package api

import (
	"net/http"
	"strings"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/mail"
)

type smtpBody struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
}

type smtpTestBody struct {
	To string `json:"to"`
}

func (s *Server) handleAdminSMTP(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.Mail.LoadConfig(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"host":         cfg.Host,
		"port":         cfg.Port,
		"username":     cfg.Username,
		"from":         cfg.From,
		"configured":   cfg.Host != "" && cfg.From != "",
		"test_sent_at": cfg.TestSentAt,
	})
}

func (s *Server) handleAdminSetSMTP(w http.ResponseWriter, r *http.Request) {
	body, ok := bindJSON[smtpBody](w, r)
	if !ok {
		return
	}
	if body.Password == "" {
		current, err := s.Mail.LoadConfig(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		body.Password = current.Password
	}
	if err := s.Mail.SaveConfig(r.Context(), mail.Config{
		Host:     body.Host,
		Port:     body.Port,
		Username: body.Username,
		Password: body.Password,
		From:     body.From,
	}); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminSMTPTest(w http.ResponseWriter, r *http.Request) {
	body, ok := bindJSON[smtpTestBody](w, r)
	if !ok {
		return
	}
	if err := s.Mail.SendTest(r.Context(), body.To); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminVAPID(w http.ResponseWriter, r *http.Request) {
	if err := s.Push.EnsureKeys(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	pub, err := s.Push.PublicKey(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"public_key": pub})
}

func (s *Server) handleAdminPushTest(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	sess, err := s.Auth.IsAdminSession(r.Context(), token)
	if err != nil {
		writeError(w, auth.ErrForbidden)
		return
	}
	// Панель присылает подписку своего браузера: хранить её под аккаунтом
	// админа нельзя — endpoint уникален, и в общем с участником браузере
	// подписка панели отняла бы у участника его уведомления.
	var body pushSubscribeBody
	if r.ContentLength > 0 {
		if b, ok := bindJSON[pushSubscribeBody](w, r); ok {
			body = b
		} else {
			return
		}
	}
	if strings.TrimSpace(body.Endpoint) != "" {
		err = s.Push.SendTestTo(r.Context(), body.Endpoint, body.P256dh, body.Auth)
	} else {
		err = s.Push.SendTest(r.Context(), sess.AccountID)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
