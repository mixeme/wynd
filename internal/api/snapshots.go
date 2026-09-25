package api

import (
	"net/http"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func (s *Server) handleFeed(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	posts, err := s.Chronicle.FeedSnapshot(r.Context(), circleID, sess.AccountID)
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
		"posts":             feedPostResponses(posts),
		"events":            events,
		"circle_started_at": meta.CircleStartedAt.UTC().Format(time.RFC3339),
	}
	if meta.VisibleFrom != nil {
		out["visible_from"] = meta.VisibleFrom.UTC().Format(time.RFC3339)
	} else {
		out["visible_from"] = nil
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGrid(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	items, err := s.Chronicle.GridSnapshot(r.Context(), circleID, sess.AccountID)
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
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"circle_id": circleID, "items": out})
}

func (s *Server) handleMap(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	pins, err := s.Chronicle.MapSnapshot(r.Context(), circleID, sess.AccountID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]map[string]any, len(pins))
	for i, pin := range pins {
		out[i] = map[string]any{
			"post_id":    pin.PostID,
			"blob_id":    pin.BlobID,
			"entry_date": pin.EntryDate,
			"created_at": pin.CreatedAt.UTC().Format(time.RFC3339),
			"geo_lat":    pin.GeoLat,
			"geo_lng":    pin.GeoLng,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"circle_id": circleID, "pins": out})
}

func (s *Server) handleDays(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
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
			"entry_date":  d.Day.EntryDate,
			"post_count":  d.PostCount,
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
		out[i] = row
	}
	writeJSON(w, http.StatusOK, map[string]any{"circle_id": circleID, "days": out})
}

func (s *Server) handleDayDetail(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	entryDate := r.PathValue("date")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	posts, err := s.Chronicle.DayPostsSnapshot(r.Context(), circleID, sess.AccountID, entryDate)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"circle_id":  circleID,
		"entry_date": entryDate,
		"posts":      feedPostResponses(posts),
	})
}

func feedPostResponses(posts []chronicle.FeedPost) []map[string]any {
	out := make([]map[string]any, len(posts))
	for i, fp := range posts {
		out[i] = feedPostResponse(fp)
	}
	return out
}

func feedPostResponse(fp chronicle.FeedPost) map[string]any {
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
			comments[j] = commentResponse(c)
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
