package api

import (
	"net"
	"net/http"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

type acceptInviteBody struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type joinInviteBody struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

func (s *Server) handleAcceptInvite(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	body, ok := bindJSON[acceptInviteBody](w, r)
	if !ok {
		return
	}
	err := s.Auth.AcceptInvite(r.Context(), auth.AcceptInviteInput{
		Token:    token,
		Email:    body.Email,
		Name:     body.Name,
		ClientIP: s.clientIP(r),
		Now:      time.Now().UTC(),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "code_sent"})
}

func (s *Server) handlePeekInvite(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	inv, err := s.Auth.InviteByToken(r.Context(), token)
	if err != nil {
		writeError(w, err)
		return
	}
	now := time.Now().UTC()
	// Та же проверка, что и при использовании: раньше просмотр показывал
	// круг и по исчерпанной ссылке, и на закрытом сервере (SEC-9).
	if err := s.Auth.ValidateInvite(r.Context(), inv, now); err != nil {
		writeError(w, err)
		return
	}

	info, err := s.Auth.Instance(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	host := s.inviteHost(r)
	if inv.IsServer() {
		name, err := s.Chronicle.ServerInviteInviterName(r.Context(), inv.CreatedByAccountID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		out := map[string]any{
			"server_name": info.Name,
			"host":        host,
		}
		if name != "" {
			out["inviter_name"] = name
		}
		writeJSON(w, http.StatusOK, out)
		return
	}

	peek, err := s.Chronicle.InvitePeekForCircle(r.Context(), inv.CircleID, inv.CreatedByAccountID, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"server_name":  info.Name,
		"host":         host,
		"circle_name":  peek.CircleName,
		"color":        peek.Color,
		"member_count": peek.MemberCount,
		"members":      peek.Members,
	})
}

func (s *Server) handleClaimInvite(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	res, err := s.Auth.ClaimInvite(r.Context(), sess.AccountID, token, time.Now().UTC())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"circle_id":      res.CircleID,
		"already_member": res.AlreadyMember,
	})
}

func (s *Server) handleJoinInvite(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	inv, err := s.Auth.InviteByToken(r.Context(), token)
	if err != nil {
		writeError(w, err)
		return
	}
	if inv.IsServer() {
		writeError(w, chronicle.ErrNotFound)
		return
	}

	body, ok := bindJSON[joinInviteBody](w, r)
	if !ok {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeError(w, chronicle.ErrInvalid)
		return
	}

	now := time.Now().UTC()
	if err := s.Auth.CompleteCircleJoin(r.Context(), auth.CompleteCircleJoinInput{
		AccountID: sess.AccountID,
		CircleID:  inv.CircleID,
		Name:      name,
		Now:       now,
	}); err != nil {
		writeError(w, err)
		return
	}

	if text := strings.TrimSpace(body.Body); text != "" {
		entryDate := now.Format("2006-01-02")
		if _, err := s.Chronicle.CreatePost(r.Context(), chronicle.PostInput{
			CircleID:  inv.CircleID,
			AccountID: sess.AccountID,
			Body:      text,
			EntryDate: entryDate,
			Now:       now,
		}); err != nil {
			writeDomainError(w, err)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":    "ok",
		"circle_id": inv.CircleID,
	})
}

func (s *Server) inviteHost(r *http.Request) string {
	if s.PublicURL() != "" {
		return publicHost(s.PublicURL())
	}
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return host
}
