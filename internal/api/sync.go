package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
	if cursor < 0 {
		cursor = 0
	}

	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/event-stream") {
		s.syncSSE(w, r, sess.AccountID, cursor)
		return
	}
	if strings.Contains(accept, "application/x-ndjson") {
		s.syncNDJSON(w, r, sess.AccountID, cursor)
		return
	}

	events, err := s.Chronicle.SyncEvents(r.Context(), sess.AccountID, cursor, chronicle.SyncBatchSize())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	maxSeq, err := s.Chronicle.MaxEventSeq(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events":  eventResponses(events),
		"max_seq": maxSeq,
	})
}

func (s *Server) syncSSE(w http.ResponseWriter, r *http.Request, accountID string, cursor int64) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, chronicle.ErrInvalid)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	cur := cursor
	for {
		events, err := s.Chronicle.SyncEvents(r.Context(), accountID, cur, chronicle.SyncBatchSize())
		if err != nil {
			return
		}
		for _, ev := range events {
			data, _ := json.Marshal(eventResponse(ev))
			fmt.Fprintf(w, "event: sync\ndata: %s\n\n", data)
			flusher.Flush()
			cur = ev.Seq
		}
		if len(events) > 0 {
			continue
		}
		select {
		case <-r.Context().Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func (s *Server) syncNDJSON(w http.ResponseWriter, r *http.Request, accountID string, cursor int64) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, _ := w.(http.Flusher)

	cur := cursor
	for {
		events, err := s.Chronicle.SyncEvents(r.Context(), accountID, cur, chronicle.SyncBatchSize())
		if err != nil {
			return
		}
		if len(events) == 0 {
			break
		}
		for _, ev := range events {
			data, _ := json.Marshal(eventResponse(ev))
			_, _ = w.Write(data)
			_, _ = w.Write([]byte("\n"))
			if flusher != nil {
				flusher.Flush()
			}
			cur = ev.Seq
		}
	}
}

func eventResponse(ev chronicle.Event) map[string]any {
	out := map[string]any{
		"seq":         ev.Seq,
		"id":          ev.ID,
		"circle_id":   ev.CircleID,
		"type":        ev.Type,
		"is_service":  ev.IsService,
		"actor_name":  ev.ActorName,
		"summary":     ev.Summary,
		"created_at":  ev.CreatedAt.UTC().Format(time.RFC3339),
		"payload":     json.RawMessage(ev.Payload),
	}
	if ev.ActorIdentityID != "" {
		out["actor_identity_id"] = ev.ActorIdentityID
	}
	if ev.TargetID != "" {
		out["target_id"] = ev.TargetID
	}
	return out
}

func eventResponses(events []chronicle.Event) []map[string]any {
	out := make([]map[string]any, len(events))
	for i, ev := range events {
		out[i] = eventResponse(ev)
	}
	return out
}
