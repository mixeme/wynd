package api_test

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Инвариант (план 42, ARC-6): название дня — сказанное, purge стирает его
// вместе с записями до отсечки, поэтому оно попадает в архив в обеих
// раскладках, экранированным. Название дня без записей в срезе — нет.
func TestArchiveIncludesDayTitles(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	tok, _ := registerSession(t, srv, caps, "titles@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", tok, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", tok, map[string]any{
		"body": "на даче", "entry_date": "2026-08-01",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("post: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+circleID+"/days/2026-08-01/title", tok, map[string]any{
		"title": "Дача <script>",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("title: %d %s", rec.Code, rec.Body.String())
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

	const want = "2026-08-01 · «Дача &lt;script&gt;»"
	for _, layout := range []string{"", "?layout=posts"} {
		r := doGET(t, srv, "/api/v1/circles/"+circleID+"/archive/download"+layout, tok)
		if r.Code != http.StatusOK {
			t.Fatalf("download%s: %d %s", layout, r.Code, r.Body.String())
		}
		html := zipHTML(t, r.Body.Bytes())
		if !strings.Contains(html, want) {
			t.Fatalf("download%s: нет названия дня %q:\n%s", layout, want, html)
		}
		if strings.Contains(html, "<script>") {
			t.Fatalf("download%s: название дня не экранировано", layout)
		}
	}
}

// zipHTML склеивает все HTML-файлы архива.
func zipHTML(t *testing.T, data []byte) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".html") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		b.Write(body)
	}
	return b.String()
}
