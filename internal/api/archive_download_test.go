package api_test

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/api"
)

// Инвариант (план 42, ARC-1, ARC-2): архив отдаётся с длиной и поддержкой
// Range — обрыв на гигабайтах докачивается, а не собирается заново с нуля;
// повторная сборка даёт те же байты. Медиа лежат в ZIP без сжатия, HTML —
// сжат. После ответа во временном каталоге ничего не остаётся.
func TestArchiveDownloadRangeAndStoredMedia(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	tok, _ := registerSession(t, srv, caps, "range@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", tok, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")
	photo := bytes.Repeat([]byte("jpeg-bytes-"), 64)
	photoID := uploadBytes(t, srv, tok, photo, "image/jpeg")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", tok, map[string]any{
		"body": "с фото", "entry_date": "2026-08-01",
		"media": []map[string]any{{"blob_id": photoID, "kind": "photo", "is_cover": true}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("post: %d %s", rec.Code, rec.Body.String())
	}
	now := time.Now().UTC()
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/archive", tok, map[string]any{
		"cutoff_date":         now.Add(24 * time.Hour).Format("2006-01-02"),
		"deadline":            now.Add(48 * time.Hour).Format(time.RFC3339),
		"reminder_before_sec": 3600,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("start archive: %d %s", rec.Code, rec.Body.String())
	}

	full := downloadArchive(t, srv, tok, circleID)
	zr, err := zip.NewReader(bytes.NewReader(full), int64(len(full)))
	if err != nil {
		t.Fatal(err)
	}
	var sawMedia bool
	for _, f := range zr.File {
		switch {
		case strings.HasPrefix(f.Name, "media/"):
			sawMedia = true
			if f.Method != zip.Store {
				t.Fatalf("%s: method %d, want Store", f.Name, f.Method)
			}
		case strings.HasSuffix(f.Name, ".html"):
			if f.Method != zip.Deflate {
				t.Fatalf("%s: method %d, want Deflate", f.Name, f.Method)
			}
		}
	}
	if !sawMedia {
		t.Fatal("архив без медиа")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/circles/"+circleID+"/archive/download", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Range", "bytes=10-")
	part := httptest.NewRecorder()
	srv.ServeHTTP(part, req)
	if part.Code != http.StatusPartialContent {
		t.Fatalf("range: %d %s", part.Code, part.Body.String())
	}
	if !bytes.Equal(part.Body.Bytes(), full[10:]) {
		t.Fatal("докачка не совпала с хвостом полного архива: сборка недетерминирована")
	}
	if got := part.Header().Get("Content-Length"); got != strconv.Itoa(len(full)-10) {
		t.Fatalf("Content-Length = %q, want %d", got, len(full)-10)
	}

	entries, err := os.ReadDir(api.ArchiveTempDir(srv.DataDir))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("во временном каталоге остались файлы: %v", entries)
	}
}
