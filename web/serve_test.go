package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestSPA_fallbackAndAssets(t *testing.T) {
	root := fstest.MapFS{
		"index.html":           &fstest.MapFile{Data: []byte("<html>app</html>")},
		"_app/chunk.js":        &fstest.MapFile{Data: []byte("console.log(1)")},
		"manifest.webmanifest": &fstest.MapFile{Data: []byte(`{"name":"Wynd"}`)},
	}

	h := SPA(root)

	t.Run("existing asset", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/_app/chunk.js", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status: %d", rec.Code)
		}
		if rec.Body.String() != "console.log(1)" {
			t.Fatalf("body: %q", rec.Body.String())
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
			t.Fatalf("cache-control: %q", got)
		}
	})

	t.Run("client route fallback", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/bootstrap", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status: %d", rec.Code)
		}
		if rec.Body.String() != "<html>app</html>" {
			t.Fatalf("body: %q", rec.Body.String())
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
			t.Fatalf("cache-control: %q", got)
		}
	})

	t.Run("webmanifest mime", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status: %d", rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/manifest+json" {
			t.Fatalf("content-type: %q", got)
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
			t.Fatalf("cache-control: %q", got)
		}
	})

	t.Run("missing asset 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/_app/missing.js", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status: %d", rec.Code)
		}
	})
}

var _ fs.FS = fstest.MapFS{}

func TestSPA_securityHeaders(t *testing.T) {
	root := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>app</html>")}}
	req := httptest.NewRequest(http.MethodGet, "/circles/abc", nil)
	rec := httptest.NewRecorder()
	SPA(root).ServeHTTP(rec, req)
	for k, want := range map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Content-Security-Policy": "frame-ancestors 'none'; object-src 'none'; base-uri 'self'",
	} {
		if got := rec.Header().Get(k); got != want {
			t.Fatalf("%s: got %q want %q", k, got, want)
		}
	}
}
