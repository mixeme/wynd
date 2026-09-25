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
		"kind": "multi", "max_uses": 5, "ttl_days": 7,
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
