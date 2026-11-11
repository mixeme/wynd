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
