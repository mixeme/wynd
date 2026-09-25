package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

func TestInvitePeekAndDeferredJoin(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, _ := registerSession(t, srv, caps, "anya@example.com")

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", ownerTok, map[string]any{
		"name": "Семья", "owner_name": "Аня", "color": "olive",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")

	allowCircleMultiInvites(t, srv, circleID, ownerTok)
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/invites", ownerTok, map[string]any{
		"kind": "multi", "max_uses": 5, "ttl_sec": 604800,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("invite: %d %s", rec.Code, rec.Body.String())
	}
	token := jsonStr(t, rec, "token")

	rec = doGET(t, srv, "/api/v1/invites/"+token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("peek: %d %s", rec.Code, rec.Body.String())
	}
	if jsonStr(t, rec, "circle_name") != "Семья" {
		t.Fatalf("peek circle_name: %s", rec.Body.String())
	}
	if jsonStr(t, rec, "color") != "olive" {
		t.Fatalf("peek color: %s", rec.Body.String())
	}
	var peek map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &peek); err != nil {
		t.Fatal(err)
	}
	members, ok := peek["members"].([]any)
	if !ok || len(members) != 1 {
		t.Fatalf("peek members: %s", rec.Body.String())
	}
	first, _ := members[0].(map[string]any)
	if first["name"] != "Аня" || first["is_owner"] != true {
		t.Fatalf("owner in peek: %v", first)
	}

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/invites/"+token+"/accept", "", map[string]string{
		"email": "bob@example.com",
	})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("accept without name: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/auth/verify", "", map[string]string{
		"email": "bob@example.com", "code": caps.Last("bob@example.com"),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("verify: %d %s", rec.Code, rec.Body.String())
	}
	if jsonStr(t, rec, "pending_circle_id") != circleID {
		t.Fatalf("pending_circle_id: %s", rec.Body.String())
	}
	bobTok := jsonStr(t, rec, "token")

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/invites/"+token+"/join", bobTok, map[string]string{
		"name": "Боб", "body": "Привет всем",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("join: %d %s", rec.Code, rec.Body.String())
	}

	rec = doGET(t, srv, "/api/v1/circles/"+circleID+"/feed?limit=5", bobTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("feed: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Привет всем") {
		t.Fatalf("expected first post in feed: %s", rec.Body.String())
	}
}

func TestCircleInviteRejectsMultiWhenSingleOnly(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, _ := registerSession(t, srv, caps, "solo@example.com")

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", ownerTok, map[string]any{
		"name": "Только одно", "owner_name": "Владелец", "color": "olive",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/invites", ownerTok, map[string]any{
		"kind": "multi", "max_uses": 5, "ttl_sec": 3600,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invite multi when single-only: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/invites", ownerTok, map[string]any{
		"kind": "single", "ttl_sec": 3600,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("invite single: %d %s", rec.Code, rec.Body.String())
	}
}

