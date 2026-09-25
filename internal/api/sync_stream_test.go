package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/api"
)

// Инвариант (API-1): отмена базового контекста закрывает открытый SSE-поток
// сразу. Без этого Shutdown ждал поток все 10 секунд и процесс уходил по
// таймауту с кодом 1.
func TestSSEClosesWhenBaseContextCancelled(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	token, _ := registerSession(t, srv, caps, "sse@example.com")

	ctx, cancel := context.WithCancel(t.Context())
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/sync", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		srv.ServeHTTP(rec, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("поток не закрылся после отмены контекста")
	}
}

// Инвариант (API-2): заблокированная учётка теряет поток на ближайшем опросе,
// а не досматривает события до разрыва соединения.
func TestSSEStopsForBlockedAccount(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	token, accountID := registerSession(t, srv, caps, "blocked@example.com")

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/sync", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		srv.ServeHTTP(rec, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	if err := srv.Auth.SetAccountBlocked(t.Context(), accountID, true); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("поток заблокированной учётки жив")
	}
}

// Кадр-пульс — комментарий SSE: прокси с таймаутом чтения не рвёт тихий поток.
func TestSSEFrameFormat(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	token, _ := registerSession(t, srv, caps, "frames@example.com")
	rec0 := doJSON(t, srv, http.MethodPost, "/api/v1/circles", token, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	if rec0.Code != http.StatusCreated {
		t.Fatalf("create circle: %d %s", rec0.Code, rec0.Body.String())
	}

	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/sync", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "event: sync\ndata: {") {
		t.Fatalf("нет кадра события:\n%s", body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}
}

// Инвариант (план 43, A2): /sync — только SSE. Запрос без text/event-stream
// получает 406, а не молчаливый JSON-ответ.
func TestSyncRejectsNonSSE(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	token, _ := registerSession(t, srv, caps, "accept@example.com")
	for _, accept := range []string{"", "application/json", "application/x-ndjson"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sync?cursor=0", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotAcceptable || !strings.Contains(rec.Body.String(), "not_acceptable") {
			t.Fatalf("Accept %q: %d %s", accept, rec.Code, rec.Body.String())
		}
	}
}

// syncSSE читает поток /sync с курсора. Накопленное сервер отдаёт сразу, а
// следующий опрос — через 2 с, поэтому 300 мс хватает на всю пачку.
// Возвращает max_seq из кадра hello и события по порядку.
func syncSSE(t *testing.T, srv *api.Server, token string, cursor int64) (int64, []map[string]any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/sync?cursor="+strconv.FormatInt(cursor, 10), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("sync: %d %s", rec.Code, rec.Body.String())
	}

	var maxSeq int64
	var events []map[string]any
	for _, frame := range strings.Split(rec.Body.String(), "\n\n") {
		name, rest, ok := strings.Cut(frame, "\n")
		if !ok {
			continue // пульс «:» или хвост
		}
		data := strings.TrimPrefix(rest, "data: ")
		switch name {
		case "event: hello":
			var hello struct {
				MaxSeq int64 `json:"max_seq"`
			}
			if err := json.Unmarshal([]byte(data), &hello); err != nil {
				t.Fatalf("кадр hello %q: %v", data, err)
			}
			maxSeq = hello.MaxSeq
		case "event: sync":
			var ev map[string]any
			if err := json.Unmarshal([]byte(data), &ev); err != nil {
				t.Fatalf("кадр sync %q: %v", data, err)
			}
			events = append(events, ev)
		}
	}
	return maxSeq, events
}
