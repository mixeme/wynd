package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestMemberInviteFromOtherCircle(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, ownerID := registerSession(t, srv, caps, "anya@example.com")

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", ownerTok, map[string]any{
		"name": "Семья", "owner_name": "Аня", "color": "olive",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create family: %d %s", rec.Code, rec.Body.String())
	}
	familyID := jsonStr(t, rec, "id")

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles", ownerTok, map[string]any{
		"name": "Дача", "owner_name": "Аня", "color": "ochre",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create dacha: %d %s", rec.Code, rec.Body.String())
	}
	dachaID := jsonStr(t, rec, "id")

	bobTok, bobID := registerSession(t, srv, caps, "bob@example.com")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+familyID+"/invites", ownerTok, map[string]any{
		"kind": "single", "ttl_sec": 3600,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("family invite: %d %s", rec.Code, rec.Body.String())
	}
	token := jsonStr(t, rec, "token")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/invites/"+token+"/accept", "", map[string]string{
		"email": "bob@example.com",
	})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/auth/verify", "", map[string]string{
		"email": "bob@example.com", "code": caps.Last("bob@example.com"),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("verify bob: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/invites/"+token+"/join", bobTok, map[string]string{
		"name": "Боб",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("join family: %d %s", rec.Code, rec.Body.String())
	}

	rec = doGET(t, srv, "/api/v1/circles/"+dachaID+"/invite-candidates", ownerTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("candidates: %d %s", rec.Code, rec.Body.String())
	}
	var listed map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	groups, ok := listed["groups"].([]any)
	if !ok || len(groups) != 1 {
		t.Fatalf("expected one source group, got %s", rec.Body.String())
	}
	first, _ := groups[0].(map[string]any)
	members, _ := first["members"].([]any)
	if len(members) != 1 {
		t.Fatalf("expected bob in candidates: %s", rec.Body.String())
	}
	m0, _ := members[0].(map[string]any)
	if m0["account_id"] != bobID || m0["invited"] != false {
		t.Fatalf("candidate bob: %v", m0)
	}

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+dachaID+"/member-invites", ownerTok, map[string]string{
		"account_id": bobID,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("member invite: %d %s", rec.Code, rec.Body.String())
	}

	rec = doGET(t, srv, "/api/v1/circles/"+dachaID+"/invites", ownerTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("live invites: %d %s", rec.Code, rec.Body.String())
	}
	var live struct {
		Invites []any `json:"invites"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &live); err != nil {
		t.Fatal(err)
	}
	if len(live.Invites) != 0 {
		t.Fatalf("personal invite leaked into live list: %s", rec.Body.String())
	}

	rec = doGET(t, srv, "/api/v1/pending-circle-joins", bobTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("pending joins: %d %s", rec.Code, rec.Body.String())
	}
	var pending struct {
		CircleIDs []string `json:"circle_ids"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &pending); err != nil {
		t.Fatal(err)
	}
	if len(pending.CircleIDs) != 1 || pending.CircleIDs[0] != dachaID {
		t.Fatalf("pending joins: %s", rec.Body.String())
	}

	rec = doGET(t, srv, "/api/v1/circles", bobTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("bob circles: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), dachaID) {
		t.Fatalf("pending circle listed before join: %s", rec.Body.String())
	}

	rec = doGET(t, srv, "/api/v1/circles/"+dachaID+"/feed?limit=5", bobTok)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Fatalf("feed before join: %d %s", rec.Code, rec.Body.String())
	}

	rec = doGET(t, srv, "/api/v1/circles/"+dachaID+"/join-preview", bobTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("join preview: %d %s", rec.Code, rec.Body.String())
	}
	if jsonStr(t, rec, "circle_name") != "Дача" {
		t.Fatalf("preview name: %s", rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+dachaID+"/join", bobTok, map[string]string{
		"name": "Бобик",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("join dacha: %d %s", rec.Code, rec.Body.String())
	}

	rec = doGET(t, srv, "/api/v1/circles/"+dachaID+"/feed?limit=5", bobTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("feed: %d %s", rec.Code, rec.Body.String())
	}

	_ = ownerID
}
