package desktop

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testDist() fstest.MapFS {
	return fstest.MapFS{
		"index.html":          &fstest.MapFile{Data: []byte("<html><head><title>Wynd</title></head><body></body></html>")},
		"_app/immutable/x.js": &fstest.MapFile{Data: []byte("console.log(1)")},
		"icons.svg":           &fstest.MapFile{Data: []byte("<svg></svg>")},
	}
}

func embeddedFor(dist fstest.MapFS) http.Handler {
	return EmbeddedHandler(dist, func() string { return "https://home.example.org" }, "http://127.0.0.1:41234")
}

func TestEmbeddedHandlerPages(t *testing.T) {
	h := embeddedFor(testDist())

	// Корень — index.html с мостом окна.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("/: %d", rec.Code)
	}
	if !strings.Contains(body, `window.__wyndDesktop={"origin":"https://home.example.org","apiBase":"http://127.0.0.1:41234"}`) {
		t.Fatalf("мост не вставлен: %s", body)
	}
	if !strings.Contains(rec.Header().Get("Cache-Control"), "no-cache") {
		t.Fatalf("кэш index: %q", rec.Header().Get("Cache-Control"))
	}

	// Маршрут клиента без расширения — тоже index.html (SPA).
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/circles/abc/posts/1", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "__wyndDesktop") {
		t.Fatalf("SPA-фолбэк: %d %s", rec.Code, rec.Body.String())
	}

	// Файл отдаётся как есть, без моста.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_app/immutable/x.js", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "console.log(1)" {
		t.Fatalf("файл: %d %q", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "__wyndDesktop") {
		t.Fatal("мост попал в статику")
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("content-type: %q", rec.Header().Get("Content-Type"))
	}

	// Отсутствующий файл с расширением — 404, без расширения — index.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing.js", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("нет файла: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing/route", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("нет маршрута: %d", rec.Code)
	}

	// HEAD отдаёт заголовки без тела.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodHead, "/", nil))
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 || rec.Header().Get("Content-Length") == "" {
		t.Fatalf("HEAD: %d body=%d", rec.Code, rec.Body.Len())
	}
}

// Во вшитом режиме серверные пути остаются серверными, а страницы — из exe.
func TestProxyEmbeddedRouting(t *testing.T) {
	p := NewProxy()
	p.SetAPIBase("http://127.0.0.1:1")
	p.SetEmbeddedUI(embeddedFor(testDist()))
	shell := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("shell")) })
	h := p.Handler(shell)

	// Серверные пути без выбранного сервера — отказ прокси, а не файл клиента.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/instance", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("api: %d %s", rec.Code, rec.Body.String())
	}
	// Страница — из вшитого клиента.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings/app", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "__wyndDesktop") {
		t.Fatalf("страница: %d", rec.Code)
	}
	// Оболочка — по службам путям.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/__shell/", nil))
	if rec.Body.String() != "shell" {
		t.Fatalf("оболочка: %q", rec.Body.String())
	}
	// Iframe просит интерфейс сервера меткой ?wynd=server — этот путь серверный.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?wynd=server", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("iframe сервера: %d %s", rec.Code, rec.Body.String())
	}
}
