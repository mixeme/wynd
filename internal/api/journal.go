package api

import (
	"context"
	"net/http"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

type createPostBody struct {
	Body       string      `json:"body"`
	EntryDate  string      `json:"entry_date"`
	CapturedAt *string     `json:"captured_at"`
	Media      []mediaBody `json:"media"`
}

type mediaBody struct {
	BlobID     string   `json:"blob_id"`
	Kind       string   `json:"kind"`
	CapturedAt *string  `json:"captured_at"`
	GeoLat     *float64 `json:"geo_lat"`
	GeoLng     *float64 `json:"geo_lng"`
	IsCover    bool     `json:"is_cover"`
}

type editPostBody struct {
	Body        string  `json:"body"`
	EntryDate   string  `json:"entry_date"`
	CoverBlobID *string `json:"cover_blob_id,omitempty"`
}

type textBody struct {
	Body string `json:"body"`
}

type reactionBody struct {
	Emoji string `json:"emoji"`
}

type dayTitleBody struct {
	Title string `json:"title"`
}

type dayCoverBody struct {
	PostID string `json:"post_id"`
	BlobID string `json:"blob_id"`
}

func (s *Server) handleCreatePost(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	var body createPostBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if body.Body == "" && len(body.Media) == 0 {
		writeError(w, chronicle.ErrInvalid)
		return
	}
	now := time.Now().UTC()
	var captured *time.Time
	if body.CapturedAt != nil {
		t, err := time.Parse(time.RFC3339, *body.CapturedAt)
		if err != nil {
			writeError(w, chronicle.ErrInvalid)
			return
		}
		captured = &t
	}
	media, err := parseMediaInput(body.Media)
	if err != nil {
		writeError(w, chronicle.ErrInvalid)
		return
	}
	if len(media) > 0 {
		blobIDs := make([]string, len(media))
		for i, m := range media {
			blobIDs[i] = m.BlobID
		}
		if err := s.Blobs.ValidateOwnedComplete(r.Context(), sess.AccountID, blobIDs); err != nil {
			writeDomainError(w, err)
			return
		}
		total, err := s.Blobs.TotalBytesForBlobs(r.Context(), blobIDs)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		if err := s.Blobs.CheckMediaQuota(r.Context(), circleID, total); err != nil {
			writeDomainError(w, err)
			return
		}
	}
	post, err := s.Chronicle.CreatePost(r.Context(), chronicle.PostInput{
		CircleID: circleID, AccountID: sess.AccountID, Body: body.Body,
		EntryDate: body.EntryDate, CapturedAt: captured, Now: now,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if len(media) > 0 {
		if err := s.Chronicle.AttachMedia(r.Context(), post.ID, media); err != nil {
			s.rollbackNewPost(r.Context(), circleID, sess.AccountID, post.ID)
			writeDomainError(w, err)
			return
		}
		for _, m := range media {
			if err := s.Blobs.AddRef(r.Context(), nil, m.BlobID, "post", post.ID); err != nil {
				s.rollbackNewPost(r.Context(), circleID, sess.AccountID, post.ID)
				writeDomainError(w, err)
				return
			}
		}
	}
	items, _ := s.Chronicle.ListPostMedia(r.Context(), post.ID)
	s.notifyCircle(circleID, sess.AccountID, "post")
	writeJSON(w, http.StatusCreated, postResponse(post, items))
}

func (s *Server) handleEditPost(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	postID := r.PathValue("post_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	var body editPostBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	now := time.Now().UTC()
	err := s.Chronicle.EditPost(r.Context(), circleID, sess.AccountID, postID, body.Body, body.EntryDate, now)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if body.CoverBlobID != nil && *body.CoverBlobID != "" {
		if err := s.Chronicle.SetPostCover(r.Context(), circleID, sess.AccountID, postID, *body.CoverBlobID, now); err != nil {
			writeDomainError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeletePost(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	postID := r.PathValue("post_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	blobIDs, err := s.Chronicle.PostMediaBlobIDs(r.Context(), postID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if err := s.Chronicle.DeletePost(r.Context(), circleID, sess.AccountID, postID, time.Now().UTC()); err != nil {
		writeDomainError(w, err)
		return
	}
	if err := s.Chronicle.DeletePostMedia(r.Context(), postID); err != nil {
		writeDomainError(w, err)
		return
	}
	if err := s.Blobs.RemoveRefsFor(r.Context(), "post", postID); err != nil {
		writeDomainError(w, err)
		return
	}
	if err := s.Blobs.ReleaseBlobs(r.Context(), blobIDs); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCreateComment(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	postID := r.PathValue("post_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	var body textBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	c, err := s.Chronicle.CreateComment(r.Context(), chronicle.CommentInput{
		CircleID: circleID, AccountID: sess.AccountID, PostID: postID,
		Body: body.Body, Now: time.Now().UTC(),
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	s.notifyCircle(circleID, sess.AccountID, "comment")
	writeJSON(w, http.StatusCreated, commentResponse(c))
}

func (s *Server) handleEditComment(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	commentID := r.PathValue("comment_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	var body textBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	err := s.Chronicle.EditComment(r.Context(), circleID, sess.AccountID, commentID, body.Body, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteComment(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	commentID := r.PathValue("comment_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	err := s.Chronicle.DeleteComment(r.Context(), circleID, sess.AccountID, commentID, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSetReaction(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	postID := r.PathValue("post_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	var body reactionBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	rx, err := s.Chronicle.SetReaction(r.Context(), chronicle.ReactionInput{
		CircleID: circleID, AccountID: sess.AccountID, PostID: postID,
		Emoji: body.Emoji, Now: time.Now().UTC(),
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	s.notifyCircle(circleID, sess.AccountID, "reaction")
	writeJSON(w, http.StatusOK, reactionResponse(rx))
}

func (s *Server) handleDeleteReaction(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	postID := r.PathValue("post_id")
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
	reactionID, err := s.Chronicle.ReactionIDForPost(r.Context(), circleID, postID, mem.IdentityID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	err = s.Chronicle.DeleteReaction(r.Context(), circleID, sess.AccountID, reactionID, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSetDayTitle(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	entryDate := r.PathValue("date")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	var body dayTitleBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	err := s.Chronicle.SetDayTitle(r.Context(), chronicle.DayTitleInput{
		CircleID: circleID, AccountID: sess.AccountID, EntryDate: entryDate,
		Title: body.Title, Now: time.Now().UTC(),
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleClearDayTitle(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	entryDate := r.PathValue("date")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	err := s.Chronicle.ClearDayTitle(r.Context(), circleID, sess.AccountID, entryDate, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSetDayCover(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	entryDate := r.PathValue("date")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	var body dayCoverBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	err := s.Chronicle.SetDayCover(r.Context(), chronicle.DayCoverInput{
		CircleID: circleID, AccountID: sess.AccountID, EntryDate: entryDate,
		PostID: body.PostID, BlobID: body.BlobID, Now: time.Now().UTC(),
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleClearDayCover(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	entryDate := r.PathValue("date")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	err := s.Chronicle.ClearDayCover(r.Context(), circleID, sess.AccountID, entryDate, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func parseMediaInput(items []mediaBody) ([]chronicle.MediaInput, error) {
	out := make([]chronicle.MediaInput, len(items))
	for i, m := range items {
		kind := chronicle.MediaKind(m.Kind)
		if kind != chronicle.MediaPhoto && kind != chronicle.MediaVideo && kind != chronicle.MediaAttachment {
			return nil, chronicle.ErrInvalid
		}
		var captured *time.Time
		if m.CapturedAt != nil {
			t, err := time.Parse(time.RFC3339, *m.CapturedAt)
			if err != nil {
				return nil, err
			}
			captured = &t
		}
		out[i] = chronicle.MediaInput{
			BlobID: m.BlobID, Kind: kind, CapturedAt: captured,
			GeoLat: m.GeoLat, GeoLng: m.GeoLng, IsCover: m.IsCover,
		}
	}
	if err := chronicle.ValidateMediaKinds(out); err != nil {
		return nil, chronicle.ErrInvalid
	}
	return out, nil
}

type postJSON struct {
	ID         string                   `json:"id"`
	CircleID   string                   `json:"circle_id"`
	Body       string                   `json:"body"`
	EntryDate  string                   `json:"entry_date"`
	CapturedAt *string                  `json:"captured_at,omitempty"`
	CreatedAt  string                   `json:"created_at"`
	Media      []chronicle.MediaSummary `json:"media,omitempty"`
}

func postResponse(p chronicle.Post, media []chronicle.PostMedia) postJSON {
	out := postJSON{
		ID: p.ID, CircleID: p.CircleID, Body: p.Body, EntryDate: p.EntryDate,
		CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339),
	}
	if p.CapturedAt != nil {
		s := p.CapturedAt.UTC().Format(time.RFC3339)
		out.CapturedAt = &s
	}
	if len(media) > 0 {
		out.Media = chronicle.SummarizeMedia(media)
	}
	return out
}

func (s *Server) rollbackNewPost(ctx context.Context, circleID, accountID, postID string) {
	blobIDs, _ := s.Chronicle.PostMediaBlobIDs(ctx, postID)
	_ = s.Chronicle.DeletePost(ctx, circleID, accountID, postID, time.Now().UTC())
	_ = s.Chronicle.DeletePostMedia(ctx, postID)
	if s.Blobs != nil {
		_ = s.Blobs.RemoveRefsFor(ctx, "post", postID)
		_ = s.Blobs.ReleaseBlobs(ctx, blobIDs)
	}
}

func commentResponse(c chronicle.Comment) map[string]any {
	row := map[string]any{
		"id": c.ID, "post_id": c.PostID, "body": c.Body,
		"author_name": c.AuthorName, "identity_id": c.IdentityID,
		"created_at": c.CreatedAt.UTC().Format(time.RFC3339),
	}
	appendEditPolicy(row, c.EditWindow, c.EditableUntil)
	return row
}

func reactionResponse(rx chronicle.Reaction) map[string]any {
	row := map[string]any{
		"id": rx.ID, "post_id": rx.PostID, "emoji": rx.Emoji,
		"author_name": rx.AuthorName, "identity_id": rx.IdentityID,
		"created_at": rx.CreatedAt.UTC().Format(time.RFC3339),
	}
	appendEditPolicy(row, rx.EditWindow, rx.EditableUntil)
	return row
}
