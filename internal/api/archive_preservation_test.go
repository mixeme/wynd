package api_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/api"
	"gitea.mixdep.ru/mix/wynd/internal/archive"
)

func archivePreservationCircle(t *testing.T) (srv *api.Server, owner, member, cid, memberID string) {
	t.Helper()
	srv, caps, _, _ := setupAPI(t)
	owner, _ = registerSession(t, srv, caps, "anya@example.com")
	cid = jsonStr(t, doJSON(t, srv, http.MethodPost, "/api/v1/circles", owner, map[string]any{"name": "Семья", "owner_name": "Аня"}), "id")
	post := func(body string) {
		t.Helper()
		if r := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+cid+"/posts", owner, map[string]any{"body": body, "entry_date": "2026-10-14"}); r.Code != http.StatusCreated {
			t.Fatalf("post: %d %s", r.Code, r.Body.String())
		}
	}
	post("hidden-before-entry")
	member = joinAsMember(t, srv, caps, owner, cid, "bob@example.com", "Боб")
	memberID = memberAccountID(t, srv, owner, cid, "Боб")
	post("visible-after-entry")
	return
}

// A19: готовая и потоковая bearer-ссылки отзываются и после возврата владения.
func TestArchivePreservationRevokesDownloadLinks(t *testing.T) {
	srv, owner, member, cid, memberID := archivePreservationCircle(t)
	ownerID := memberAccountID(t, srv, owner, cid, "Аня")
	if r := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+cid+"/full_archive", owner, map[string]any{}); r.Code != http.StatusCreated {
		t.Fatal(r.Body.String())
	}
	st := waitFullStatus(t, srv, owner, cid, "ready")
	ready := fullLink(t, srv, owner, cid)
	stream := jsonStr(t, doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+cid+"/full_archive/stream_link", owner, map[string]any{}), "url")
	if r := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+cid+"/transfer", owner, map[string]any{"new_owner_account_id": memberID}); r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	if fetch(srv, ready, "").Code != http.StatusNotFound {
		t.Fatal("old ready link survived transfer")
	}
	if _, err := os.Stat(filepath.Join(srv.DataDir, "archives", st.Archive.ID+".wynd")); !os.IsNotExist(err) {
		t.Fatalf("old file remains: %v", err)
	}
	if r := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+cid+"/transfer", member, map[string]any{"new_owner_account_id": ownerID, "transfer_past": true}); r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	for _, url := range []string{ready, stream} {
		if r := fetch(srv, url, ""); r.Code != http.StatusNotFound {
			t.Fatalf("old link survived return: %d", r.Code)
		}
	}
	if st := readFullState(t, srv, owner, cid); st.Archive != nil {
		t.Fatal("old archive reappeared after return")
	}
}

