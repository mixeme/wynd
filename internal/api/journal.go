package api

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

type createPostBody struct {
	Body       string      `json:"body"`
	EntryDate  string      `json:"entry_date"`
	CapturedAt *string     `json:"captured_at"`
	Media      []mediaBody `json:"media"`
	// ClientID — ключ идемпотентности офлайн-очереди; поле необязательное.
	ClientID string `json:"client_id"`
}

type mediaBody struct {
	BlobID            string   `json:"blob_id"`
	Kind              string   `json:"kind"`
	CapturedAt        *string  `json:"captured_at"`
	GeoLat            *float64 `json:"geo_lat"`
	GeoLng            *float64 `json:"geo_lng"`
	IsCover           bool     `json:"is_cover"`
	AudioArtist       string   `json:"audio_artist"`
	AudioTitle        string   `json:"audio_title"`
	AudioCoverBlobID  string   `json:"audio_cover_blob_id"`
	VideoPosterBlobID string   `json:"video_poster_blob_id"`
	// Voice, AudioDurationMs, AudioPeaks — голосовое (C14).
	Voice           bool  `json:"voice"`
	AudioDurationMs int64 `json:"audio_duration_ms"`
	AudioPeaks      []int `json:"audio_peaks"`
	// Crop — кадр обложки для ленты (4.16).
	Crop *chronicle.CoverCrop `json:"crop"`
}

type editPostBody struct {
	Body        string       `json:"body"`
	EntryDate   string       `json:"entry_date"`
	CoverBlobID *string      `json:"cover_blob_id,omitempty"`
	Media       *[]mediaBody `json:"media,omitempty"`
}

type textBody struct {
	Body string `json:"body"`
	// ClientID — ключ идемпотентности офлайн-очереди; поле необязательное.
	ClientID string `json:"client_id"`
}

