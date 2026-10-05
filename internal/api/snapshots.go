package api

import (
	"net/http"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func (s *Server) handleFeed(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	before, err := chronicle.ParsePageCursor(r.URL.Query().Get("before"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	posts, next, err := s.Chronicle.FeedPage(r.Context(), circleID, sess.AccountID, before)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	avatars, err := s.Chronicle.IdentityAvatarBlobIDs(r.Context(), chronicle.IdentityIDsFromFeed(posts))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	meta, err := s.Chronicle.FeedMetaForAccount(r.Context(), circleID, sess.AccountID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	events := make([]map[string]any, len(meta.Events))
	for i, ev := range meta.Events {
		events[i] = map[string]any{
			"seq":        ev.Seq,
			"summary":    ev.Summary,
			"created_at": ev.CreatedAt.UTC().Format(time.RFC3339),
		}
	}
	out := map[string]any{
		"circle_id":         circleID,
		"posts":             feedPostResponses(posts, avatars),
		"events":            events,
		"circle_started_at": meta.CircleStartedAt.UTC().Format(time.RFC3339),
	}
	if meta.VisibleFrom != nil {
		out["visible_from"] = meta.VisibleFrom.UTC().Format(time.RFC3339)
	} else {
		out["visible_from"] = nil
	}
	if before != nil {
		// Старшие порции — только записи: строки журнала пришли с первой.
		out["events"] = []map[string]any{}
	}
	setNextBefore(out, next)
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGrid(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	before, err := chronicle.ParsePageCursor(r.URL.Query().Get("before"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	items, next, err := s.Chronicle.GridPage(r.Context(), circleID, sess.AccountID, before)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]map[string]any, len(items))
	for i, item := range items {
		out[i] = map[string]any{
			"post_id":    item.PostID,
			"blob_id":    item.BlobID,
			"entry_date": item.EntryDate,
			"created_at": item.CreatedAt.UTC().Format(time.RFC3339),
			"is_cover":   item.IsCover,
			"kind":       item.Kind,
		}
	}
	resp := map[string]any{"circle_id": circleID, "items": out}
	setNextBefore(resp, next)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleMap(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	before, err := chronicle.ParsePageCursor(r.URL.Query().Get("before"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	pins, next, err := s.Chronicle.MapPage(r.Context(), circleID, sess.AccountID, before)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]map[string]any, len(pins))
	for i, pin := range pins {
		out[i] = map[string]any{
			"post_id":     pin.PostID,
			"blob_id":     pin.BlobID,
			"entry_date":  pin.EntryDate,
			"created_at":  pin.CreatedAt.UTC().Format(time.RFC3339),
			"geo_lat":     pin.GeoLat,
			"geo_lng":     pin.GeoLng,
			"author_name": pin.AuthorName,
			"body":        pin.Body,
		}
	}
	resp := map[string]any{"circle_id": circleID, "pins": out}
	setNextBefore(resp, next)
	writeJSON(w, http.StatusOK, resp)
}

// setNextBefore — курсор следующей порции (C18); нет — дальше ничего.
func setNextBefore(out map[string]any, next *chronicle.PageCursor) {
	if next != nil {
		out["next_before"] = next.String()
	}
}

func (s *Server) handleDays(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	days, err := s.Chronicle.DaysSnapshot(r.Context(), circleID, sess.AccountID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]map[string]any, len(days))
	for i, d := range days {
		row := map[string]any{
			"entry_date": d.Day.EntryDate,
			"post_count": d.PostCount,
		}
		if d.Day.Title != "" {
			row["title"] = d.Day.Title
		}
		if d.Day.CoverPostID != "" {
			row["cover_post_id"] = d.Day.CoverPostID
		}
		if d.Day.CoverBlobID != "" {
			row["cover_blob_id"] = d.Day.CoverBlobID
		}
		row["title_editable_until"] = editableUntilJSON(d.TitleEditableUntil)
		row["cover_editable_until"] = editableUntilJSON(d.CoverEditableUntil)
		if d.FallbackCoverBlobID != "" {
			row["fallback_cover_blob_id"] = d.FallbackCoverBlobID
		}
		if d.CoverImageBlobID != "" {
			row["cover_image_blob_id"] = d.CoverImageBlobID
		}
		if d.PhotoCount > 0 {
			row["photo_count"] = d.PhotoCount
		}
		out[i] = row
	}
	writeJSON(w, http.StatusOK, map[string]any{"circle_id": circleID, "days": out})
}

func (s *Server) handleDayDetail(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	entryDate := r.PathValue("date")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	posts, err := s.Chronicle.DayPostsSnapshot(r.Context(), circleID, sess.AccountID, entryDate)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	avatars, err := s.Chronicle.IdentityAvatarBlobIDs(r.Context(), chronicle.IdentityIDsFromFeed(posts))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"circle_id":  circleID,
		"entry_date": entryDate,
		"posts":      feedPostResponses(posts, avatars),
	})
}

func feedPostResponses(posts []chronicle.FeedPost, avatars map[string]string) []map[string]any {
	out := make([]map[string]any, len(posts))
	for i, fp := range posts {
		out[i] = feedPostResponse(fp, avatars)
	}
	return out
}

func feedPostResponse(fp chronicle.FeedPost, avatars map[string]string) map[string]any {
	p := fp.Post
	row := map[string]any{
		"id":          p.ID,
		"body":        p.Body,
		"entry_date":  p.EntryDate,
		"author_name": p.AuthorName,
		"identity_id": p.IdentityID,
		"created_at":  p.CreatedAt.UTC().Format(time.RFC3339),
		"event_seq":   p.EventSeq,
	}
	if blobID := avatars[p.IdentityID]; blobID != "" {
		row["author_avatar_blob_id"] = blobID
	}
	if p.CapturedAt != nil {
		row["captured_at"] = p.CapturedAt.UTC().Format(time.RFC3339)
	}
	appendEditPolicy(row, p.EditWindow, p.EditableUntil)
	if len(fp.Media) > 0 {
		row["media"] = chronicle.SummarizeMedia(fp.Media)
	}
	if len(fp.Comments) > 0 {
		comments := make([]map[string]any, len(fp.Comments))
		for j, c := range fp.Comments {
			comments[j] = commentResponse(c, avatars)
		}
		row["comments"] = comments
	}
	if len(fp.Reactions) > 0 {
		reactions := make([]map[string]any, len(fp.Reactions))
		for j, rx := range fp.Reactions {
			reactions[j] = reactionResponse(rx)
		}
		row["reactions"] = reactions
	}
	return row
}

func appendEditPolicy(row map[string]any, w chronicle.EditWindow, until *time.Time) {
	if w.Seconds != nil {
		row["edit_window_sec"] = *w.Seconds
	} else {
		row["edit_window_sec"] = nil
	}
	if until != nil {
		row["editable_until"] = until.UTC().Format(time.RFC3339)
	}
}

func editableUntilJSON(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}
