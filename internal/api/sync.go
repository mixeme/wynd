package api

import (
	"encoding/json"
	"fmt"
	"log"
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

const (
	// Тихий поток рвут прокси с таймаутом чтения, поэтому раз в 15 с уходит
	// комментарный кадр SSE. Дедлайн записи не даёт зависнуть на клиенте,
	// который перестал читать (API-3).
	sseHeartbeat     = 15 * time.Second
	sseWriteDeadline = 30 * time.Second
	ssePollInterval  = 2 * time.Second
)

func (s *Server) syncSSE(w http.ResponseWriter, r *http.Request, accountID string, cursor int64) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, chronicle.ErrInvalid)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	rc := http.NewResponseController(w)
	write := func(format string, args ...any) bool {
		_ = rc.SetWriteDeadline(time.Now().Add(sseWriteDeadline))
		if _, err := fmt.Fprintf(w, format, args...); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	ctx := r.Context()
	token := bearerToken(r)
	lastBeat := time.Now()
	cur := cursor
	for {
		// Сессия и подписка перечитываются перед каждым опросом: иначе
		// заблокированная учётка и истёкшая подписка получают события до
		// разрыва соединения (API-2).
		if _, err := s.Auth.IsParticipantSession(ctx, token); err != nil {
			return
		}
		if err := s.requirePaidSession(r); err != nil {
			return
		}
		events, err := s.Chronicle.SyncEvents(ctx, accountID, cur, chronicle.SyncBatchSize())
		if err != nil {
			log.Printf("sync sse: %v", err)
			return
		}
		for _, ev := range events {
			data, _ := json.Marshal(eventResponse(ev))
			if !write("event: sync\ndata: %s\n\n", data) {
				return
			}
			lastBeat = time.Now()
			cur = ev.Seq
		}
		if len(events) > 0 {
			continue
		}
		if time.Since(lastBeat) >= sseHeartbeat {
			if !write(":\n\n") {
				return
			}
			lastBeat = time.Now()
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(ssePollInterval):
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
			log.Printf("sync ndjson: %v", err)
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
