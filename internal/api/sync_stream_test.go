package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
