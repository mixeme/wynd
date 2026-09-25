package api

import (
	"log"
	"net/http"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func (s *Server) handleListCircles(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	circles, err := s.Chronicle.ListAccountCircles(r.Context(), sess.AccountID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]map[string]any, len(circles))
	for i, c := range circles {
		item := map[string]any{
			"id":            c.ID,
			"name":          c.Name,
			"color":         c.Color,
			"status":        string(c.Status),
			"unread":        c.Unread,
			"last_read_seq": c.LastReadSeq,
		}
		if c.LastSummary != "" && c.LastAt != nil {
			item["last_summary"] = c.LastSummary
			item["last_at"] = c.LastAt.UTC().Format(time.RFC3339)
		}
		if banner, err := s.archiveCycleJSON(r.Context(), c.ID, sess.AccountID); err == nil && banner != nil {
			item["archive_cycle"] = banner
		}
		out[i] = item
	}
	writeJSON(w, http.StatusOK, map[string]any{"circles": out})
}

type createCircleBody struct {
	Name          string  `json:"name"`
	OwnerName     string  `json:"owner_name"`
	EditWindowSec *int64  `json:"edit_window_sec"`
	Color         *string `json:"color"`
}

func (s *Server) handleCreateCircle(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[createCircleBody](w, r)
	if !ok {
		return
	}
	window := chronicle.UnlimitedWindow()
	if body.EditWindowSec != nil {
		// Та же проверка, что и в PatchCircle: отрицательное окно правок
		// круга — invalid. При создании её не было (аудит 2026-09-22).
		if *body.EditWindowSec < 0 {
			writeError(w, chronicle.ErrInvalid)
			return
		}
		window = chronicle.EditWindow{Seconds: body.EditWindowSec}
	}
	color := "ochre"
	if body.Color != nil {
		color = *body.Color
	}
	circle, _, _, err := s.Chronicle.CreateCircle(r.Context(), chronicle.CreateCircleInput{
		Name: body.Name, OwnerAccountID: sess.AccountID, OwnerName: body.OwnerName,
		Color: color, EditWindow: window, Now: time.Now().UTC(),
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": circle.ID, "name": circle.Name, "color": circle.Color,
	})
}

type createCircleInviteBody struct {
	Kind    string `json:"kind"`
	MaxUses int    `json:"max_uses"`
	TTLSec  int    `json:"ttl_sec"`
}

func (s *Server) handleCreateCircleInvite(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.RequireCanInvite(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	body, ok := bindJSON[createCircleInviteBody](w, r)
	if !ok {
		return
	}
	kind := auth.InviteSingle
	if body.Kind == "multi" {
		kind = auth.InviteMulti
	}
	if kind == auth.InviteMulti {
		settings, err := s.Chronicle.GetInviteSettings(r.Context(), circleID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		if settings.InviteKindDefault == "single" {
			writeDomainError(w, chronicle.ErrInvalid)
			return
		}
	}
	maxUses := body.MaxUses
	if maxUses < 1 {
		maxUses = 1
	}
	ttl := 3 * 24 * time.Hour
	if body.TTLSec > 0 {
		ttl = time.Duration(body.TTLSec) * time.Second
	}
	inv, err := s.Auth.CreateInvite(r.Context(), auth.CreateInviteInput{
		CircleID: circleID, Kind: kind, MaxUses: maxUses, TTL: ttl,
		CreatedByAccountID: sess.AccountID, Now: time.Now().UTC(),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         inv.ID,
		"token":      inv.Token,
		"expires_at": inv.ExpiresAt.UTC().Format(time.RFC3339),
		"max_uses":   inv.MaxUses,
	})
}

func (s *Server) handleListCircleInvites(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.RequireCanInvite(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	invites, err := s.Auth.ListCircleInvites(r.Context(), circleID, time.Now().UTC())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"invites": invites})
}

func (s *Server) handleRevokeCircleInvite(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	inviteID := r.PathValue("id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.RequireCanInvite(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	if err := s.Auth.RevokeCircleInvite(r.Context(), circleID, inviteID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type leaveCircleBody struct {
	RetainAccess bool `json:"retain_access"`
}

func (s *Server) handleLeaveCircle(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	var body leaveCircleBody
	if r.ContentLength != 0 {
		if err := readJSON(r, &body); err != nil {
			writeError(w, err)
			return
		}
	}
	now := time.Now().UTC()
	var err error
	if body.RetainAccess {
		err = s.Chronicle.LeaveWithAccess(r.Context(), circleID, sess.AccountID, now)
	} else {
		err = s.Chronicle.Leave(r.Context(), circleID, sess.AccountID, now)
	}
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// Ушедший не оставляет живой ссылки в круг (SEC-9).
	if err := s.Auth.RevokeCircleInvitesBy(r.Context(), circleID, sess.AccountID, now); err != nil {
		log.Printf("leaveCircle: revoke invites %s/%s: %v", circleID, sess.AccountID, err)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type readCursorBody struct {
	Seq int64 `json:"seq"`
}

func (s *Server) handleSetReadCursor(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[readCursorBody](w, r)
	if !ok {
		return
	}
	err := s.Chronicle.SetReadCursor(r.Context(), sess.AccountID, circleID, body.Seq, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
