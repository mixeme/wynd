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

// Инвариант (план 42, ARC-1, ARC-2; план 46, C38): архив участника идёт
// потоком с точной длиной заранее — браузер видит ход; повторная сборка даёт
// те же байты. Range не поддерживается: оборванное начинается заново (решение
// владельца 2026-10-14). Медиа лежат в ZIP без сжатия, HTML — сжат. На диске
// сервера архив не лежит.
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
	// Обе раскладки сразу (6.37): лента и страницы по записям с оглавлением.
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	if !names["index.html"] || !names["posts/index.html"] {
		t.Fatalf("нет одной из раскладок: %v", names)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/circles/"+circleID+"/archive/download", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Range", "bytes=10-")
	again := httptest.NewRecorder()
	srv.ServeHTTP(again, req)
	if again.Code != http.StatusOK {
		t.Fatalf("повтор с Range: %d, ждали весь архив заново", again.Code)
	}
	if !bytes.Equal(again.Body.Bytes(), full) {
		t.Fatal("повторная сборка дала другие байты: сборка недетерминирована")
	}
	if got := again.Header().Get("Content-Length"); got != strconv.Itoa(len(full)) {
		t.Fatalf("Content-Length = %q, а пришло %d байт", got, len(full))
	}

	entries, err := os.ReadDir(api.ArchiveTempDir(srv.DataDir))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("во временном каталоге остались файлы: %v", entries)
	}
}

// Инвариант (C38): одноразовая ссылка отдаёт тот же архив без заголовка
// входа, работает один раз, и выдаётся только участнику круга.
func TestArchiveOneTimeLink(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	tok, _ := registerSession(t, srv, caps, "link@example.com")
	stranger, _ := registerSession(t, srv, caps, "stranger@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", tok, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")
	startArchive(t, srv, tok, circleID)

	if rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/archive/link", stranger, nil); rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
		t.Fatalf("ссылка чужому: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/archive/link", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("link: %d %s", rec.Code, rec.Body.String())
	}
	url := jsonStr(t, rec, "url")
	byLink := doGET(t, srv, url, "")
	if byLink.Code != http.StatusOK || byLink.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("по ссылке: %d %s", byLink.Code, byLink.Header().Get("Content-Type"))
	}
	if !bytes.Equal(byLink.Body.Bytes(), downloadArchive(t, srv, tok, circleID)) {
		t.Fatal("по ссылке пришёл другой архив")
	}
	if again := doGET(t, srv, url, ""); again.Code != http.StatusNotFound {
		t.Fatalf("второй раз по той же ссылке: %d", again.Code)
	}
}

// Инвариант (C39): звук и видео лежат в архиве со своим расширением —
// с диска браузер узнаёт тип только по нему, и «.bin» не играл.
func TestArchiveKeepsMediaExtensions(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	tok, _ := registerSession(t, srv, caps, "ext@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", tok, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	circleID := jsonStr(t, rec, "id")
	voice := uploadBytes(t, srv, tok, bytes.Repeat([]byte("webm"), 32), "audio/webm")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", tok, map[string]any{
		"body": "голосовое", "entry_date": "2026-08-01",
		"media": []map[string]any{{"blob_id": voice, "kind": "attachment"}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("post: %d %s", rec.Code, rec.Body.String())
	}
	startArchive(t, srv, tok, circleID)
	data := downloadArchive(t, srv, tok, circleID)
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	want := "media/" + voice + ".webm"
	var found bool
	var index string
	for _, f := range zr.File {
		if f.Name == want {
			found = true
		}
		if f.Name == "index.html" {
			rc, _ := f.Open()
			b := new(bytes.Buffer)
			_, _ = b.ReadFrom(rc)
			_ = rc.Close()
			index = b.String()
		}
	}
	if !found {
		t.Fatalf("в архиве нет %s", want)
	}
	if !strings.Contains(index, `<audio controls src="`+want+`"`) {
		t.Fatalf("плеер не на том файле:\n%s", index)
	}
}
