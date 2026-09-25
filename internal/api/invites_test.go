package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
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
