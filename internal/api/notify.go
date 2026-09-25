package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/push"
)

type notifyPrefsBody struct {
	Posts     *bool `json:"posts"`
	Comments  *bool `json:"comments"`
	Reactions *bool `json:"reactions"`
}

func (s *Server) handleGetAccountNotifyPrefs(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, auth.ErrForbidden)
		return
	}
	prefs, err := s.Auth.AccountNotifyPrefs(r.Context(), sess.AccountID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, prefs)
}

func (s *Server) handleSetAccountNotifyPrefs(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, auth.ErrForbidden)
		return
	}
	var body notifyPrefsBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	prefs := auth.DefaultNotifyPrefs()
	existing, _ := s.Auth.AccountNotifyPrefs(r.Context(), sess.AccountID)
	prefs = existing
	if body.Posts != nil {
		prefs.Posts = *body.Posts
	}
	if body.Comments != nil {
		prefs.Comments = *body.Comments
	}
	if body.Reactions != nil {
		prefs.Reactions = *body.Reactions
	}
	prefs.Mentions = true
	if err := s.Auth.SaveAccountNotifyPrefs(r.Context(), sess.AccountID, prefs); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, prefs)
}

func (s *Server) handleGetCircleNotifyPrefs(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, auth.ErrForbidden)
		return
	}
	circleID := r.PathValue("circle_id")
	prefs, err := s.Auth.CircleNotifyPrefs(r.Context(), sess.AccountID, circleID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, prefs)
}

func (s *Server) handleSetCircleNotifyPrefs(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, auth.ErrForbidden)
		return
	}
	circleID := r.PathValue("circle_id")
	var body notifyPrefsBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	base, err := s.Auth.AccountNotifyPrefs(r.Context(), sess.AccountID)
	if err != nil {
		writeError(w, err)
		return
	}
	prefs := base
	if ov, err := s.Auth.CircleNotifyPrefs(r.Context(), sess.AccountID, circleID); err == nil {
		prefs = ov
	}
	if body.Posts != nil {
		prefs.Posts = *body.Posts
	}
	if body.Comments != nil {
		prefs.Comments = *body.Comments
	}
	if body.Reactions != nil {
		prefs.Reactions = *body.Reactions
	}
	prefs.Mentions = true
	if err := s.Auth.SaveCircleNotifyPrefs(r.Context(), sess.AccountID, circleID, prefs, base); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, prefs)
}

func (s *Server) notifyCircle(circleID, actorAccountID, signalType string) {
	if s == nil || s.Push == nil {
		return
	}
	s.notifyWG.Add(1)
	go func() {
		defer s.notifyWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ids, err := s.Chronicle.CircleMemberAccountIDs(ctx, circleID)
		if err != nil {
			log.Printf("notifyCircle: members %s: %v", circleID, err)
			return
		}
		for _, accountID := range ids {
			if accountID == actorAccountID {
				continue
			}
			prefs, err := s.Auth.CircleNotifyPrefs(ctx, accountID, circleID)
			if err != nil {
				log.Printf("notifyCircle: prefs %s/%s: %v", accountID, circleID, err)
				continue
			}
			if !auth.NotifyPrefAllows(prefs, signalType) {
				continue
			}
			if err := s.Push.SendSignal(ctx, accountID, push.Signal{
				CircleID: circleID,
				Type:     signalType,
				Count:    1,
			}); err != nil {
				log.Printf("notifyCircle: push %s/%s: %v", accountID, circleID, err)
			}
		}
	}()
}

type pushSubscribeBody struct {
	Endpoint string `json:"endpoint"`
	P256dh   string `json:"p256dh"`
	Auth     string `json:"auth"`
}

func (s *Server) handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, auth.ErrForbidden)
		return
	}
	var body pushSubscribeBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if err := s.Push.EnsureKeys(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	if err := s.Push.Subscribe(r.Context(), push.SubscribeInput{
		AccountID: sess.AccountID,
		Endpoint:  body.Endpoint,
		P256dh:    body.P256dh,
		Auth:      body.Auth,
		UserAgent: r.UserAgent(),
		Now:       time.Now().UTC(),
	}); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handlePushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, auth.ErrForbidden)
		return
	}
	var body pushSubscribeBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if err := s.Push.Unsubscribe(r.Context(), sess.AccountID, body.Endpoint); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type quotaRequestBody struct {
	RequestedBytes int64 `json:"requested_bytes"`
}

func (s *Server) handleCreateQuotaRequest(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, auth.ErrForbidden)
		return
	}
	circleID := r.PathValue("circle_id")
	if err := s.Chronicle.RequireOwner(r.Context(), circleID, sess.AccountID); err != nil {
		writeError(w, err)
		return
	}
	var body quotaRequestBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	id, err := s.Blobs.CreateQuotaRequest(r.Context(), circleID, sess.AccountID, body.RequestedBytes)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (s *Server) handleAdminQuotaRequests(w http.ResponseWriter, r *http.Request) {
	items, err := s.Blobs.ListPendingQuotaRequests(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	type row struct {
		ID             string  `json:"id"`
		CircleID       string  `json:"circle_id"`
		RequesterID    string  `json:"requester_id"`
		RequesterMail  string  `json:"requester_email"`
		RequestedBytes int64   `json:"requested_bytes"`
		Status         string  `json:"status"`
		AdminNote      *string `json:"admin_note,omitempty"`
		CreatedAt      string  `json:"created_at"`
		ResolvedAt     *string `json:"resolved_at,omitempty"`
	}
	out := make([]row, len(items))
	for i, item := range items {
		out[i] = row{
			ID: item.ID, CircleID: item.CircleID, RequesterID: item.RequesterID,
			RequesterMail: item.RequesterEmail, RequestedBytes: item.RequestedBytes,
			Status: item.Status, AdminNote: item.AdminNote, CreatedAt: item.CreatedAt,
			ResolvedAt: item.ResolvedAt,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"requests": out})
}

type quotaResolveBody struct {
	AdminNote string `json:"admin_note"`
}

func (s *Server) handleAdminApproveQuotaRequest(w http.ResponseWriter, r *http.Request) {
	s.resolveQuotaRequest(w, r, true)
}

func (s *Server) handleAdminRejectQuotaRequest(w http.ResponseWriter, r *http.Request) {
	s.resolveQuotaRequest(w, r, false)
}

func (s *Server) resolveQuotaRequest(w http.ResponseWriter, r *http.Request, approve bool) {
	id := r.PathValue("id")
	var body quotaResolveBody
	_ = readJSON(r, &body)
	status, err := s.Blobs.ResolveQuotaRequest(r.Context(), id, approve, body.AdminNote)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}
