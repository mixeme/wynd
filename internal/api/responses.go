package api

import (
	"net/http"
	"strconv"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// «Отклики» (3.13): страница откликов круга от свежего к старому.
func (s *Server) handleResponses(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	var before int64
	if v := r.URL.Query().Get("before"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			writeDomainError(w, chronicle.ErrInvalid)
			return
		}
		before = n
	}
	page, err := s.Chronicle.Responses(r.Context(), circleID, sess.AccountID, before)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	items := make([]map[string]any, len(page.Items))
	for i, it := range page.Items {
		item := map[string]any{
			"kind":       it.Kind,
			"seq":        it.Seq,
			"at":         it.At.UTC().Format(time.RFC3339),
			"actor_id":   it.ActorIdentityID,
			"actor_name": it.ActorName,
		}
		setIf(item, "actor_avatar_blob_id", it.ActorAvatarBlobID)
		setIf(item, "comment_id", it.CommentID)
		setIf(item, "body", it.Body)
		setIf(item, "emoji", it.Emoji)
		setIf(item, "title", it.Title)
		setIf(item, "post_id", it.PostID)
		setIf(item, "entry_date", it.EntryDate)
		setIf(item, "cover_blob_id", it.CoverBlobID)
		items[i] = item
	}
	posts := make(map[string]any, len(page.Posts))
	for id, p := range page.Posts {
		ref := map[string]any{
			"id":                 p.ID,
			"author_identity_id": p.AuthorIdentityID,
			"author_name":        p.AuthorName,
			"entry_date":         p.EntryDate,
			"created_at":         p.CreatedAt.UTC().Format(time.RFC3339),
			"excerpt":            p.Excerpt,
		}
		setIf(ref, "cover_blob_id", p.CoverBlobID)
		posts[id] = ref
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":     items,
		"posts":     posts,
		"day_posts": page.DayPosts,
		"read_seq":  page.ReadSeq,
		"has_more":  page.HasMore,
	})
}

func setIf(m map[string]any, key, value string) {
	if value != "" {
		m[key] = value
	}
}

func (s *Server) handleSetResponsesRead(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[readCursorBody](w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.SetResponseReadSeq(r.Context(), circleID, sess.AccountID, body.Seq, time.Now().UTC()); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
