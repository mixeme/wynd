package api_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIncomingReplacementRemovesZIPAndRange(t *testing.T) {
	src, owner, cid := fullArchiveCircle(t)
	h := httptest.NewServer(src)
	defer h.Close()
	link := startMove(t, src, owner, cid)
	link = h.URL + "/move/" + link[strings.LastIndex(link, "/")+1:]
	dst, caps, _, _ := setupAPI(t)
	allowLocalMoveSource(dst)
	token, _ := registerSession(t, dst, caps, "receiver@example.com")
	r := doJSON(t, dst, http.MethodPost, "/api/v1/moves/incoming", token, map[string]string{"link": link})
	if r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	oldID := jsonStr(t, r, "id")
	zip := filepath.Join(dst.DataDir, "moves", "in-"+oldID+".wynd")
	if err := os.MkdirAll(filepath.Dir(zip), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{zip, zip + ".range"} {
		if err := os.WriteFile(path, []byte("16"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// A draft without a strong archive validator must start afresh, even when
	// the card is the same. Both parts of the obsolete download must disappear.
	r = doJSON(t, dst, http.MethodPost, "/api/v1/moves/incoming", token, map[string]string{"link": link})
	if r.Code != http.StatusOK || jsonStr(t, r, "id") == oldID {
		t.Fatalf("replacement: %d %s", r.Code, r.Body.String())
	}
	for _, path := range []string{zip, zip + ".range"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("obsolete download retained: %s: %v", filepath.Base(path), err)
		}
	}
}
