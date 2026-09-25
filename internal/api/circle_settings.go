package api

import (
	"log"
	"net/http"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func (s *Server) handleCircleMembers(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	members, err := s.Chronicle.ListMembers(r.Context(), circleID, sess.AccountID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]map[string]any, len(members))
	for i, m := range members {
		out[i] = map[string]any{
			"account_id":   m.AccountID,
			"identity_id":  m.IdentityID,
			"name":         m.Name,
			"status":       string(m.Status),
			"can_settings": m.CanSettings,
			"is_owner":     m.IsOwner,
			"joined_at":    m.JoinedAt.UTC().Format(time.RFC3339),
			"can_read":     m.CanRead,
			"can_write":    m.CanWrite,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": out})
}

type patchCircleBody struct {
	Name               *string `json:"name"`
	EditWindowSec      *int64  `json:"edit_window_sec"`
	Color              *string `json:"color"`
	InviteWho          *string `json:"invite_who"`
	InviteKindDefault  *string `json:"invite_kind_default"`
}

func (s *Server) handlePatchCircle(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	var body patchCircleBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	// Один вызов домена: значения проверяются до первой записи, изменения
	// применяются одной транзакцией (QLT-3).
	in := chronicle.PatchCircleInput{
		Name:              body.Name,
		Color:             body.Color,
		InviteWho:         body.InviteWho,
		InviteKindDefault: body.InviteKindDefault,
	}
	if body.EditWindowSec != nil {
		in.EditWindow = &chronicle.EditWindow{Seconds: body.EditWindowSec}
	}
	if err := s.Chronicle.PatchCircle(r.Context(), circleID, sess.AccountID, in, time.Now().UTC()); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type updateIdentityBody struct {
	Name           *string `json:"name"`
	AvatarBlobID   *string `json:"avatar_blob_id"`
}

func (s *Server) handleUpdateIdentity(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	var body updateIdentityBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if body.AvatarBlobID != nil && *body.AvatarBlobID != "" {
		if err := s.Blobs.ValidateOwnedComplete(r.Context(), sess.AccountID, []string{*body.AvatarBlobID}); err != nil {
			writeDomainError(w, err)
			return
		}
	}
	err := s.Chronicle.UpdateIdentity(r.Context(), circleID, sess.AccountID, chronicle.UpdateIdentityInput{
		Name:         body.Name,
		AvatarBlobID: body.AvatarBlobID,
	}, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type setMemberBody struct {
	CanSettings bool `json:"can_settings"`
}

func (s *Server) handleSetMember(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	targetID := r.PathValue("account_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	var body setMemberBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	err := s.Chronicle.SetMemberCanSettings(r.Context(), circleID, sess.AccountID, targetID, body.CanSettings, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleIdentityHistory(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	mem, err := s.Chronicle.MembershipForAccount(r.Context(), circleID, sess.AccountID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	names, err := s.Chronicle.ListIdentityNames(r.Context(), mem.IdentityID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]map[string]any, len(names))
	for i, n := range names {
		out[i] = map[string]any{
			"name":         n.Name,
			"effective_at": n.EffectiveAt.UTC().Format(time.RFC3339),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"names": out})
}

type transferOwnerBody struct {
	NewOwnerAccountID string `json:"new_owner_account_id"`
}

func (s *Server) handleTransferOwnership(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	var body transferOwnerBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	err := s.Chronicle.TransferOwnership(r.Context(), circleID, sess.AccountID, body.NewOwnerAccountID, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type excludeMemberBody struct {
	AccountID string `json:"account_id"`
}

func (s *Server) handleExcludeMember(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	var body excludeMemberBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	now := time.Now().UTC()
	err := s.Chronicle.Exclude(r.Context(), circleID, sess.AccountID, body.AccountID, now)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// Исключённый не оставляет живой ссылки в круг: по ней человек вошёл бы
	// уже без пригласившего (аудит 2026-09-22, SEC-9).
	if err := s.Auth.RevokeCircleInvitesBy(r.Context(), circleID, body.AccountID, now); err != nil {
		log.Printf("excludeMember: revoke invites %s/%s: %v", circleID, body.AccountID, err)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type deleteCircleBody struct {
	Name string `json:"name"`
}

func (s *Server) handleDeleteCircle(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	var body deleteCircleBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	blobIDs, err := s.Chronicle.DeleteCircle(r.Context(), circleID, sess.AccountID, body.Name, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// Файлы освобождаются после коммита; их судьба не меняет ответ (BLB-4).
	if s.Blobs != nil {
		if err := s.Blobs.ReleaseBlobs(r.Context(), blobIDs); err != nil {
			log.Printf("delete circle %s: release blobs: %v", circleID, err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
