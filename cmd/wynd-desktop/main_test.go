//go:build dev || desktop || bindings

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/desktop"
)

// Иконку и описание exe Wails вшивает из build. Без этих файлов сборка
// выходит с чужой иконкой — так выпуск 0.2.0 остался без своей.
func TestDesktopBuildAssets(t *testing.T) {
	files := []string{
		filepath.Join("build", "appicon.png"),
		filepath.Join("build", "windows", "icon.ico"),
		filepath.Join("build", "windows", "info.json"),
		filepath.Join("build", "windows", "wails.exe.manifest"),
	}
	for _, name := range files {
		if _, err := os.Stat(name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// Локальный порт API открыт странице окна (wails.localhost) и `wails dev`
// (localhost). Чужой сайт, перебравший порты 127.0.0.1, ответы читать не
// должен: без своего origin в Access-Control-Allow-Origin браузер их отбросит.
func TestCORSOnlyForWindowOrigins(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	cases := []struct {
		origin string
		want   bool
	}{
		{"http://wails.localhost", true},
		{"wails://localhost", true},
		{"http://localhost:34115", true},
		{"http://127.0.0.1:9000", true},
		{"http://[::1]:9000", true},
		{"https://evil.example", false},
		{"null", false},
		{"", false},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/instance", nil)
		if c.origin != "" {
			r.Header.Set("Origin", c.origin)
		}
		w := httptest.NewRecorder()
		corsHandler(next).ServeHTTP(w, r)
		if got := w.Header().Get("Access-Control-Allow-Origin"); (got != "") != c.want {
			t.Errorf("origin %q: Access-Control-Allow-Origin = %q, want present %v", c.origin, got, c.want)
		}
	}

	// Загрузка вложения: странице нужно прислать Upload-Offset и прочитать
	// его из ответа, иначе браузер блокирует PUT (C66).
	r := httptest.NewRequest(http.MethodOptions, "/api/v1/uploads/x", nil)
	r.Header.Set("Origin", "http://wails.localhost")
	w := httptest.NewRecorder()
	corsHandler(next).ServeHTTP(w, r)
	if !strings.Contains(w.Header().Get("Access-Control-Allow-Headers"), "Upload-Offset") {
		t.Fatalf("preflight must allow Upload-Offset: %q", w.Header().Get("Access-Control-Allow-Headers"))
	}
	got := httptest.NewRecorder()
	corsHandler(next).ServeHTTP(got, httptest.NewRequest(http.MethodHead, "/api/v1/uploads/x", nil))
	_ = got
	r = httptest.NewRequest(http.MethodGet, "/api/v1/uploads/x", nil)
	r.Header.Set("Origin", "http://wails.localhost")
	w = httptest.NewRecorder()
	corsHandler(next).ServeHTTP(w, r)
	if !strings.Contains(w.Header().Get("Access-Control-Expose-Headers"), "Upload-Offset") {
		t.Fatalf("window origin must read Upload-Offset: %q", w.Header().Get("Access-Control-Expose-Headers"))
	}

	// Preflight чужого сайта не получает разрешения и не доходит до прокси.
	r = httptest.NewRequest(http.MethodOptions, "/api/v1/instance", nil)
	r.Header.Set("Origin", "https://evil.example")
	w = httptest.NewRecorder()
	corsHandler(next).ServeHTTP(w, r)
	if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("foreign preflight: status %d, allow-origin %q", w.Code, w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestInterfacePreviewAddAndRemoveDoNotChangeSelection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/instance" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"name":              "Файлы",
			"version":           "0.51.1",
			"registration_mode": "closed",
		})
	}))
	defer srv.Close()

	store, err := desktop.OpenStore(filepath.Join(t.TempDir(), "servers.json"))
	if err != nil {
		t.Fatal(err)
	}
	current := desktop.Server{Origin: "https://home.example", Name: "Дом Ани", Interface: true}
	if err := store.Put(current, true); err != nil {
		t.Fatal(err)
	}
	app := &App{
		ctx:    context.Background(),
		store:  store,
		proxy:  desktop.NewProxy(),
		client: srv.Client(),
	}

	preview, err := app.PreviewServer(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Name != "Файлы" || len(app.ListServers()) != 1 {
		t.Fatalf("preview persisted or changed: %+v, %+v", preview, app.ListServers())
	}
	if _, err := app.AddInterface(srv.URL); err != nil {
		t.Fatal(err)
	}
	selected, _ := store.Snapshot()
	if selected != current.Origin || len(app.ListServers()) != 2 {
		t.Fatalf("add changed selection: %q, %+v", selected, app.ListServers())
	}
	if err := app.RemoveInterface(current.Origin); !errors.Is(err, desktop.ErrSelectedServer) {
		t.Fatalf("remove selected: %v", err)
	}
	if err := app.RemoveInterface(srv.URL); err != nil {
		t.Fatal(err)
	}
	selected, _ = store.Snapshot()
	if selected != current.Origin || len(app.ListServers()) != 1 {
		t.Fatalf("remove changed selection: %q, %+v", selected, app.ListServers())
	}
}
