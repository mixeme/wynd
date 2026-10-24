package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// «Отделиться» через API (A9): предпросмотр, новый круг, приглашение
// позванной — строка в её списке, экран «позвали» и вход под прежним именем.
func TestSplitCircleAPI(t *testing.T) {
	srv, caps, ch, _ := setupAPI(t)
	anyaTok, anyaID := registerSession(t, srv, caps, "anya@example.com")
	mouseTok, mouseID := registerSession(t, srv, caps, "mouse@example.com")
	strangerTok, _ := registerSession(t, srv, caps, "stranger@example.com")

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", anyaTok, map[string]any{"name": "Семья", "owner_name": "Аня"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	familyID := jsonStr(t, rec, "id")
	if _, _, err := ch.Join(context.Background(), chronicle.JoinInput{CircleID: familyID, AccountID: mouseID, Name: "Мышь", Gender: chronicle.GenderFemale, Now: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+familyID+"/posts", anyaTok, map[string]any{"body": "на даче", "entry_date": "2026-08-05"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("post: %d %s", rec.Code, rec.Body.String())
	}

	if rec := doGET(t, srv, "/api/v1/circles/"+familyID+"/split", strangerTok); rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
		t.Fatalf("чужой видит предпросмотр: %d %s", rec.Code, rec.Body.String())
	}
	rec = doGET(t, srv, "/api/v1/circles/"+familyID+"/split", mouseTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	var pre struct {
		Posts  int `json:"posts"`
		People []struct {
			AccountID string `json:"account_id"`
			Name      string `json:"name"`
		} `json:"people"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &pre); err != nil {
		t.Fatal(err)
	}
	if pre.Posts != 1 || len(pre.People) != 1 || pre.People[0].AccountID != anyaID {
		t.Fatalf("preview: %s", rec.Body.String())
	}
	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+familyID+"/split", mouseTok, map[string]any{"name": ""}); rec.Code != http.StatusBadRequest {
		t.Fatalf("без имени: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+familyID+"/split", mouseTok, map[string]any{
		"name": "Наша семья", "invite": []string{anyaID},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("split: %d %s", rec.Code, rec.Body.String())
	}
	newID := jsonStr(t, rec, "circle_id")

	rec = doGET(t, srv, "/api/v1/pending-circle-joins", anyaTok)
	var pending struct {
		CircleIDs []string `json:"circle_ids"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &pending); err != nil || len(pending.CircleIDs) != 1 || pending.CircleIDs[0] != newID {
		t.Fatalf("строка «зовёт вас»: %s", rec.Body.String())
	}
	rec = doGET(t, srv, "/api/v1/circles/"+newID+"/join-preview", anyaTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("join preview: %d %s", rec.Code, rec.Body.String())
	}
	var peek struct {
		CircleName string `json:"circle_name"`
		Split      struct {
			From       string `json:"split_from"`
			InviteLine string `json:"invite_line"`
			YouName    string `json:"you_name"`
			PastFrom   string `json:"past_from"`
		} `json:"split"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &peek); err != nil {
		t.Fatal(err)
	}
	if peek.CircleName != "Наша семья" || peek.Split.From != "Семья" || peek.Split.YouName != "Аня" || peek.Split.PastFrom == "" ||
		peek.Split.InviteLine != "Мышь отделила круг от круга «Семья» и зовёт вас" {
		t.Fatalf("экран «позвали»: %s", rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+newID+"/join", anyaTok, map[string]string{"name": "Аня"})
	if rec.Code != http.StatusOK {
		t.Fatalf("join: %d %s", rec.Code, rec.Body.String())
	}
	rec = doGET(t, srv, "/api/v1/circles/"+newID+"/feed?limit=5", anyaTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("feed: %d %s", rec.Code, rec.Body.String())
	}
}
