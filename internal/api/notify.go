package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/push"
)

type notifyPrefsBody struct {
	Posts     *bool `json:"posts"`
	Comments  *bool `json:"comments"`
	Reactions *bool `json:"reactions"`
}

type NotifyPrefs struct {
	Posts     bool `json:"posts"`
	Comments  bool `json:"comments"`
	Reactions bool `json:"reactions"`
	Mentions  bool `json:"mentions"`
}

func defaultNotifyPrefs() NotifyPrefs {
	return NotifyPrefs{Posts: true, Comments: true, Reactions: true, Mentions: true}
}

func (s *Server) handleGetAccountNotifyPrefs(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, auth.ErrForbidden)
		return
	}
	prefs, err := s.loadAccountNotifyPrefs(r.Context(), sess.AccountID)
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
	prefs := defaultNotifyPrefs()
	existing, _ := s.loadAccountNotifyPrefs(r.Context(), sess.AccountID)
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
	if err := s.saveAccountNotifyPrefs(r.Context(), sess.AccountID, prefs); err != nil {
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
	prefs, err := s.loadCircleNotifyPrefs(r.Context(), sess.AccountID, circleID)
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
	base, err := s.loadAccountNotifyPrefs(r.Context(), sess.AccountID)
	if err != nil {
		writeError(w, err)
		return
	}
	prefs := base
	if ov, err := s.loadCircleNotifyPrefs(r.Context(), sess.AccountID, circleID); err == nil {
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
	if err := s.saveCircleNotifyPrefs(r.Context(), sess.AccountID, circleID, prefs, base); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, prefs)
}

func (s *Server) loadAccountNotifyPrefs(ctx context.Context, accountID string) (NotifyPrefs, error) {
	prefs := defaultNotifyPrefs()
	err := s.Auth.DB().QueryRowContext(ctx, `
		SELECT posts, comments, reactions FROM account_notify_prefs WHERE account_id = ?
	`, accountID).Scan(&prefs.Posts, &prefs.Comments, &prefs.Reactions)
	if errors.Is(err, sql.ErrNoRows) {
		return prefs, nil
	}
	if err != nil {
		return NotifyPrefs{}, err
	}
	prefs.Mentions = true
	return prefs, nil
}

func (s *Server) saveAccountNotifyPrefs(ctx context.Context, accountID string, prefs NotifyPrefs) error {
	_, err := s.Auth.DB().ExecContext(ctx, `
		INSERT INTO account_notify_prefs (account_id, posts, comments, reactions)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(account_id) DO UPDATE SET
			posts = excluded.posts,
			comments = excluded.comments,
			reactions = excluded.reactions
	`, accountID, boolInt(prefs.Posts), boolInt(prefs.Comments), boolInt(prefs.Reactions))
	return err
}

func (s *Server) loadCircleNotifyPrefs(ctx context.Context, accountID, circleID string) (NotifyPrefs, error) {
	base, err := s.loadAccountNotifyPrefs(ctx, accountID)
	if err != nil {
		return NotifyPrefs{}, err
	}
	var posts, comments, reactions sql.NullInt64
	err = s.Auth.DB().QueryRowContext(ctx, `
		SELECT posts, comments, reactions FROM circle_notify_prefs
		WHERE account_id = ? AND circle_id = ?
	`, accountID, circleID).Scan(&posts, &comments, &reactions)
	if errors.Is(err, sql.ErrNoRows) {
		return base, nil
	}
	if err != nil {
		return NotifyPrefs{}, err
	}
	if posts.Valid {
		base.Posts = posts.Int64 == 1
	}
	if comments.Valid {
		base.Comments = comments.Int64 == 1
	}
	if reactions.Valid {
		base.Reactions = reactions.Int64 == 1
	}
	base.Mentions = true
	return base, nil
}

