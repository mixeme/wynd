package api_test

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/api"
	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

// This helper runs as an independent OS process, with only synthetic data.
func TestMoveProcessHelper(t *testing.T) {
	dir := os.Getenv("WYND_TEST_MOVE_DIR")
	if dir == "" {
		return
	}
	st, err := store.Open(filepath.Join(dir, "wynd.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ch, err := chronicle.New(st)
	if err != nil {
		t.Fatal(err)
	}
	bs, err := blob.New(st, filepath.Join(dir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	as, err := auth.New(st, ch, auth.NewCaptureCodes(), true)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", os.Getenv("WYND_TEST_MOVE_LISTEN"))
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	url := "http://" + ln.Addr().String()
	srv := api.NewServer(as, ch, bs, nil, nil, "bootstrap", dir, url, ln.Addr().String(), true)
	allowLocalMoveSource(srv)
	srv.MoveRetry = 20 * time.Millisecond
	srv.RecoverMoveArchives(t.Context())
	srv.ResumeIncomingMoves(t.Context())
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/move/archive" {
			f, err := os.OpenFile(filepath.Join(dir, "ranges.txt"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				http.Error(w, "test log", 500)
				return
			}
			fmt.Fprintln(f, r.Header.Get("Range")+" "+r.Header.Get("If-Range"))
			f.Close()
			if os.Getenv("WYND_TEST_MOVE_CUT") == "1" {
				w = &cutMoveWriter{ResponseWriter: w, request: r, marker: filepath.Join(dir, "cut.ready"), remaining: 1 << 20}
			}
		}
		srv.ServeHTTP(w, r)
	})
	fmt.Println(url)
	if err := http.Serve(ln, handler); err != nil {
		t.Fatal(err)
	}
}

type cutMoveWriter struct {
	http.ResponseWriter
	request   *http.Request
	marker    string
	remaining int
}

func (w *cutMoveWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p[:min(len(p), w.remaining)])
	w.remaining -= n
	if err != nil {
		return n, err
	}
	if w.remaining == 0 {
		if f, ok := w.ResponseWriter.(http.Flusher); ok {
			f.Flush()
		}
		if err := os.WriteFile(w.marker, []byte("ready"), 0o600); err != nil {
			return n, err
		}
		<-w.request.Context().Done()
		return n, io.ErrUnexpectedEOF
	}
	return n, nil
}

type moveProcess struct {
	cmd    *exec.Cmd
	url    string
	stderr bytes.Buffer
	once   sync.Once
}

func startMoveProcess(t *testing.T, dir, address string, cut bool) *moveProcess {
	t.Helper()
	p := &moveProcess{cmd: exec.Command(os.Args[0], "-test.run=^TestMoveProcessHelper$")}
	p.cmd.Env = append(os.Environ(), "WYND_TEST_MOVE_DIR="+dir, "WYND_TEST_MOVE_LISTEN="+address, fmt.Sprintf("WYND_TEST_MOVE_CUT=%d", map[bool]int{true: 1, false: 0}[cut]))
	p.cmd.Stderr = &p.stderr
	out, err := p.cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.stop)
	ready := make(chan string, 1)
	go func() {
		scan := bufio.NewScanner(out)
		if scan.Scan() {
			ready <- scan.Text()
		} else {
			ready <- ""
		}
		_, _ = io.Copy(io.Discard, out)
	}()
	select {
	case p.url = <-ready:
		if !strings.HasPrefix(p.url, "http://127.0.0.1:") {
			p.stop()
			t.Fatalf("child startup: %q %s", p.url, p.stderr.String())
		}
	case <-time.After(15 * time.Second):
		p.stop()
		t.Fatalf("child startup timeout: %s", p.stderr.String())
	}
	return p
}

func (p *moveProcess) stop() { p.once.Do(func() { _ = p.cmd.Process.Kill(); _ = p.cmd.Wait() }) }

