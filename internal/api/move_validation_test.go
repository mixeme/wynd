package api_test

import (
	"archive/zip"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/api"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Run the actual background worker in another process: a panic escaping it
// would kill the child rather than being hidden by the test's own recover.
func TestMoveMalformedArchiveWorkerSurvives(t *testing.T) {
	if os.Getenv("WYND_TEST_MALFORMED_MOVE_CHILD") != "1" {
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(t.Context(), exe, "-test.run=^TestMoveMalformedArchiveWorkerSurvives$", "-test.v")
		cmd.Env = append(os.Environ(), "WYND_TEST_MALFORMED_MOVE_CHILD=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("worker process failed: %v\n%s", err, out)
		}
		return
	}
	srv, caps, _, _ := setupAPI(t)
	_, account := registerSession(t, srv, caps, "receiver@example.com")
	m, err := srv.Chronicle.PutIncomingDraft(t.Context(), account, "https://source.example/move/AAAA-BBBB", chronicle.IncomingCard{CircleName: "source", ExpiresAt: time.Now().Add(time.Hour)}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.Chronicle.StartIncoming(t.Context(), m.ID, false, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := srv.Chronicle.SetIncomingStatus(t.Context(), m.ID, chronicle.IncomingReviving, ""); err != nil {
		t.Fatal(err)
	}
	dir := api.MoveDir(srv.DataDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, "in-"+m.ID+".wynd"))
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for name, value := range map[string]any{
		"manifest.json":   map[string]any{"format": "wynd-circle", "format_version": 2, "schema_version": 15, "tables": []string{"blobs"}},
		"data/blobs.json": map[string]any{"columns": []string{"id", "sha256", "size_bytes"}, "rows": [][]any{{"short"}}},
	} {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.NewEncoder(w).Encode(value); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	srv.ResumeIncomingMoves(t.Context())
	deadline := time.Now().Add(5 * time.Second)
	for {
		m, err = srv.Chronicle.GetIncoming(t.Context(), m.ID)
		if err != nil {
			t.Fatal(err)
		}
		if m.Status == chronicle.IncomingFailed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker did not fail safely")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if m.Error != "archive_invalid" {
		t.Fatalf("error=%s", m.Error)
	}
	var circles, blobs int
	if err := srv.Chronicle.DB().QueryRow(`SELECT (SELECT COUNT(*) FROM circles), (SELECT COUNT(*) FROM blobs)`).Scan(&circles, &blobs); err != nil {
		t.Fatal(err)
	}
	if circles != 0 || blobs != 0 {
		t.Fatalf("import side effects: %d circles, %d blobs", circles, blobs)
	}
}