// A19: оценка, машинная и читаемая части совпадают; удаление после
// частичного архива требует того же явно подтверждённого предупреждения.
func TestArchivePreservationPartialArtifactAndRemoval(t *testing.T) {
	srv, owner, member, cid, memberID := archivePreservationCircle(t)
	if r := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+cid+"/transfer", owner, map[string]any{"new_owner_account_id": memberID}); r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	var preview struct {
		Posts int  `json:"posts"`
		Full  bool `json:"full_journal"`
	}
	if err := json.Unmarshal(doGET(t, srv, "/api/v1/circles/"+cid+"/full_archive", member).Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Posts != 1 || preview.Full {
		t.Fatalf("preview: %+v", preview)
	}
	if r := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+cid+"/full_archive", member, map[string]any{"delete_circle": true}); r.Code != http.StatusCreated {
		t.Fatal(r.Body.String())
	}
	st := waitFullStatus(t, srv, member, cid, "ready")
	job, err := srv.Chronicle.GetFullArchive(t.Context(), st.Archive.ID)
	if err != nil || job.FullJournal || job.Posts != 1 {
		t.Fatalf("snapshot metadata: %+v %v", job, err)
	}
	file := fetch(srv, fullLink(t, srv, member, cid), "")
	if file.Code != http.StatusOK {
		t.Fatalf("download: %d %s", file.Code, file.Body.String())
	}
	z, err := zip.NewReader(bytes.NewReader(file.Body.Bytes()), int64(file.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range z.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(b, []byte("hidden-before-entry")) {
			t.Fatalf("hidden history in %s", f.Name)
		}
		if f.Name == "manifest.json" {
			var m archive.FullManifest
			if err := json.Unmarshal(b, &m); err != nil || m.Posts != 1 || m.JournalScope != "partial" {
				t.Fatalf("manifest: %s %v", b, err)
			}
		}
		if f.Name == "index.html" && !bytes.Contains(b, []byte("Часть журнала круга")) {
			t.Fatal("partial file claims completeness")
		}
	}
	path := "/api/v1/circles/" + cid + "/full_archive/remove_circle"
	if r := doJSON(t, srv, http.MethodPost, path, member, nil); r.Code != http.StatusConflict || jsonStr(t, r, "error") != "partial_archive_confirmation_required" {
		t.Fatalf("no warning consent: %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, srv, http.MethodPost, path, member, map[string]any{"acknowledge_partial": true}); r.Code != http.StatusOK {
		t.Fatalf("confirmed removal: %d %s", r.Code, r.Body.String())
	}
}

// A19: межсерверный перенос доставляет весь физический журнал, сохраняя
// ограничение выгрузки и отдельную видимость нового лица.
func TestArchivePreservationMoveBetweenServers(t *testing.T) {
	src, owner, member, cid, memberID := archivePreservationCircle(t)
	if r := doJSON(t, src, http.MethodPost, "/api/v1/circles/"+cid+"/transfer", owner, map[string]any{"new_owner_account_id": memberID}); r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	h := httptest.NewServer(src)
	defer h.Close()
	link := startMove(t, src, member, cid)
	link = h.URL + "/move/" + link[strings.LastIndex(link, "/")+1:]
	dst, caps, _, _ := setupAPI(t)
	dst.MoveRetry = 10 * time.Millisecond
	tok, account := registerSession(t, dst, caps, "stranger@example.com")
	r := doJSON(t, dst, http.MethodPost, "/api/v1/moves/incoming", tok, map[string]any{"link": link})
	if r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	id := jsonStr(t, r, "id")
	if r := doJSON(t, dst, http.MethodPost, "/api/v1/moves/incoming/"+id+"/start", tok, map[string]any{"new_member": map[string]any{"name": "Слава"}}); r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	var state struct {
		Move struct {
			Status string `json:"status"`
			Error  string `json:"error"`
			Result struct {
				CircleID string `json:"circle_id"`
				Posts    int    `json:"posts"`
				Verified bool   `json:"verified"`
			} `json:"result"`
		} `json:"move"`
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		r := doGET(t, dst, "/api/v1/moves/incoming", tok)
		if err := json.Unmarshal(r.Body.Bytes(), &state); err != nil {
			t.Fatal(err)
		}
		if state.Move.Status == "done" || state.Move.Status == "failed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("move timeout")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if state.Move.Status != "done" || !state.Move.Result.Verified || state.Move.Result.Posts != 2 {
		t.Fatalf("move: %+v", state.Move)
	}
	result := state.Move.Result.CircleID
	raw, err := dst.Chronicle.FullArchiveData(t.Context(), result)
	if err != nil || raw.Posts != 2 {
		t.Fatalf("physical history: %+v %v", raw, err)
	}
	copy, err := dst.Chronicle.OwnerArchiveData(t.Context(), result)
	if err != nil || copy.Posts != 1 || !copy.PartialJournal {
		t.Fatalf("preservation widened: %+v %v", copy, err)
	}
	feed, err := dst.Chronicle.FeedSnapshot(t.Context(), result, account)
	if err != nil || len(feed) != 0 {
		t.Fatalf("new face reading: %+v %v", feed, err)
	}
}
