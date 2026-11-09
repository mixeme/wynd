//go:build dev || desktop || bindings

// Окно Wynd для компьютера (планы 55 и 61). Собирается через `wails build`
// или `wails dev`: без их меток сборки пакет пуст, и ворота основного сервера
// (go vet, go test ./...) его не трогают.
package main

import (
	"context"
	"embed"
	"io/fs"
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
	proxy    *desktop.Proxy
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
	if err := a.proxy.Set(res.Origin); err != nil {
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

// CheckCurrent проверяет выбранный сервер при запуске и после «Повторить»
// (ПК3). Имя и режим обновляются; адрес не меняется.
func (a *App) CheckCurrent() (desktop.CheckResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.storeErr != nil {
		return desktop.CheckResult{}, a.storeErr
	}
	selected, servers := a.store.Snapshot()
	if selected == "" {
		return desktop.CheckResult{}, desktop.ErrUnknownServer
	}
	var saved desktop.Server
	for _, server := range servers {
		if server.Origin == selected {
			saved = server
			break
		}
	}
	res, err := desktop.CheckServer(a.ctx, a.client, selected)
	if err != nil {
		return desktop.CheckResult{}, err
	}
	saved.Name, saved.Version, saved.RegistrationMode = res.Name, res.Version, res.RegistrationMode
	if err := a.store.Put(saved, false); err != nil {
		return desktop.CheckResult{}, err
	}
	if err := a.proxy.Set(res.Origin); err != nil {
		return desktop.CheckResult{}, err
	}
	return res, nil
}

// ForgetSelected — «Другой сервер» на ПК3: снять выбор, оставить серверы.
func (a *App) ForgetSelected() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.store == nil {
		return nil
	}
	if err := a.store.ClearSelection(); err != nil {
		return err
	}
	return a.proxy.Set("")
}

// Autostart — запись текущего пользователя в Run (ПК5).
func (a *App) Autostart() (desktop.AutostartStatus, error) {
	return desktop.AutostartState()
}

// SetAutostart включает или выключает автозапуск и возвращает прочитанное
// состояние; при отказе окно возвращает прежнее положение переключателя.
func (a *App) SetAutostart(enabled bool) (desktop.AutostartStatus, error) {
	return desktop.SetAutostart(enabled)
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
	proxy := desktop.NewProxy()
	if selected, servers := store.Snapshot(); selected != "" {
		for _, server := range servers {
			if server.Origin == selected {
				if err := proxy.Set(server.Origin); err != nil {
					log.Fatal(err)
				}
				break
			}
		}
	}
	shellFS, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}
	app := &App{
		store:  store,
		proxy:  proxy,
		client: &http.Client{Timeout: 12 * time.Second},
	}
	err = wails.Run(&options.App{
		Title:       "Wynd",
		Width:       1000,
		Height:      660,
		MinWidth:    720,
		MinHeight:   480,
		AssetServer: &assetserver.Options{Handler: proxy.Handler(http.FileServer(http.FS(shellFS)))},
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