func TestCircleInviteListAndRevoke(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, _ := registerSession(t, srv, caps, "owner@example.com")

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", ownerTok, map[string]any{
		"name": "Дача", "owner_name": "Владелец", "color": "ochre",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/invites", ownerTok, map[string]any{
		"kind": "single", "ttl_sec": 3600,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("invite single: %d %s", rec.Code, rec.Body.String())
	}
	singleID := jsonStr(t, rec, "id")

	allowCircleMultiInvites(t, srv, circleID, ownerTok)
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/invites", ownerTok, map[string]any{
		"kind": "multi", "max_uses": 10, "ttl_sec": 604800,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("invite multi: %d %s", rec.Code, rec.Body.String())
	}
	multiID := jsonStr(t, rec, "id")

	rec = doGET(t, srv, "/api/v1/circles/"+circleID+"/invites", ownerTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("list invites: %d %s", rec.Code, rec.Body.String())
	}
	var listed map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	invites, ok := listed["invites"].([]any)
	if !ok || len(invites) != 2 {
		t.Fatalf("expected 2 live invites, got: %s", rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodDelete, "/api/v1/circles/"+circleID+"/invites/"+multiID, ownerTok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke multi: %d %s", rec.Code, rec.Body.String())
	}

	rec = doGET(t, srv, "/api/v1/circles/"+circleID+"/invites", ownerTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("list after revoke: %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	invites, ok = listed["invites"].([]any)
	if !ok || len(invites) != 1 {
		t.Fatalf("expected 1 live invite after revoke, got: %s", rec.Body.String())
	}
	first, _ := invites[0].(map[string]any)
	if first["id"] != singleID {
		t.Fatalf("remaining invite id: %v", first["id"])
	}

	rec = doJSON(t, srv, http.MethodDelete, "/api/v1/circles/"+circleID+"/invites/wrong-id", ownerTok, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("revoke foreign invite: %d %s", rec.Code, rec.Body.String())
	}
}

func TestServerInvitePeek(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	admin := adminToken(t, srv)

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/admin/invites", admin, map[string]any{
		"kind": "multi", "max_uses": 5, "ttl_sec": 3600,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("server invite: %d %s", rec.Code, rec.Body.String())
	}
	token := jsonStr(t, rec, "token")

	rec = doGET(t, srv, "/api/v1/invites/"+token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("peek empty server: %d %s", rec.Code, rec.Body.String())
	}
	if jsonStr(t, rec, "server_name") != "Дом Ани" {
		t.Fatalf("peek server_name: %s", rec.Body.String())
	}
	if jsonStr(t, rec, "host") == "" {
		t.Fatalf("peek host empty: %s", rec.Body.String())
	}
	var emptyPeek map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &emptyPeek); err != nil {
		t.Fatal(err)
	}
	if _, ok := emptyPeek["circle_name"]; ok {
		t.Fatalf("server peek must not include circle_name: %s", rec.Body.String())
	}
	if _, ok := emptyPeek["inviter_name"]; ok {
		t.Fatalf("no circles yet: inviter_name should be omitted: %s", rec.Body.String())
	}

	ownerTok, _ := registerSession(t, srv, caps, "slava@example.com")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles", ownerTok, map[string]any{
		"name": "Семья", "owner_name": "Слава", "color": "olive",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create circle: %d %s", rec.Code, rec.Body.String())
	}

	rec = doGET(t, srv, "/api/v1/invites/"+token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("peek after circle: %d %s", rec.Code, rec.Body.String())
	}
	if jsonStr(t, rec, "inviter_name") != "Слава" {
		t.Fatalf("peek inviter_name: %s", rec.Body.String())
	}

	expired, err := srv.Auth.CreateServerInvite(t.Context(), auth.CreateServerInviteInput{
		Kind: auth.InviteSingle, MaxUses: 1, TTL: time.Hour,
		Now: time.Now().UTC().Add(-2 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	rec = doGET(t, srv, "/api/v1/invites/"+expired.Token, "")
	if rec.Code != http.StatusGone {
		t.Fatalf("expired server peek: %d %s", rec.Code, rec.Body.String())
	}
}

// Инвариант (SEC-9, аудит 2026-09-22): просмотр ссылки проверяет её так же,
// как и вход по ней. Раньше смотрели только отзыв и срок, и исчерпанная
// ссылка показывала посторонним название круга и список участников.
func TestPeekRefusesExhaustedInvite(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, _ := registerSession(t, srv, caps, "anya@example.com")

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", ownerTok, map[string]any{
		"name": "Семья", "owner_name": "Аня", "color": "olive",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/invites", ownerTok, map[string]any{
		"kind": "single", "max_uses": 1, "ttl_sec": 3600,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("invite: %d %s", rec.Code, rec.Body.String())
	}
	token := jsonStr(t, rec, "token")

	rec = doGET(t, srv, "/api/v1/invites/"+token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("peek живой ссылки: %d %s", rec.Code, rec.Body.String())
	}

	// Ссылку израсходовали: вход по ней уже невозможен, просмотр — тоже.
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/invites/"+token+"/accept", "", map[string]string{
		"email": "bob@example.com", "name": "Боб",
	})
	if rec.Code != http.StatusAccepted && rec.Code != http.StatusOK {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body.String())
	}
	code := caps.Last("bob@example.com")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/auth/verify", "", map[string]string{
		"email": "bob@example.com", "code": code,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("verify: %d %s", rec.Code, rec.Body.String())
	}

	rec = doGET(t, srv, "/api/v1/invites/"+token, "")
	if rec.Code == http.StatusOK {
		t.Fatalf("исчерпанная ссылка показала круг: %s", rec.Body.String())
	}
}
