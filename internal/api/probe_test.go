package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
		"photo_max_px":           2048,
		"photo_quality":          80,
		"video_max_height":       1080,
		"video_bitrate_kbps":     6000,
		"attachment_max_bytes":   1 << 20,
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
}