func processJSON(t *testing.T, p *moveProcess, method, path, token string, body any) map[string]any {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, p.url+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("%s %s: %d %s", method, path, resp.StatusCode, b)
	}
	var result map[string]any
	if err = json.Unmarshal(b, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func waitProcessCondition(t *testing.T, what string, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for !f() {
		if time.Now().After(deadline) {
			t.Fatal("timeout: " + what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// VAL-01: a real TCP transfer larger than a 32 MiB chunk survives abrupt
// termination of both processes, preserving the committed prefix and validator.
func TestMoveLargeArchiveAcrossBothProcessRestarts(t *testing.T) {
	if testing.Short() {
		t.Skip("40 MiB process integration")
	}
	src, owner, cid := fullArchiveCircle(t)
	payload := make([]byte, 40<<20)
	if _, err := rand.Read(payload); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	blobID := uploadBytes(t, src, owner, payload, "application/octet-stream")
	r := doJSON(t, src, http.MethodPost, "/api/v1/circles/"+cid+"/posts", owner, map[string]any{
		"body": "large immutable media", "entry_date": "2026-10-14", "media": []map[string]any{{"blob_id": blobID, "kind": "attachment"}},
	})
	if r.Code != http.StatusCreated {
		t.Fatal(r.Body.String())
	}
	dst, caps, _, _ := setupAPI(t)
	token, _ := registerSession(t, dst, caps, "anya@example.com")
	if err := src.Chronicle.DB().Close(); err != nil {
		t.Fatal(err)
	}
	if err := dst.Chronicle.DB().Close(); err != nil {
		t.Fatal(err)
	}
	source := startMoveProcess(t, src.DataDir, "127.0.0.1:0", true)
	receiver := startMoveProcess(t, dst.DataDir, "127.0.0.1:0", false)
	sourceURL, receiverURL := source.url, receiver.url
	link := processJSON(t, source, http.MethodPost, "/api/v1/circles/"+cid+"/move", owner, nil)["link"].(string)
	id := processJSON(t, receiver, http.MethodPost, "/api/v1/moves/incoming", token, map[string]any{"link": link})["id"].(string)
	processJSON(t, receiver, http.MethodPost, "/api/v1/moves/incoming/"+id+"/start", token, map[string]any{})
	waitProcessCondition(t, "first MiB received", func() bool { _, err := os.Stat(filepath.Join(src.DataDir, "cut.ready")); return err == nil })
	source.stop()
	var committed int64
	waitProcessCondition(t, "partial chunk committed", func() bool {
		m := processJSON(t, receiver, http.MethodGet, "/api/v1/moves/incoming", token, nil)["move"].(map[string]any)
		committed = int64(m["pulled_bytes"].(float64))
		return committed >= 1<<20
	})
	receiver.stop()
	part := filepath.Join(dst.DataDir, "moves", "in-"+id+".wynd")
	if fi, err := os.Stat(part); err != nil || fi.Size() != committed {
		t.Fatalf("persisted prefix: %v %v", fi, err)
	}
	if err := os.WriteFile(filepath.Join(src.DataDir, "ranges.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	source = startMoveProcess(t, src.DataDir, strings.TrimPrefix(sourceURL, "http://"), false)
	receiver = startMoveProcess(t, dst.DataDir, strings.TrimPrefix(receiverURL, "http://"), false)
	var final map[string]any
	waitProcessCondition(t, "restarted transfer complete", func() bool {
		m := processJSON(t, receiver, http.MethodGet, "/api/v1/moves/incoming", token, nil)["move"].(map[string]any)
		if m["status"] == "failed" {
			t.Fatalf("transfer failed: %+v", m)
		}
		final = m
		return m["status"] == "done"
	})
	result := final["result"].(map[string]any)
	if result["verified"] != true || result["posts"] != float64(3) {
		t.Fatalf("verification: %+v", result)
	}
	ranges, err := os.ReadFile(filepath.Join(src.DataDir, "ranges.txt"))
	if err != nil {
		t.Fatal(err)
	}
	first := strings.SplitN(string(ranges), "\n", 2)[0]
	if !strings.HasPrefix(first, fmt.Sprintf("bytes=%d-", committed)) || !strings.Contains(first, ` "`) {
		t.Fatalf("prefix/If-Range lost: %q", first)
	}
	if len(strings.FieldsFunc(string(ranges), func(r rune) bool { return r == '\n' })) < 2 {
		t.Fatalf("40 MiB did not cross chunk boundary: %q", ranges)
	}
	st := processJSON(t, source, http.MethodGet, "/api/v1/circles/"+cid+"/move", owner, nil)
	if st["move"].(map[string]any)["status"] != "accepted" {
		t.Fatalf("source not accepted: %+v", st)
	}
	receiver.stop()
	source.stop()
	db, err := store.Open(filepath.Join(dst.DataDir, "wynd.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	bs, err := blob.New(db, filepath.Join(dst.DataDir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	var importedID string
	if err := bs.DB().QueryRow(`SELECT id FROM blobs WHERE sha256=? AND size_bytes=?`, hex.EncodeToString(sum[:]), len(payload)).Scan(&importedID); err != nil {
		t.Fatal(err)
	}
	info, err := bs.OpenBlob(t.Context(), importedID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(info.Path)
	if err != nil || sha256.Sum256(got) != sum {
		t.Fatal("restored media digest differs", err)
	}
	if _, err := os.Stat(part); !os.IsNotExist(err) {
		t.Fatal("completed ZIP retained", err)
	}
	t.Logf("40 MiB; committed prefix %d; resumed %s; verified 3 posts and media SHA-256", committed, first)
}
