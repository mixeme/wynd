package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/api"
	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

func streamCircle(t *testing.T) (*api.Server, string, string, string, string) {
	t.Helper()
	srv, tok, cid := fullArchiveCircle(t)
	payload := bytes.Repeat([]byte("immutable-media-0123456789"), 24000)
	id := uploadBytes(t, srv, tok, payload, "application/octet-stream")
	r := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+cid+"/posts", tok, map[string]any{
		"body": "body-at-snapshot", "entry_date": "2026-10-14", "media": []map[string]any{{"blob_id": id, "kind": "attachment"}},
	})
	if r.Code != http.StatusCreated {
		t.Fatal(r.Body.String())
	}
	return srv, tok, cid, id, jsonStr(t, r, "id")
}

func readyStream(t *testing.T, srv *api.Server, tok, cid string) string {
	t.Helper()
	link := startMove(t, srv, tok, cid)
	if r := moveCall(t, srv, http.MethodPost, "/api/v1/move/claim", link, map[string]any{"dest_url": "https://receiver.example"}, ""); r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		state := readMoveState(t, srv, tok, cid)
		if state.Move.ArchiveStatus == "ready" {
			return link
		}
		if state.Move.ArchiveStatus == "failed" || time.Now().After(deadline) {
			t.Fatalf("snapshot: %+v", state.Move)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func noOutgoingCopy(t *testing.T, srv *api.Server) {
	t.Helper()
	files, err := os.ReadDir(filepath.Join(srv.DataDir, "moves"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "out-") {
			t.Fatalf("outgoing archive created: %s", f.Name())
		}
	}
}

func restartAPI(t *testing.T, old *api.Server) *api.Server {
	t.Helper()
	if err := old.Chronicle.DB().Close(); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(old.DataDir, "wynd.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ch, err := chronicle.New(st)
	if err != nil {
		t.Fatal(err)
	}
	blobs, err := blob.New(st, filepath.Join(old.DataDir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	svc, err := auth.New(st, ch, auth.NewCaptureCodes(), true)
	if err != nil {
		t.Fatal(err)
	}
	srv := api.NewServer(svc, ch, blobs, nil, nil, "bootstrap", old.DataDir, old.PublicURL(), ":7676", true)
	srv.TrustedProxies = old.TrustedProxies
	srv.MoveRetry = 10 * time.Millisecond
	srv.RecoverMoveArchives(t.Context())
	return srv
}

type incomingResult struct {
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

func waitIncoming(t *testing.T, srv *api.Server, tok, want string) incomingResult {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		r := doGET(t, srv, "/api/v1/moves/incoming", tok)
		var state incomingResult
		if err := json.Unmarshal(r.Body.Bytes(), &state); err != nil {
			t.Fatal(err)
		}
		if state.Move.Status == want {
			return state
		}
		if state.Move.Status == "failed" || time.Now().After(deadline) {
			t.Fatalf("incoming: %s", r.Body.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type rangeProbe struct {
	mu                 sync.Mutex
	ranges, validators []string
	cut                int64
	change             bool
}
type limitedResponse struct {
	io.Reader
	io.Closer
}

func (p *rangeProbe) RoundTrip(r *http.Request) (*http.Response, error) {
	response, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	if r.URL.Path == "/api/v1/move/archive" && response.StatusCode == http.StatusPartialContent {
		p.mu.Lock()
		p.ranges = append(p.ranges, r.Header.Get("Range"))
		p.validators = append(p.validators, r.Header.Get("If-Range"))
		first := len(p.ranges) == 1
		p.mu.Unlock()
		if first && p.cut > 0 && response.StatusCode == http.StatusPartialContent {
			response.Body = limitedResponse{io.LimitReader(response.Body, p.cut), response.Body}
		}
		if !first && p.change {
			response.Header.Set("ETag", `"different-snapshot"`)
		}
	}
	return response, nil
}

// A21: a persisted prefix survives both server restarts and an expired link.
// The replacement link keeps identical bytes even after live metadata changes.
func TestMoveStreamRestartAndReissueKeepSnapshot(t *testing.T) {
	src, owner, cid, _, postID := streamCircle(t)
	link := readyStream(t, src, owner, cid)
	whole := moveCall(t, src, http.MethodGet, "/api/v1/move/archive", link, nil, "")
	if whole.Code != http.StatusOK {
		t.Fatal(whole.Body.String())
	}
	const prefix = 8192
	part := moveCall(t, src, http.MethodGet, "/api/v1/move/archive", link, nil, fmt.Sprintf("bytes=0-%d", prefix-1))
	if part.Code != http.StatusPartialContent || part.Body.Len() != prefix || part.Header().Get("ETag") == "" {
		t.Fatalf("prefix: %d %s", part.Code, part.Header())
	}
	if !bytes.Equal(part.Body.Bytes(), whole.Body.Bytes()[:prefix]) {
		t.Fatal("prefix differs")
	}
	noOutgoingCopy(t, src)
	var indexBytes int64
	if err := src.Chronicle.DB().QueryRow(`SELECT SUM(length(layout)+(SELECT COALESCE(SUM(length(data)),0) FROM move_snapshot_literals WHERE snapshot_id=move_snapshots.id)) FROM move_snapshots`).Scan(&indexBytes); err != nil {
		t.Fatal(err)
	}
	if indexBytes > int64(whole.Body.Len()/8) {
		t.Fatalf("full media copied into snapshot: %d", indexBytes)
	}
	var source atomic.Pointer[api.Server]
	source.Store(src)
	h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { source.Load().ServeHTTP(w, r) }))
	defer h.Close()
	fullLink := h.URL + "/move/" + link[strings.LastIndex(link, "/")+1:]
	dst, caps, _, _ := setupAPI(t)
	tok, account := registerSession(t, dst, caps, "receiver@example.com")
	r := doJSON(t, dst, http.MethodPost, "/api/v1/moves/incoming", tok, map[string]any{"link": fullLink})
	if r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	incomingID := jsonStr(t, r, "id")
	if _, err := dst.Chronicle.StartIncoming(t.Context(), incomingID, false, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dst.DataDir, "moves"), 0o700); err != nil {
		t.Fatal(err)
	}
	inPath := filepath.Join(dst.DataDir, "moves", "in-"+incomingID+".wynd")
	if err := os.WriteFile(inPath, append(part.Body.Bytes(), []byte("uncommitted-tail")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := dst.Chronicle.SetIncomingArchiveETag(t.Context(), incomingID, part.Header().Get("ETag")); err != nil {
		t.Fatal(err)
	}
	if err := dst.Chronicle.SetIncomingPulled(t.Context(), incomingID, prefix); err != nil {
		t.Fatal(err)
	}
	if _, err := src.Chronicle.DB().Exec(`UPDATE posts SET body='changed-after-snapshot' WHERE id=?`, postID); err != nil {
		t.Fatal(err)
	}
	src = restartAPI(t, src)
	source.Store(src)
	dst = restartAPI(t, dst)
	probe := &rangeProbe{}
	dst.MoveClient = &http.Client{Transport: probe}
	if _, err := src.Chronicle.DB().Exec(`UPDATE circle_moves SET expires_at=? WHERE circle_id=?`, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano), cid); err != nil {
		t.Fatal(err)
	}
	if n := src.ExpireMoveArchives(t.Context(), time.Now()); n != 0 {
		t.Fatal("expired link discarded reusable snapshot")
	}
	dst.ResumeIncomingMoves(t.Context())
	failure := waitIncoming(t, dst, tok, "failed")
	if failure.Move.Error != "move_expired" {
		t.Fatalf("wrong failure: %+v", failure)
	}
	if info, err := os.Stat(inPath); err != nil || info.Size() != prefix {
		t.Fatalf("expired link lost prefix: %v %v", info, err)
	}
	r = doJSON(t, src, http.MethodPost, "/api/v1/circles/"+cid+"/move/reissue", owner, nil)
	if r.Code != http.StatusCreated {
		t.Fatal(r.Body.String())
	}
	newLink := jsonStr(t, r, "link")
	newWholeClaim := moveCall(t, src, http.MethodPost, "/api/v1/move/claim", newLink, map[string]any{"dest_url": "https://receiver.example"}, "")
	if newWholeClaim.Code != http.StatusOK {
		t.Fatal(newWholeClaim.Body.String())
	}
	newWhole := moveCall(t, src, http.MethodGet, "/api/v1/move/archive", newLink, nil, "")
	if !bytes.Equal(newWhole.Body.Bytes(), whole.Body.Bytes()) || newWhole.Header().Get("ETag") != part.Header().Get("ETag") {
		t.Fatal("reissue changed frozen representation")
	}
	// Reissue once more so the receiver can claim its own URL; representation stays shared.
	if _, err := src.Chronicle.DB().Exec(`UPDATE circle_moves SET expires_at=? WHERE circle_id=? AND status='pulling'`, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano), cid); err != nil {
		t.Fatal(err)
	}
	r = doJSON(t, src, http.MethodPost, "/api/v1/circles/"+cid+"/move/reissue", owner, nil)
	if r.Code != http.StatusCreated {
		t.Fatal(r.Body.String())
	}
	newLink = jsonStr(t, r, "link")
	fullLink = h.URL + "/move/" + newLink[strings.LastIndex(newLink, "/")+1:]
	r = doJSON(t, dst, http.MethodPost, "/api/v1/moves/incoming", tok, map[string]any{"link": fullLink})
	if r.Code != http.StatusOK || jsonStr(t, r, "id") != incomingID {
		t.Fatalf("replacement lost incoming operation: %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, dst, http.MethodPost, "/api/v1/moves/incoming/"+incomingID+"/start", tok, map[string]any{}); r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	result := waitIncoming(t, dst, tok, "done")
	if !result.Move.Result.Verified || result.Move.Result.Posts != 3 {
		t.Fatalf("verification: %+v", result)
	}
	probe.mu.Lock()
	ranges, validators := append([]string(nil), probe.ranges...), append([]string(nil), probe.validators...)
	probe.mu.Unlock()
	if len(ranges) == 0 || !strings.HasPrefix(ranges[0], fmt.Sprintf("bytes=%d-", prefix)) || validators[0] != part.Header().Get("ETag") {
		t.Fatalf("restarted download repeated prefix: %v %v", ranges, validators)
	}
	feed, err := dst.Chronicle.FeedSnapshot(t.Context(), result.Move.Result.CircleID, account)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, post := range feed {
		if post.Post.Body == "changed-after-snapshot" {
			t.Fatal("live data replaced snapshot")
		}
		if post.Post.Body == "body-at-snapshot" {
			found = true
		}
	}
	if !found {
		t.Fatal("snapshot post lost")
	}
	noOutgoingCopy(t, src)
	var snapshots int
	if err := src.Chronicle.DB().QueryRow(`SELECT COUNT(*) FROM move_snapshots`).Scan(&snapshots); err != nil || snapshots != 0 {
		t.Fatalf("accepted snapshot retained: %d %v", snapshots, err)
	}
}

// A21: interruption halfway through a response keeps its received bytes instead
// of rolling back to the start of the whole chunk.
func TestMoveStreamKeepsPartialHTTPChunk(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "resume", true: "changed-representation"}[changed], func(t *testing.T) { testPartialHTTPChunk(t, changed) })
	}
}

func testPartialHTTPChunk(t *testing.T, changed bool) {
	src, owner, cid, _, _ := streamCircle(t)
	h := httptest.NewServer(src)
	defer h.Close()
	link := startMove(t, src, owner, cid)
	link = h.URL + "/move/" + link[strings.LastIndex(link, "/")+1:]
	dst, caps, _, _ := setupAPI(t)
	dst.MoveRetry = 10 * time.Millisecond
	probe := &rangeProbe{cut: 12345, change: changed}
	dst.MoveClient = &http.Client{Transport: probe}
	tok, _ := registerSession(t, dst, caps, "receiver@example.com")
	r := doJSON(t, dst, http.MethodPost, "/api/v1/moves/incoming", tok, map[string]any{"link": link})
	if r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	id := jsonStr(t, r, "id")
	if r := doJSON(t, dst, http.MethodPost, "/api/v1/moves/incoming/"+id+"/start", tok, map[string]any{}); r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	if changed {
		result := waitIncoming(t, dst, tok, "failed")
		if result.Move.Error != "archive_changed" {
			t.Fatalf("mixed representations accepted: %+v", result)
		}
		if r := doJSON(t, src, http.MethodPost, "/api/v1/circles/"+cid+"/move/cancel", owner, nil); r.Code != http.StatusOK {
			t.Fatal(r.Body.String())
		}
		return
	}
	result := waitIncoming(t, dst, tok, "done")
	if !result.Move.Result.Verified {
		t.Fatal("not verified")
	}
	probe.mu.Lock()
	defer probe.mu.Unlock()
	if len(probe.ranges) < 2 || !strings.HasPrefix(probe.ranges[1], "bytes=12345-") || probe.validators[1] == "" {
		t.Fatalf("partial bytes discarded: %v %v", probe.ranges, probe.validators)
	}
	noOutgoingCopy(t, src)
}

// A21: pre-upgrade archive files are reused by reference through restarts and
// reissues, then removed when the final link ends. They are never duplicated.
func TestMoveStreamLegacyReadyFileSurvivesUpgrade(t *testing.T) {
	src, owner, cid := fullArchiveCircle(t)
	if r := doJSON(t, src, http.MethodPost, "/api/v1/circles/"+cid+"/full_archive", owner, map[string]any{}); r.Code != http.StatusCreated {
		t.Fatal(r.Body.String())
	}
	waitFullStatus(t, src, owner, cid, "ready")
	file := fetch(src, fullLink(t, src, owner, cid), "")
	if file.Code != http.StatusOK {
		t.Fatal("legacy fixture download failed")
	}
	link := startMove(t, src, owner, cid)
	m, err := src.Chronicle.LiveMove(t.Context(), cid)
	if err != nil || m == nil {
		t.Fatalf("move: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(src.DataDir, "moves"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(src.DataDir, "moves", "out-"+m.ID+".wynd")
	if err := os.WriteFile(path, file.Body.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := src.Chronicle.DB().Exec(`UPDATE circle_moves SET status='pulling',archive_status='ready',legacy_archive_id=id,archive_bytes=?,archive_posts=2,archive_names=1 WHERE id=?`, file.Body.Len(), m.ID); err != nil {
		t.Fatal(err)
	}
	before := moveCall(t, src, http.MethodGet, "/api/v1/move/archive", link, nil, "bytes=0-31")
	if before.Code != http.StatusPartialContent {
		t.Fatal(before.Body.String())
	}
	src = restartAPI(t, src)
	if _, err := src.Chronicle.DB().Exec(`UPDATE circle_moves SET expires_at=? WHERE id=?`, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano), m.ID); err != nil {
		t.Fatal(err)
	}
	r := doJSON(t, src, http.MethodPost, "/api/v1/circles/"+cid+"/move/reissue", owner, nil)
	if r.Code != http.StatusCreated {
		t.Fatal(r.Body.String())
	}
	next := jsonStr(t, r, "link")
	if r := moveCall(t, src, http.MethodPost, "/api/v1/move/claim", next, map[string]any{"dest_url": "https://receiver.example"}, ""); r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	after := moveCall(t, src, http.MethodGet, "/api/v1/move/archive", next, nil, "")
	if after.Code != http.StatusOK || !bytes.Equal(after.Body.Bytes(), file.Body.Bytes()) || after.Header().Get("ETag") != before.Header().Get("ETag") {
		t.Fatal("legacy representation changed")
	}
	files, err := os.ReadDir(filepath.Join(src.DataDir, "moves"))
	if err != nil || len(files) != 1 {
		t.Fatalf("legacy file duplicated: %v %v", files, err)
	}
	if r := doJSON(t, src, http.MethodPost, "/api/v1/circles/"+cid+"/move/cancel", owner, nil); r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("legacy file not released: %v", err)
	}
}

// A21: snapshot references prevent GC, without becoming a reading permission.
// Once cancelled, an otherwise unreferenced source file can be collected.
func TestMoveStreamPinsDoNotGrantAccess(t *testing.T) {
	src, owner, cid, blobID, _ := streamCircle(t)
	link := readyStream(t, src, owner, cid)
	if _, err := src.Chronicle.DB().Exec(`DELETE FROM post_media WHERE blob_id=?`, blobID); err != nil {
		t.Fatal(err)
	}
	if err := src.Blobs.ReleaseBlobs(t.Context(), []string{blobID}); err != nil {
		t.Fatal(err)
	}
	if _, err := src.Blobs.OpenBlob(t.Context(), blobID); err != nil {
		t.Fatalf("pinned file deleted: %v", err)
	}
	if access, err := src.Blobs.CanAccessBlob(t.Context(), "other-account", blobID); err != nil || access {
		t.Fatalf("pin grants access: %v %v", access, err)
	}
	if r := moveCall(t, src, http.MethodGet, "/api/v1/move/archive", link, nil, ""); r.Code != http.StatusOK {
		t.Fatal("pinned archive unavailable")
	}
	if r := doJSON(t, src, http.MethodPost, "/api/v1/circles/"+cid+"/move/cancel", owner, nil); r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	if _, err := src.Blobs.OpenBlob(t.Context(), blobID); err == nil {
		t.Fatal("cancelled snapshot kept unreferenced file")
	}
}
