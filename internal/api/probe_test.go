package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbeEndpoints(t *testing.T) {
	srv, _, _, _ := setupAPI(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/probe", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("probe: %d %s", rec.Code, rec.Body.String())
	}
	var probe map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &probe); err != nil {
		t.Fatal(err)
	}
	if probe["body_probe_bytes"] != float64(2<<20) {
		t.Fatalf("body_probe_bytes: %v", probe["body_probe_bytes"])
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/probe/sse", nil)
	req.Header.Set("Accept", "text/event-stream")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("probe sse: %d %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("event: probe")) {
		t.Fatalf("probe sse body: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/probe/stream", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("probe stream: %d %s", rec.Code, rec.Body.String())
	}
	if len(rec.Body.Bytes()) < 20 {
		t.Fatalf("probe stream short: %d bytes", len(rec.Body.Bytes()))
	}

	body := bytes.Repeat([]byte("x"), 2<<20)
	req = httptest.NewRequest(http.MethodPut, "/api/v1/probe/body", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("probe body: %d %s", rec.Code, rec.Body.String())
	}
}

func TestProbeBodyTooLarge(t *testing.T) {
	srv, _, _, _ := setupAPI(t)
	token := adminToken(t, srv)
	rec := doJSON(t, srv, http.MethodPut, "/api/v1/admin/compression", token, map[string]any{
		"photo_max_px":         2048,
		"photo_quality":        80,
		"video_max_height":     1080,
		"video_bitrate_kbps":   6000,
		"attachment_max_bytes": 1 << 20,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("compression: %d %s", rec.Code, rec.Body.String())
	}

	body := bytes.Repeat([]byte("x"), 2<<20)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/probe/body", io.NopCloser(bytes.NewReader(body)))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("probe body limit: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/probe", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var capped map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &capped); err != nil {
		t.Fatal(err)
	}
	if capped["body_probe_bytes"] != float64(1<<20) {
		t.Fatalf("body_probe_bytes after compression: %v", capped["body_probe_bytes"])
	}

	// Потолок пробы — объявленный body_probe_bytes, а не attachment_max
	// (аудит 2026-09-22): при потолке вложений 100 МиБ проба принимает 2 МиБ.
	rec = doJSON(t, srv, http.MethodPut, "/api/v1/admin/compression", token, map[string]any{
		"photo_max_px": 2048, "photo_quality": 80, "video_max_height": 1080,
		"video_bitrate_kbps": 6000, "attachment_max_bytes": 100 << 20,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("compression: %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPut, "/api/v1/probe/body", io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("x"), 3<<20))))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("probe body above 2 MiB accepted: %d", rec.Code)
	}
}

// Инвариант (DEP-5): тяжёлые публичные пробы ограничены по адресу.
// /probe/body анонимно принимает мегабайты, /probe/stream держит соединение.
func TestProbeBodyAndStreamRateLimited(t *testing.T) {
	srv, _, _, _ := setupAPI(t)
	limited := false
	for i := 0; i < probeLimitProbes; i++ {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/probe/body", strings.NewReader("x"))
		req.Header.Set("X-Forwarded-For", "203.0.113.70")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			limited = true
			break
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("проба %d: %d %s", i+1, rec.Code, rec.Body.String())
		}
	}
	if !limited {
		t.Fatalf("/probe/body не упёрся в лимит за %d запросов", probeLimitProbes)
	}

	// Другой адрес лимитом не задет.
	req := httptest.NewRequest(http.MethodPut, "/api/v1/probe/body", strings.NewReader("x"))
	req.Header.Set("X-Forwarded-For", "203.0.113.71")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("чужой адрес задет лимитом: %d", rec.Code)
	}
}

const probeLimitProbes = 30
