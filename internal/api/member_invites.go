package api

import (
	"net/http"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func (s *Server) handleListInviteCandidates(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	now := time.Now().UTC()
	invited, err := s.Auth.MemberInviteTargets(r.Context(), circleID, now)
	if err != nil {
		writeError(w, err)
		return
	}
	groups, err := s.Chronicle.ListInviteCandidates(r.Context(), circleID, sess.AccountID, invited)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups})
}

type createMemberInviteBody struct {
	AccountID string `json:"account_id"`
}

func (s *Server) handleCreateMemberInvite(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	if err := s.Chronicle.RequireCanInvite(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	var body createMemberInviteBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	target := strings.TrimSpace(body.AccountID)
	if target == "" {
		writeError(w, chronicle.ErrInvalid)
		return
	}
	okTarget, err := s.Chronicle.CanInviteAccount(r.Context(), circleID, sess.AccountID, target)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if !okTarget {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	now := time.Now().UTC()
	inv, err := s.Auth.CreateMemberInvite(r.Context(), auth.CreateMemberInviteInput{
		CircleID:           circleID,
		TargetAccountID:    target,
		CreatedByAccountID: sess.AccountID,
		Now:                now,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.Chronicle.RecordMemberInvited(r.Context(), circleID, sess.AccountID, now); err != nil {
		writeDomainError(w, err)
		return
	}
	s.notifyMemberInvited(circleID, target)
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     inv.ID,
		"status": "invited",
	})
}

func (s *Server) handleListPendingCircleJoins(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, auth.ErrForbidden)
		return
	}
	ids, err := s.Auth.ListPendingCircleJoins(r.Context(), sess.AccountID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"circle_ids": ids})
}

func (s *Server) handleJoinPreview(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	okPending, err := s.Auth.HasPendingCircleJoin(r.Context(), sess.AccountID, circleID, time.Now().UTC())
	if err != nil {
		writeError(w, err)
		return
	}
	if !okPending {
		writeDomainError(w, chronicle.ErrForbidden)
		return
	}
	peek, err := s.Chronicle.InvitePeekForCircle(r.Context(), circleID, "")
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"circle_name":  peek.CircleName,
		"color":        peek.Color,
		"member_count": peek.MemberCount,
		"members":      peek.Members,
	})
}

type joinPendingBody struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

func (s *Server) handleJoinPendingCircle(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	var body joinPendingBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
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
		CircleID:  circleID,
		Name:      name,
		Now:       now,
	}); err != nil {
		writeError(w, err)
		return
	}
	if text := strings.TrimSpace(body.Body); text != "" {
		entryDate := now.Format("2006-01-02")
		if _, err := s.Chronicle.CreatePost(r.Context(), chronicle.PostInput{
			CircleID:  circleID,
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
		"circle_id": circleID,
	})
}