func (s *Server) saveCircleNotifyPrefs(ctx context.Context, accountID, circleID string, prefs, base NotifyPrefs) error {
	if prefs.Posts == base.Posts && prefs.Comments == base.Comments && prefs.Reactions == base.Reactions {
		_, err := s.Auth.DB().ExecContext(ctx, `
			DELETE FROM circle_notify_prefs WHERE account_id = ? AND circle_id = ?
		`, accountID, circleID)
		return err
	}
	_, err := s.Auth.DB().ExecContext(ctx, `
		INSERT INTO circle_notify_prefs (account_id, circle_id, posts, comments, reactions)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(account_id, circle_id) DO UPDATE SET
			posts = excluded.posts,
			comments = excluded.comments,
			reactions = excluded.reactions
	`, accountID, circleID, nullBool(prefs.Posts), nullBool(prefs.Comments), nullBool(prefs.Reactions))
	return err
}

func (s *Server) notifyCircle(circleID, actorAccountID, signalType string) {
	if s == nil || s.Push == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		ids, err := s.Chronicle.CircleMemberAccountIDs(ctx, circleID)
		if err != nil {
			return
		}
		for _, accountID := range ids {
			if accountID == actorAccountID {
				continue
			}
			prefs, err := s.loadCircleNotifyPrefs(ctx, accountID, circleID)
			if err != nil {
				continue
			}
			if !prefAllows(prefs, signalType) {
				continue
			}
			_ = s.Push.SendSignal(ctx, accountID, push.Signal{
				CircleID: circleID,
				Type:     signalType,
				Count:    1,
			})
		}
	}()
}

func prefAllows(prefs NotifyPrefs, signalType string) bool {
	switch signalType {
	case "post":
		return prefs.Posts
	case "comment":
		return prefs.Comments
	case "reaction":
		return prefs.Reactions
	case "mention":
		return true
	default:
		return true
	}
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullBool(v bool) any {
	if v {
		return 1
	}
	return 0
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
	if body.RequestedBytes < 1 {
		writeError(w, auth.ErrInvalid)
		return
	}
	id, err := uuid.NewV7()
	if err != nil {
		writeError(w, err)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.Auth.DB().ExecContext(r.Context(), `
		INSERT INTO quota_requests (id, circle_id, requested_by_account_id, requested_bytes, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, id.String(), circleID, sess.AccountID, body.RequestedBytes, now)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id.String()})
}

func (s *Server) handleAdminQuotaRequests(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Auth.DB().QueryContext(r.Context(), `
		SELECT qr.id, qr.circle_id, qr.requested_by_account_id, a.email,
			qr.requested_bytes, qr.status, qr.admin_note, qr.created_at, qr.resolved_at
		FROM quota_requests qr
		JOIN accounts a ON a.id = qr.requested_by_account_id
		WHERE qr.status = 'pending'
		ORDER BY qr.created_at ASC
	`)
	if err != nil {
		writeError(w, err)
		return
	}
	defer rows.Close()

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
	var out []row
	for rows.Next() {
		var item row
		var note, resolved sql.NullString
		if err := rows.Scan(&item.ID, &item.CircleID, &item.RequesterID, &item.RequesterMail,
			&item.RequestedBytes, &item.Status, &note, &item.CreatedAt, &resolved); err != nil {
			writeError(w, err)
			return
		}
		if note.Valid {
			item.AdminNote = &note.String
		}
		if resolved.Valid {
			item.ResolvedAt = &resolved.String
		}
		out = append(out, item)
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

	var circleID string
	var requested int64
	err := s.Auth.DB().QueryRowContext(r.Context(), `
		SELECT circle_id, requested_bytes FROM quota_requests WHERE id = ? AND status = 'pending'
	`, id).Scan(&circleID, &requested)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, auth.ErrNotFound)
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}

	status := "rejected"
	if approve {
		status = "approved"
		if _, err := s.Auth.DB().ExecContext(r.Context(), `
			UPDATE circles SET quota_bytes = COALESCE(quota_bytes, 0) + ? WHERE id = ?
		`, requested, circleID); err != nil {
			writeError(w, err)
			return
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.Auth.DB().ExecContext(r.Context(), `
		UPDATE quota_requests SET status = ?, admin_note = NULLIF(?, ''), resolved_at = ? WHERE id = ?
	`, status, body.AdminNote, now, id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}
