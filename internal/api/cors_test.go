package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func corsTestServer() *Server {
	s := &Server{Mux: http.NewServeMux()}
	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
	s.Mux.HandleFunc("GET /api/v1/instance", ok)
	s.Mux.HandleFunc("PUT /api/v1/uploads/{id}", ok)
	s.Mux.HandleFunc("GET /api/v1/admin/access", ok)
	s.Mux.HandleFunc("GET /api/v1/archive/{token}", ok)
	s.Mux.HandleFunc("GET /api/v1/full_archive_stream/{token}", ok)
	return s
}

func corsRequest(s *Server, method, path, origin string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func TestParticipantCORSResponse(t *testing.T) {
	rec := corsRequest(corsTestServer(), http.MethodGet, "/api/v1/instance", "https://b.example", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	h := rec.Header()
	if got := h.Get("Access-Control-Allow-Origin"); got != "https://b.example" {
		t.Fatalf("allow-origin: %q", got)
	}
	for _, name := range []string{
		"Retry-After", "Content-Range", "Content-Disposition", "ETag", "Upload-Offset", "Accept-Ranges",
	} {
		if !strings.Contains(h.Get("Access-Control-Expose-Headers"), name) {
			t.Fatalf("expose headers %q missing %s", h.Get("Access-Control-Expose-Headers"), name)
		}
	}
	if got := h.Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("allow-credentials must be empty, got %q", got)
	}
	if got := strings.Join(h.Values("Vary"), ", "); !strings.Contains(got, "Origin") {
		t.Fatalf("vary: %q", got)
	}
}

func TestParticipantCORSPreflight(t *testing.T) {
	rec := corsRequest(corsTestServer(), http.MethodOptions, "/api/v1/uploads/session", "https://b.example", map[string]string{
		"Access-Control-Request-Method":  "PUT",
		"Access-Control-Request-Headers": "authorization,content-type,upload-offset",
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: %d, want 204", rec.Code)
	}
	h := rec.Header()
	if got := h.Get("Access-Control-Allow-Origin"); got != "https://b.example" {
		t.Fatalf("allow-origin: %q", got)
	}
	if !strings.Contains(h.Get("Access-Control-Allow-Methods"), "PUT") {
		t.Fatalf("allow-methods: %q", h.Get("Access-Control-Allow-Methods"))
	}
	for _, name := range []string{
		"Authorization", "Content-Type", "Range", "Content-Range", "Upload-Offset", "If-Range",
		"If-None-Match", "If-Modified-Since",
	} {
		if !strings.Contains(h.Get("Access-Control-Allow-Headers"), name) {
			t.Fatalf("allow-headers %q missing %s", h.Get("Access-Control-Allow-Headers"), name)
		}
	}
	if got := h.Get("Access-Control-Max-Age"); got != "600" {
		t.Fatalf("max-age: %q", got)
	}
	if got := h.Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("allow-credentials must be empty, got %q", got)
	}
}

func TestParticipantCORSKeepsAdminAndLinksClosed(t *testing.T) {
	s := corsTestServer()
	for _, path := range []string{
		"/api/v1/admin/access",
		"/api/v1/archive/token",
		"/api/v1/full_archive_stream/token",
	} {
		rec := corsRequest(s, http.MethodGet, path, "https://b.example", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status: %d", path, rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("%s allow-origin: %q, want empty", path, got)
		}
	}
	rec := corsRequest(s, http.MethodOptions, "/api/v1/admin/access", "https://b.example", map[string]string{
		"Access-Control-Request-Method": "GET",
	})
	if rec.Code == http.StatusNoContent {
		t.Fatal("admin preflight must not be answered as CORS")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("admin preflight allow-origin: %q, want empty", got)
	}
}

func TestParticipantCORSRejectsNonOrigins(t *testing.T) {
	for _, origin := range []string{
		"null",
		"file://host",
		"wails.localhost",
		"https://a.example/path",
		"https://a.example/",
		"https://user@a.example",
		"https://a.example?x",
		"https://a.example#x",
	} {
		rec := corsRequest(corsTestServer(), http.MethodGet, "/api/v1/instance", origin, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%q status: %d", origin, rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("%q allow-origin: %q, want empty", origin, got)
		}
	}
}

func TestParticipantCORSWithoutOrigin(t *testing.T) {
	rec := corsRequest(corsTestServer(), http.MethodGet, "/api/v1/instance", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("allow-origin: %q, want empty", got)
	}
	if got := strings.Join(rec.Header().Values("Vary"), ", "); !strings.Contains(got, "Origin") {
		t.Fatalf("vary: %q", got)
	}
}

func TestParticipantCORSIgnoresPlainOptions(t *testing.T) {
	rec := corsRequest(corsTestServer(), http.MethodOptions, "/api/v1/instance", "https://b.example", nil)
	if rec.Code == http.StatusNoContent {
		t.Fatal("plain OPTIONS without a preflight request must not be answered by CORS")
	}
}
