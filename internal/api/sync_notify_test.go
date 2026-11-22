package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/api"
)

// openSSEFor открывает поток /sync и читает его, пока не кончится срок.
// Возвращает всё, что пришло: hello, sync, street, notify.
func openSSEFor(t *testing.T, srv *api.Server, token string, dur time.Duration) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/sync", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec.Body.String()
}

// Живой сигнал уведомлений (план 66): новую запись участнику несёт рамка
// event: notify в его потоке — тем же правилом и из той же точки, что пуш.
// Автор своего поста не уведомляется.
func TestSSENotifyFrameForNewPost(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, _ := registerSession(t, srv, caps, "owner-notify@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", ownerTok, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	circleID := jsonStr(t, rec, "id")
	allowCircleMultiInvites(t, srv, circleID, ownerTok)
	inv := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/invites", ownerTok, map[string]any{
		"kind": "multi", "max_uses": 5, "ttl_sec": 7 * 86400,
	})
	bobTok := acceptInvite(t, srv, caps, jsonStr(t, inv, "token"), "bob-notify@example.com", "Боб")

	// Оба потока открыты до записи: сигнал идёт живому слушателю.
	done := make(chan string, 2)
	go func() { done <- openSSEFor(t, srv, bobTok, 3*time.Second) }()
	go func() { done <- openSSEFor(t, srv, ownerTok, 3*time.Second) }()
	time.Sleep(300 * time.Millisecond)

	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", ownerTok, map[string]any{
		"body": "привет из круга", "entry_date": "2026-11-22",
	}); rec.Code != http.StatusCreated {
		t.Fatalf("post: %d %s", rec.Code, rec.Body.String())
	}

	var bobBody, ownerBody string
	for i := 0; i < 2; i++ {
		select {
		case body := <-done:
			if strings.Contains(body, "event: notify") {
				bobBody = body
			} else if ownerBody == "" {
				ownerBody = body
			}
		case <-time.After(5 * time.Second):
			t.Fatal("потоки не закрылись")
		}
	}
	if bobBody == "" {
		t.Fatal("в потоке участника нет рамки notify")
	}
	if !strings.Contains(bobBody, `"type":"post"`) || !strings.Contains(bobBody, `"title":"Семья"`) {
		t.Fatalf("рамка notify без типа и имени круга:\n%s", bobBody)
	}
	if strings.Contains(ownerBody, "event: notify") {
		t.Fatalf("автор получил уведомление о своём посте:\n%s", ownerBody)
	}
}

// Тишина на срок глушит и живой сигнал: рамки notify нет, sync-события есть.
func TestSSENotifyRespectsMute(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, _ := registerSession(t, srv, caps, "owner-mute@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", ownerTok, map[string]any{
		"name": "Дача", "owner_name": "Аня",
	})
	circleID := jsonStr(t, rec, "id")
	allowCircleMultiInvites(t, srv, circleID, ownerTok)
	inv := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/invites", ownerTok, map[string]any{
		"kind": "multi", "max_uses": 5, "ttl_sec": 7 * 86400,
	})
	bobTok := acceptInvite(t, srv, caps, jsonStr(t, inv, "token"), "bob-mute@example.com", "Боб")

	if rec := doJSON(t, srv, http.MethodPut, "/api/v1/notify_prefs", bobTok, map[string]any{
		"mute_until": time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
	}); rec.Code != http.StatusOK {
		t.Fatalf("mute: %d %s", rec.Code, rec.Body.String())
	}

	body := make(chan string, 1)
	go func() { body <- openSSEFor(t, srv, bobTok, 3*time.Second) }()
	time.Sleep(300 * time.Millisecond)

	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", ownerTok, map[string]any{
		"body": "в тишину", "entry_date": "2026-11-22",
	}); rec.Code != http.StatusCreated {
		t.Fatalf("post: %d %s", rec.Code, rec.Body.String())
	}

	select {
	case got := <-body:
		if strings.Contains(got, "event: notify") {
			t.Fatalf("тишина пропустила живой сигнал:\n%s", got)
		}
		if !strings.Contains(got, "event: sync") {
			t.Fatal("sync-событие при тишине пропало")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("поток не закрылся")
	}
}
