package desktop

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestProxyRoutesShellServerAndAPI(t *testing.T) {
	var gotPath, gotHost, gotOrigin, gotQuery string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotHost, gotOrigin, gotQuery = r.URL.Path, r.Host, r.Header.Get("Origin"), r.URL.RawQuery
		_, _ = io.WriteString(w, "target:"+r.URL.Path)
	}))
	defer target.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("SHELL"), 0o600); err != nil {
		t.Fatal(err)
	}
	shell := http.FileServer(http.Dir(dir))

	proxy := NewProxy()
	if err := proxy.Set(target.URL); err != nil {
		t.Fatal(err)
	}
	handler := proxy.Handler(shell)

	call := func(path string, hdr map[string]string) (int, string) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		body, _ := io.ReadAll(rec.Result().Body)
		return rec.Code, string(body)
	}

	if code, body := call("/", nil); code != 200 || body != "SHELL" {
		t.Fatalf("shell root: %d %q", code, body)
	}
	if code, body := call("/", map[string]string{"Sec-Fetch-Dest": "iframe"}); code != 200 || body != "target:/" {
		t.Fatalf("iframe root: %d %q", code, body)
	}
	if code, body := call("/__server/", nil); code != 200 || body != "target:/" {
		t.Fatalf("server prefix: %d %q", code, body)
	}
	if code, body := call("/?wynd=server", nil); code != 200 || body != "target:/" {
		t.Fatalf("server root marker: %d %q", code, body)
	}
	if gotQuery != "" {
		t.Fatalf("marker leaked to server: %q", gotQuery)
	}
	if code, body := call("/circles?x=1", nil); code != 200 || body != "target:/circles" {
		t.Fatalf("server query: %d %q", code, body)
	}
	if gotQuery != "x=1" {
		t.Fatalf("query not preserved: %q", gotQuery)
	}
	if code, body := call("/__server/circles?x=1", nil); code != 200 || body != "target:/circles" {
		t.Fatalf("server path: %d %q", code, body)
	}
	if code, body := call("/api/v1/instance", map[string]string{"Origin": "http://wails.localhost"}); code != 200 || body != "target:/api/v1/instance" {
		t.Fatalf("api: %d %q", code, body)
	}
	if gotHost != strings.TrimPrefix(target.URL, "http://") {
		t.Fatalf("target host: %q", gotHost)
	}
	if gotOrigin != "" {
		t.Fatalf("Origin leaked to server: %q", gotOrigin)
	}
	if gotPath != "/api/v1/instance" {
		t.Fatalf("target path: %q", gotPath)
	}
}

func TestProxyForeignServerAllowlist(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "foreign:"+r.URL.Path+"?"+r.URL.RawQuery)
	}))
	defer target.Close()

	proxy := NewProxy()
	if err := proxy.SetAllowed([]string{target.URL}); err != nil {
		t.Fatal(err)
	}
	handler := proxy.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))

	host := strings.TrimPrefix(target.URL, "http://")
	call := func(path string) (int, string) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		body, _ := io.ReadAll(rec.Result().Body)
		return rec.Code, string(body)
	}
	if code, body := call("/__srv/http/" + host + "/api/v1/instance?x=1"); code != 200 || body != "foreign:/api/v1/instance?x=1" {
		t.Fatalf("allowed foreign: %d %q", code, body)
	}
	if code, _ := call("/__srv/http/127.0.0.1:1/api/v1/instance"); code != http.StatusForbidden {
		t.Fatalf("foreign not in list: %d", code)
	}
}

func TestProxyAllowsSameOriginFrame(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'; object-src 'none'")
		_, _ = io.WriteString(w, "<html><head><title>x</title></head><body>ok</body></html>")
	}))
	defer target.Close()

	proxy := NewProxy()
	if err := proxy.Set(target.URL); err != nil {
		t.Fatal(err)
	}
	handler := proxy.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Sec-Fetch-Dest", "iframe")
	handler.ServeHTTP(rec, req)
	if got := rec.Header().Get("X-Frame-Options"); got != "" {
		t.Fatalf("X-Frame-Options not removed: %q", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); !strings.Contains(got, "frame-ancestors 'self'") {
		t.Fatalf("CSP not rewritten: %q", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `window.__wyndDesktop={"origin":"`+target.URL+`"}`) {
		t.Fatalf("bridge marker missing: %q", body)
	}
	if rec.Header().Get("Content-Length") != strconv.Itoa(len(body)) {
		t.Fatalf("content length not updated: %q", rec.Header().Get("Content-Length"))
	}
}

func TestProxyWithoutServer(t *testing.T) {
	shell := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "SHELL")
	})
	handler := NewProxy().Handler(shell)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/instance", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("api without server: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != 200 || rec.Body.String() != "SHELL" {
		t.Fatalf("shell without server: %d %q", rec.Code, rec.Body.String())
	}
}
