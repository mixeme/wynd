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
	sess, ok := requireSession(w, r)
	if !ok {
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
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.RequireCanInvite(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	body, ok := bindJSON[createMemberInviteBody](w, r)
	if !ok {
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
	sess, ok := requireSession(w, r)
	if !ok {
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
	sess, ok := requireSession(w, r)
	if !ok {
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
	inviter, err := s.Auth.PendingJoinInviter(r.Context(), sess.AccountID, circleID)
	if err != nil {
		writeError(w, err)
		return
	}
	peek, err := s.Chronicle.InvitePeekForCircle(r.Context(), circleID, inviter, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := map[string]any{
		"circle_name":  peek.CircleName,
		"color":        peek.Color,
		"member_count": peek.MemberCount,
		"members":      peek.Members,
	}
	// Отделённый круг (A9, 6.44): кто зовёт, откуда, под каким именем войдёте.
	split, err := s.Chronicle.SplitJoinFor(r.Context(), circleID, sess.AccountID, inviter)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if split != nil {
		out["split"] = split
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeclinePendingJoin(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Auth.DeclinePendingJoin(r.Context(), sess.AccountID, r.PathValue("circle_id"), time.Now().UTC()); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "declined"})
}

type joinPendingBody struct {
	Name   string `json:"name"`
	Gender string `json:"gender"`
	Body   string `json:"body"`
	// EntryDate — день устройства для первой записи (greetingDay).
	EntryDate string `json:"entry_date"`
}

func (s *Server) handleJoinPendingCircle(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[joinPendingBody](w, r)
	if !ok {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeError(w, chronicle.ErrInvalid)
		return
	}
	gender, err := chronicle.ParseGender(body.Gender)
	if err != nil {
		writeError(w, err)
		return
	}
	now := time.Now().UTC()
	if err := s.Auth.CompleteCircleJoin(r.Context(), auth.CompleteCircleJoinInput{
		AccountID: sess.AccountID,
		CircleID:  circleID,
		Name:      name,
		Gender:    gender,
		Now:       now,
	}); err != nil {
		writeError(w, err)
		return
	}
	if text := strings.TrimSpace(body.Body); text != "" {
		entryDate := greetingDay(body.EntryDate, now)
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
