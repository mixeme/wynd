package api_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// A20: перевыпуск не оживляет круг ни на мгновение; запоздавший новый сервер
// со старым ключом не может ни продолжить, ни подтвердить новый перенос.
func TestMoveReissueKeepsFrozenCircle(t *testing.T) {
	for _, pulling := range []bool{false, true} {
		t.Run(map[bool]string{false: "waiting", true: "pulling"}[pulling], func(t *testing.T) {
			srv, tok, circleID := fullArchiveCircle(t)
			path := "/api/v1/circles/" + circleID + "/move/reissue"
			if rec := doJSON(t, srv, http.MethodPost, path, tok, nil); rec.Code != http.StatusNotFound {
				t.Fatalf("no move: %d %s", rec.Code, rec.Body.String())
			}
			oldLink := startMove(t, srv, tok, circleID)
			frozen := readMoveState(t, srv, tok, circleID).FrozenAt
			if rec := doJSON(t, srv, http.MethodPost, path, "", nil); rec.Code != http.StatusForbidden {
				t.Fatalf("no session: %d", rec.Code)
			}
			if rec := doJSON(t, srv, http.MethodPost, path, tok, nil); rec.Code != http.StatusConflict {
				t.Fatalf("live link: %d %s", rec.Code, rec.Body.String())
			}
			if pulling {
				if rec := moveCall(t, srv, http.MethodPost, "/api/v1/move/claim", oldLink, map[string]string{"dest_url": "https://old.example"}, ""); rec.Code != http.StatusOK {
					t.Fatalf("old claim: %d %s", rec.Code, rec.Body.String())
				}
			}
			if _, err := srv.Chronicle.DB().Exec(`UPDATE circle_moves SET expires_at = ? WHERE circle_id = ?`, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano), circleID); err != nil {
				t.Fatal(err)
			}
			rec := doJSON(t, srv, http.MethodPost, path, tok, nil)
			if rec.Code != http.StatusCreated {
				t.Fatalf("reissue: %d %s", rec.Code, rec.Body.String())
			}
			newLink := jsonStr(t, rec, "link")
			if newLink == oldLink || !strings.Contains(newLink, "/move/") {
				t.Fatalf("new link = %q", newLink)
			}
			st := readMoveState(t, srv, tok, circleID)
			if st.FrozenAt != frozen || st.Move == nil || !st.Move.LinkAlive || st.Move.Status != "waiting" || st.Move.DestURL != "" {
				t.Fatalf("after reissue: %+v", st)
			}
			for _, p := range []string{"/api/v1/move/claim", "/api/v1/move/accepted"} {
				if rec := moveCall(t, srv, http.MethodPost, p, oldLink, map[string]string{"dest_url": "https://old.example"}, ""); rec.Code != http.StatusGone {
					t.Fatalf("old key %s: %d %s", p, rec.Code, rec.Body.String())
				}
			}
			if rec := moveCall(t, srv, http.MethodPost, "/api/v1/move/claim", newLink, map[string]string{"dest_url": "https://new.example"}, ""); rec.Code != http.StatusOK {
				t.Fatalf("new claim: %d %s", rec.Code, rec.Body.String())
			}
			deadline := time.Now().Add(10 * time.Second)
			for readMoveState(t, srv, tok, circleID).Move.ArchiveStatus != "ready" {
				if time.Now().After(deadline) {
					t.Fatal("new transfer archive did not become ready")
				}
				time.Sleep(10 * time.Millisecond)
			}
			var starts, unfrozen int
			if err := srv.Chronicle.DB().QueryRow(`SELECT COUNT(*) FROM events WHERE circle_id = ? AND event_type = 'circle_move_started'`, circleID).Scan(&starts); err != nil {
				t.Fatal(err)
			}
			if err := srv.Chronicle.DB().QueryRow(`SELECT COUNT(*) FROM events WHERE circle_id = ? AND event_type = 'circle_unfrozen'`, circleID).Scan(&unfrozen); err != nil {
				t.Fatal(err)
			}
			if starts != 1 || unfrozen != 0 {
				t.Fatalf("events: starts=%d unfrozen=%d", starts, unfrozen)
			}
			if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", tok, map[string]string{"body": "still frozen", "entry_date": "2026-11-02"}); rec.Code == http.StatusCreated {
				t.Fatal("reissue allowed writing")
			}
		})
	}
}

func TestMoveReissueRequiresOwnerAndUnacceptedMove(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	owner, _ := registerSession(t, srv, caps, "owner@example.com")
	other, _ := registerSession(t, srv, caps, "other@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", owner, map[string]any{"name": "Круг", "owner_name": "Аня"})
	if rec.Code != http.StatusCreated {
		t.Fatal(rec.Body.String())
	}
	id := jsonStr(t, rec, "id")
	startMove(t, srv, owner, id)
	path := "/api/v1/circles/" + id + "/move/reissue"
	if rec := doJSON(t, srv, http.MethodPost, path, other, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("other account: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := srv.Chronicle.DB().Exec(`UPDATE circle_moves SET status = 'accepted', expires_at = ? WHERE circle_id = ?`, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano), id); err != nil {
		t.Fatal(err)
	}
	if rec := doJSON(t, srv, http.MethodPost, path, owner, nil); rec.Code != http.StatusConflict {
		t.Fatalf("already accepted: %d %s", rec.Code, rec.Body.String())
	}
}
