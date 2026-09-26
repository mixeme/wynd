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

	h := SPA(root, true)

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

	// Каталог не листается (API-6, аудит 2026-09-22).
	t.Run("directory is not listed", func(t *testing.T) {
		for _, p := range []string{"/_app/", "/_app"} {
			req := httptest.NewRequest(http.MethodGet, p, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("%s: status %d body %q", p, rec.Code, rec.Body.String())
			}
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

// Инвариант (план 42, SW-2): вне loopback хешированные чанки кэшируются на
// год, а оболочка, service worker и нехешированные файлы `_app/` —
// перепроверяются; на loopback перепроверяется всё.
func TestSPA_cachePolicyOutsideLoopback(t *testing.T) {
	root := fstest.MapFS{
		"index.html":                  &fstest.MapFile{Data: []byte("<html>app</html>")},
		"sw.js":                       &fstest.MapFile{Data: []byte("//sw")},
		"push-sw.js":                  &fstest.MapFile{Data: []byte("//push")},
		"_app/version.json":           &fstest.MapFile{Data: []byte(`{"version":"1"}`)},
		"_app/immutable/chunk.abc.js": &fstest.MapFile{Data: []byte("x")},
	}
	for _, tc := range []struct {
		loopback bool
		path     string
		want     string
	}{
		{false, "/_app/immutable/chunk.abc.js", "public, max-age=31536000, immutable"},
		{false, "/_app/version.json", "no-cache"},
		{false, "/sw.js", "no-cache"},
		{false, "/push-sw.js", "no-cache"},
		{false, "/index.html", "no-cache"},
		{true, "/_app/immutable/chunk.abc.js", "no-cache"},
	} {
		rec := httptest.NewRecorder()
		SPA(root, tc.loopback).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if got := rec.Header().Get("Cache-Control"); got != tc.want {
			t.Fatalf("loopback=%v %s: %q, want %q", tc.loopback, tc.path, got, tc.want)
		}
	}
}

func TestSPA_securityHeaders(t *testing.T) {
	root := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>app</html>")}}
	req := httptest.NewRequest(http.MethodGet, "/circles/abc", nil)
	rec := httptest.NewRecorder()
	SPA(root, false).ServeHTTP(rec, req)
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
