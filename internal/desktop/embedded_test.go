package desktop

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testDist() fstest.MapFS {
	return fstest.MapFS{
		// Упоминание имени без моста: клиент так проверяет класс окна (C143).
		// Вставка моста должна смотреть на сам мост, а не на любое упоминание.
		"index.html":          &fstest.MapFile{Data: []byte("<html><head><title>Wynd</title><script>if (window.__wyndDesktop) document.documentElement.classList.add('desktop-window');</script></head><body></body></html>")},
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
	if !strings.Contains(body, `window.__wyndDesktop={"origin":"https://home.example.org","apiBase":"http://127.0.0.1:41234","embedded":true}`) {
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

// Источник интерфейса переключается на ходу: вшитый клиент из exe или
// страницы выбранного сервера. Серверные пути в обоих режимах — за прокси.
func TestProxyEmbeddedRouting(t *testing.T) {
	p := NewProxy()
	p.SetAPIBase("http://127.0.0.1:1")
	p.SetEmbeddedUI(embeddedFor(testDist()))
	p.SetSource(SourceEmbedded)
	shell := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("shell")) })
	h := p.Handler(shell)

	call := func(path string, iframe bool) (int, string) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if iframe {
			req.Header.Set("Sec-Fetch-Dest", "iframe")
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}

	// Корень без рамки — оболочка.
	if code, body := call("/", false); code != http.StatusOK || body != "shell" {
		t.Fatalf("оболочка: %d %q", code, body)
	}
	// Страница вшитого источника — из exe.
	if code, body := call("/settings/app", false); code != http.StatusOK || !strings.Contains(body, "__wyndDesktop") {
		t.Fatalf("страница exe: %d", code)
	}
	// Корень в рамке без метки — текущий источник (вшитый).
	if code, body := call("/", true); code != http.StatusOK || !strings.Contains(body, "__wyndDesktop") {
		t.Fatalf("рамка вшитого: %d", code)
	}
	// Серверные пути остаются серверными; сервера нет — отказ прокси.
	if code, _ := call("/api/v1/instance", false); code != http.StatusServiceUnavailable {
		t.Fatalf("api: %d", code)
	}
	// Оболочка — по служебным путям.
	if code, body := call("/__shell/", false); code != http.StatusOK || body != "shell" {
		t.Fatalf("оболочка: %d %q", code, body)
	}
	// Метка ?wynd=server — явный серверный интерфейс.
	if code, _ := call("/?wynd=server", false); code != http.StatusServiceUnavailable {
		t.Fatalf("метка сервера: %d", code)
	}

	// Серверный источник: страницы и рамка — с сервера (его нет — отказ).
	p.SetSource(SourceServer)
	if code, _ := call("/settings/app", false); code != http.StatusServiceUnavailable {
		t.Fatalf("страница сервера: %d", code)
	}
	if code, _ := call("/", true); code != http.StatusServiceUnavailable {
		t.Fatalf("рамка сервера: %d", code)
	}
	// Явная метка вшитого источника работает и в серверном режиме.
	if code, body := call("/?wynd=embedded", false); code != http.StatusOK || !strings.Contains(body, "__wyndDesktop") {
		t.Fatalf("метка вшитого: %d", code)
	}
	if code, body := call("/__shell/", false); code != http.StatusOK || body != "shell" {
		t.Fatalf("оболочка в серверном режиме: %d %q", code, body)
	}
}

// Служебная метка источника не попадает в адрес клиента.
func TestProxyEmbeddedMarkerStripped(t *testing.T) {
	p := NewProxy()
	p.SetSource(SourceEmbedded)
	p.SetEmbeddedUI(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.URL.RawQuery)
	}))
	h := p.Handler(http.NotFoundHandler())

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?wynd=embedded&x=1", nil))
	if rec.Body.String() != "x=1" {
		t.Fatalf("метка осталась: %q", rec.Body.String())
	}
}