// commentBody — новый комментарий: слова и вложения (4.28). Из полей media
// читаются blob_id, kind и данные голосового; остальное — про запись.
type commentBody struct {
	Body     string      `json:"body"`
	ClientID string      `json:"client_id"`
	Media    []mediaBody `json:"media"`
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
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[createPostBody](w, r)
	if !ok {
		return
	}
	if strings.TrimSpace(body.Body) == "" && len(body.Media) == 0 {
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
		blobIDs := chronicle.MediaBlobIDs(media)
		if err := s.Blobs.ValidateOwnedComplete(r.Context(), sess.AccountID, blobIDs); err != nil {
			writeDomainError(w, err)
			return
		}
		if err := s.validateAudioCovers(r.Context(), media); err != nil {
			writeDomainError(w, err)
			return
		}
		if err := s.validateVideoPosters(r.Context(), media); err != nil {
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
	post, err := s.createPostWithMedia(r.Context(), chronicle.PostInput{
		CircleID: circleID, AccountID: sess.AccountID, Body: body.Body,
		EntryDate: body.EntryDate, CapturedAt: captured, Now: now,
		AllowEmptyBody: len(media) > 0, ClientID: strings.TrimSpace(body.ClientID),
	}, media)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// До ответа: клиент сразу перечитывает ленту и должен увидеть кадр.
	s.Posters.FillPost(r.Context(), post.ID)
	items, _ := s.Chronicle.ListPostMedia(r.Context(), post.ID)
	if !post.Replayed {
		s.notifyCircle(circleID, sess.AccountID, "post")
		if ids, err := s.Chronicle.MentionedAccountIDs(r.Context(), circleID, body.Body); err == nil {
			s.notifyAccounts(circleID, sess.AccountID, "mention", ids)
		}
	}
	writeJSON(w, http.StatusCreated, postResponse(post, items))
}

func (s *Server) handleEditPost(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	postID := r.PathValue("post_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[editPostBody](w, r)
	if !ok {
		return
	}
	now := time.Now().UTC()
	// media[] задаёт обложку сам (is_cover), поэтому пара с cover_blob_id —
	// противоречивый запрос, а не два шага подряд (QLT-3).
	if body.Media != nil && body.CoverBlobID != nil {
		writeError(w, chronicle.ErrInvalid)
		return
	}
	if body.Media != nil {
		media, err := parseMediaInput(*body.Media)
		if err != nil {
			writeError(w, chronicle.ErrInvalid)
			return
		}
		if strings.TrimSpace(body.Body) == "" && len(media) == 0 {
			writeError(w, chronicle.ErrInvalid)
			return
		}
		if err := s.editPostReplaceMedia(r.Context(), circleID, sess.AccountID, postID, body.Body, body.EntryDate, media, now); err != nil {
			writeDomainError(w, err)
			return
		}
		s.Posters.FillPost(r.Context(), postID)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
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
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	// Удаление — одна доменная транзакция: запись, ветка, вложения и ссылки
	// на блобы либо уходят вместе, либо не уходят вовсе (QLT-3).
	blobIDs, err := s.Chronicle.DeletePost(r.Context(), circleID, sess.AccountID, postID, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// Файлы освобождаются после коммита: их судьба не должна менять ответ.
	if err := s.Blobs.ReleaseBlobs(r.Context(), blobIDs); err != nil {
		log.Printf("delete post %s: release blobs: %v", postID, err)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCreateComment(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	postID := r.PathValue("post_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[commentBody](w, r)
	if !ok {
		return
	}
	media, err := parseCommentMedia(body.Media)
	if err != nil {
		writeError(w, chronicle.ErrInvalid)
		return
	}
	if len(media) > 0 {
		blobIDs := chronicle.MediaBlobIDs(media)
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
	c, err := s.Chronicle.CreateComment(r.Context(), chronicle.CommentInput{
		CircleID: circleID, AccountID: sess.AccountID, PostID: postID,
		Body: body.Body, Now: time.Now().UTC(), ClientID: strings.TrimSpace(body.ClientID),
		Media: media,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// Повтор очереди с тем же ключом — комментарий уже был, второй раз не звоним.
	if !c.Replayed {
		s.notifyComment(circleID, sess.AccountID, postID)
		if ids, err := s.Chronicle.MentionedAccountIDs(r.Context(), circleID, body.Body); err == nil {
			s.notifyAccounts(circleID, sess.AccountID, "mention", ids)
		}
	}
	avatars, _ := s.Chronicle.IdentityAvatarBlobIDs(r.Context(), []string{c.IdentityID})
	writeJSON(w, http.StatusCreated, commentResponse(c, avatars))
}

func (s *Server) handleEditComment(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	commentID := r.PathValue("comment_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[textBody](w, r)
	if !ok {
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
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	blobIDs, err := s.Chronicle.DeleteComment(r.Context(), circleID, sess.AccountID, commentID, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// Файлы вложений освобождаются после коммита, как у записи.
	if err := s.Blobs.ReleaseBlobs(r.Context(), blobIDs); err != nil {
		log.Printf("delete comment %s: release blobs: %v", commentID, err)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSetReaction(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	postID := r.PathValue("post_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[reactionBody](w, r)
	if !ok {
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
	s.notifyCirclePost(circleID, sess.AccountID, "reaction", postID)
	writeJSON(w, http.StatusOK, reactionResponse(rx))
}

func (s *Server) handleDeleteReaction(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	postID := r.PathValue("post_id")
	sess, ok := requireSession(w, r)
	if !ok {
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
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[dayTitleBody](w, r)
	if !ok {
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
	sess, ok := requireSession(w, r)
	if !ok {
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
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[dayCoverBody](w, r)
	if !ok {
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
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	err := s.Chronicle.ClearDayCover(r.Context(), circleID, sess.AccountID, entryDate, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) validateAudioCovers(ctx context.Context, items []chronicle.MediaInput) error {
	for _, item := range items {
		if item.AudioCoverBlobID == "" {
			continue
		}
		b, err := s.Blobs.LoadBlob(ctx, item.AudioCoverBlobID)
		if err != nil {
			return err
		}
		mime := strings.ToLower(strings.TrimSpace(b.MimeType))
		if i := strings.Index(mime, ";"); i >= 0 {
			mime = strings.TrimSpace(mime[:i])
		}
		if mime != "image/jpeg" && mime != "image/jpg" {
			return chronicle.ErrInvalid
		}
	}
	return nil
}

func (s *Server) validateVideoPosters(ctx context.Context, items []chronicle.MediaInput) error {
	for _, item := range items {
		if item.VideoPosterBlobID == "" {
			continue
		}
		b, err := s.Blobs.LoadBlob(ctx, item.VideoPosterBlobID)
		if err != nil {
			return err
		}
		mime := strings.ToLower(strings.TrimSpace(b.MimeType))
		if i := strings.Index(mime, ";"); i >= 0 {
			mime = strings.TrimSpace(mime[:i])
		}
		if mime != "image/jpeg" && mime != "image/jpg" {
			return chronicle.ErrInvalid
		}
	}
	return nil
}

func parseMediaInput(items []mediaBody) ([]chronicle.MediaInput, error) {
	if len(items) > chronicle.MaxPostMedia {
		return nil, chronicle.ErrInvalid
	}
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
			AudioArtist: m.AudioArtist, AudioTitle: m.AudioTitle,
			AudioCoverBlobID: m.AudioCoverBlobID, VideoPosterBlobID: m.VideoPosterBlobID,
			Voice:           m.Voice,
			AudioDurationMs: m.AudioDurationMs,
			AudioPeaks:      m.AudioPeaks,
			Crop:            m.Crop,
		}
	}
	if err := chronicle.ValidateMediaKinds(out); err != nil {
		return nil, chronicle.ErrInvalid
	}
	return out, nil
}

// parseCommentMedia — вложения комментария: фото или файл (в том числе
// голосовое), не больше MaxCommentMedia. Видео — только в запись.
func parseCommentMedia(items []mediaBody) ([]chronicle.MediaInput, error) {
	if len(items) > chronicle.MaxCommentMedia {
		return nil, chronicle.ErrInvalid
	}
	out := make([]chronicle.MediaInput, len(items))
	for i, m := range items {
		kind := chronicle.MediaKind(m.Kind)
		if kind != chronicle.MediaPhoto && kind != chronicle.MediaAttachment {
			return nil, chronicle.ErrInvalid
		}
		out[i] = chronicle.MediaInput{
			BlobID: strings.TrimSpace(m.BlobID), Kind: kind,
			Voice: m.Voice, AudioDurationMs: m.AudioDurationMs, AudioPeaks: m.AudioPeaks,
		}
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

func (s *Server) createPostWithMedia(ctx context.Context, in chronicle.PostInput, media []chronicle.MediaInput) (chronicle.Post, error) {
	if len(media) == 0 {
		return s.Chronicle.CreatePost(ctx, in)
	}
	tx, err := s.Blobs.DB().BeginTx(ctx, nil)
	if err != nil {
		return chronicle.Post{}, err
	}
	defer func() { _ = tx.Rollback() }()

	post, err := s.Chronicle.CreatePostInTx(ctx, tx, in)
	if err != nil {
		return chronicle.Post{}, err
	}
	// Повтор по client_id: медиа уже привязаны первым запросом, второй
	// раз не дописываем (аудит 2026-09-22).
	if post.Replayed {
		return post, nil
	}
	if err := s.Chronicle.AttachMediaInTx(ctx, tx, post.ID, media); err != nil {
		return chronicle.Post{}, err
	}
	if err := tx.Commit(); err != nil {
		return chronicle.Post{}, err
	}
	return post, nil
}

func (s *Server) editPostReplaceMedia(ctx context.Context, circleID, accountID, postID, body, entryDate string, media []chronicle.MediaInput, now time.Time) error {
	oldIDs, err := s.Chronicle.PostMediaBlobIDs(ctx, postID)
	if err != nil {
		return err
	}
	oldSet := make(map[string]struct{}, len(oldIDs))
	for _, id := range oldIDs {
		oldSet[id] = struct{}{}
	}
	var addedIDs []string
	for _, id := range chronicle.MediaBlobIDs(media) {
		if _, ok := oldSet[id]; !ok {
			addedIDs = append(addedIDs, id)
		}
	}
	if len(addedIDs) > 0 {
		if err := s.Blobs.ValidateOwnedComplete(ctx, accountID, addedIDs); err != nil {
			return err
		}
		total, err := s.Blobs.TotalBytesForBlobs(ctx, addedIDs)
		if err != nil {
			return err
		}
		if err := s.Blobs.CheckMediaQuota(ctx, circleID, total); err != nil {
			return err
		}
	}
	// Тип обложки смотрим после проверки владения: чужой блоб не должен
	// отличаться ответом от несуществующего.
	if err := s.validateAudioCovers(ctx, media); err != nil {
		return err
	}
	if err := s.validateVideoPosters(ctx, media); err != nil {
		return err
	}

	tx, err := s.Blobs.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	allowEmpty := len(media) > 0
	if err := s.Chronicle.EditPostInTx(ctx, tx, circleID, accountID, postID, body, entryDate, now, allowEmpty); err != nil {
		return err
	}
	post, err := s.Chronicle.LoadPostInTx(ctx, tx, postID)
	if err != nil {
		return err
	}
	removed, err := s.Chronicle.ReplacePostMediaInTx(ctx, tx, circleID, postID, post.EntryDate, media)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.Blobs.ReleaseBlobs(ctx, removed)
}

func commentResponse(c chronicle.Comment, avatars map[string]string) map[string]any {
	row := map[string]any{
		"id": c.ID, "post_id": c.PostID, "body": c.Body,
		"author_name": c.AuthorName, "identity_id": c.IdentityID,
		"created_at": c.CreatedAt.UTC().Format(time.RFC3339),
	}
	if blobID := avatars[c.IdentityID]; blobID != "" {
		row["author_avatar_blob_id"] = blobID
	}
	appendEditPolicy(row, c.EditWindow, c.EditableUntil)
	if len(c.Media) > 0 {
		row["media"] = chronicle.SummarizeMedia(c.Media)
	}
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
