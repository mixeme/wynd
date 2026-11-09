//go:build dev || desktop || bindings

// Окно Wynd для компьютера (планы 55 и 61). Собирается через `wails build`
// или `wails dev`: без их меток сборки пакет пуст, и ворота основного сервера
// (go vet, go test ./...) его не трогают.
package main

import (
	"context"
	"embed"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"gitea.mixdep.ru/mix/wynd/internal/desktop"
)

//go:embed all:frontend/dist
var assets embed.FS

// App — то, что окно может попросить. Методы видны как window.go.main.App.*
// Список серверов — в internal/desktop; токенов и паролей здесь нет.
type App struct {
	mu       sync.Mutex
	ctx      context.Context
	store    *desktop.Store
	client   *http.Client
	storeErr error
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// CheckServer проверяет адрес и запоминает сервер выбранным. Ошибка одна:
// причина (опечатка, сеть, чужой сайт) человеку не различается.
func (a *App) CheckServer(address string) (desktop.CheckResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.storeErr != nil {
		return desktop.CheckResult{}, a.storeErr
	}
	res, err := desktop.CheckServer(a.ctx, a.client, address)
	if err != nil {
		return desktop.CheckResult{}, err
	}
	server := desktop.Server{
		Origin:           res.Origin,
		Name:             res.Name,
		Version:          res.Version,
		RegistrationMode: res.RegistrationMode,
	}
	if err := a.store.Put(server, true); err != nil {
		return desktop.CheckResult{}, err
	}
	return res, nil
}

// CurrentServer — выбранный сервер при следующем запуске.
func (a *App) CurrentServer() *desktop.CheckResult {
	if a.store == nil {
		return nil
	}
	selected, servers := a.store.Snapshot()
	for _, server := range servers {
		if server.Origin == selected {
			return &desktop.CheckResult{
				Origin:           server.Origin,
				Name:             server.Name,
				Version:          server.Version,
				RegistrationMode: server.RegistrationMode,
			}
		}
	}
	return nil
}

// ListServers — добавленные серверы для «Настроек → Серверы» (ПК7).
func (a *App) ListServers() []desktop.Server {
	if a.store == nil {
		return nil
	}
	_, servers := a.store.Snapshot()
	if servers == nil {
		return []desktop.Server{}
	}
	return servers
}

func main() {
	config, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	base := filepath.Join(config, "Wynd", "desktop")
	store, err := desktop.OpenStore(filepath.Join(base, "servers.json"))
	if err != nil {
		log.Fatal(err)
	}
	app := &App{
		store:  store,
		client: &http.Client{Timeout: 12 * time.Second},
	}
	err = wails.Run(&options.App{
		Title:       "Wynd",
		Width:       1000,
		Height:      660,
		MinWidth:    720,
		MinHeight:   480,
		AssetServer: &assetserver.Options{Assets: assets},
		// Цвет бумаги (--paper): окно не мигает белым, пока грузится страница.
		BackgroundColour: &options.RGBA{R: 0xF4, G: 0xF0, B: 0xE9, A: 0xFF},
		// Своя рамка (TitleBar), как на утверждённых кадрах; системный
		// заголовок Windows 10 в цвет бумаги не перекрашивается.
		Frameless: true,
		Windows: &windows.Options{
			// Постоянный профиль: сессия и медиа-кэш переживают перезапуск,
			// данные Wynd из системного профиля не используются.
			WebviewUserDataPath: filepath.Join(base, "webview"),
		},
		OnStartup: app.startup,
		Bind:      []any{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
