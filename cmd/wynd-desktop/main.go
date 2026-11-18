//go:build dev || desktop || bindings

// Окно Wynd для компьютера (планы 55 и 61). Собирается через
// scripts\build-desktop.bat (`wails3 generate syso` + `go build`) или
// `wails3 dev`: без их меток сборки пакет пуст, и ворота основного сервера
// (go vet, go test ./...) его не трогают.
package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"gitea.mixdep.ru/mix/wynd/internal/desktop"
)

//go:embed all:frontend/dist
var assets embed.FS

// App — то, что окно может попросить (main.App.* в биндингах). Список
// серверов — в internal/desktop; токенов и паролей здесь нет.
type App struct {
	mu       sync.Mutex
	ctx      context.Context
	store    *desktop.Store
	proxy    *desktop.Proxy
	client   *http.Client
	storeErr error
	// update — последняя найденная новая версия окна.
	update desktop.Update
}

// ServiceStartup — хук Wails v3: приложение передаёт контекст, который
// отменяется при закрытии окна.
func (a *App) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	a.ctx = ctx
	desktop.CleanOldSelf()
	return nil
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
		Interface:        true,
	}
	if err := a.store.Put(server, true); err != nil {
		return desktop.CheckResult{}, err
	}
	if err := a.updateAllowed(); err != nil {
		return desktop.CheckResult{}, err
	}
	if err := a.proxy.Set(res.Origin); err != nil {
		return desktop.CheckResult{}, err
	}
	return res, nil
}

func (a *App) updateAllowed() error {
	if a.store == nil {
		return nil
	}
	_, servers := a.store.Snapshot()
	origins := make([]string, 0, len(servers))
	for _, server := range servers {
		origins = append(origins, server.Origin)
	}
	return a.proxy.SetAllowed(origins)
}

// AllowServer добавляет адрес в разрешённые до входа: интерфейс сервера
// проверяет его через /__srv ещё до кода из письма. Источником он не становится.
func (a *App) AllowServer(address string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.storeErr != nil {
		return "", a.storeErr
	}
	origin, err := desktop.NormalizeAddress(address)
	if err != nil {
		return "", err
	}
	server := desktop.Server{Origin: origin, Name: desktop.HostOnly(origin), Allowed: true}
	if err := a.store.Put(server, false); err != nil {
		return "", err
	}
	if err := a.updateAllowed(); err != nil {
		return "", err
	}
	return origin, nil
}

// AddServer сохраняет имя и версию после входа на другой сервер.
func (a *App) AddServer(origin, name, version, mode string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.storeErr != nil {
		return a.storeErr
	}
	normalized, err := desktop.NormalizeAddress(origin)
	if err != nil {
		return err
	}
	server := desktop.Server{Origin: normalized, Name: name, Version: version, RegistrationMode: mode, Account: true}
	if err := a.store.Put(server, false); err != nil {
		return err
	}
	return a.updateAllowed()
}

// PreviewServer проверяет адрес без сохранения и переключения интерфейса.
func (a *App) PreviewServer(address string) (desktop.CheckResult, error) {
	return desktop.CheckServer(a.ctx, a.client, address)
}

// AddInterface добавляет проверенный источник, не меняя выбранный сервер.
func (a *App) AddInterface(address string) (desktop.CheckResult, error) {
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
		Interface:        true,
	}
	if err := a.store.Put(server, false); err != nil {
		return desktop.CheckResult{}, err
	}
	if err := a.updateAllowed(); err != nil {
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

// SelectServer переключает окно на другой источник интерфейса.
func (a *App) SelectServer(origin string) (desktop.Server, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.storeErr != nil {
		return desktop.Server{}, a.storeErr
	}
	if err := a.store.Select(origin); err != nil {
		return desktop.Server{}, err
	}
	if err := a.proxy.Set(origin); err != nil {
		return desktop.Server{}, err
	}
	_, servers := a.store.Snapshot()
	for _, server := range servers {
		if server.Origin == origin {
			return server, nil
		}
	}
	return desktop.Server{}, desktop.ErrUnknownServer
}

// RemoveInterface убирает неактивный источник после подтверждения в оболочке.
func (a *App) RemoveInterface(origin string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.storeErr != nil {
		return a.storeErr
	}
	if err := a.store.RemoveInterface(origin); err != nil {
		return err
	}
	return a.updateAllowed()
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

// ListServers — только источники интерфейса, без адресов одних учётных записей.
func (a *App) ListServers() []desktop.Server {
	if a.store == nil {
		return []desktop.Server{}
	}
	_, servers := a.store.Snapshot()
	interfaces := make([]desktop.Server, 0, len(servers))
	for _, server := range servers {
		if server.Interface {
			interfaces = append(interfaces, server)
		}
	}
	return interfaces
}

// corsHandler — локальный порт API открыт странице окна (wails.localhost):
// запросы с токеном идут по обычному TCP, поэтому нужен CORS и preflight.
// Version — версия окна для заголовка, как у Мастерской.
func (a *App) Version() string { return desktop.Version }

// CheckSelfUpdate — есть ли новый Wynd Настольный: не молча и не сразу,
// как у Мастерской (internal/desktop/selfupdate.go).
func (a *App) CheckSelfUpdate() desktop.Update {
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	update := desktop.CheckUpdate(ctx, http.DefaultClient, desktop.Version, time.Now())
	a.mu.Lock()
	a.update = update
	a.mu.Unlock()
	return update
}

// ApplySelfUpdate — «Обновить и перезапустить»: подпись не сошлась —
// "signature"; вышло — новая версия запускается, эта закрывается.
func (a *App) ApplySelfUpdate() string {
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
	defer cancel()
	a.mu.Lock()
	update := a.update
	a.mu.Unlock()
	path, err := desktop.ApplyUpdate(ctx, http.DefaultClient, update)
	if errors.Is(err, desktop.ErrBadSignature) {
		return "signature"
	}
	if err != nil {
		return err.Error()
	}
	if err := exec.Command(path).Start(); err != nil {
		return err.Error()
	}
	application.Get().Quit()
	return ""
}

// ReleasesPage — страница выпусков программ Wynd.
func (a *App) ReleasesPage() { _ = application.Get().Browser.OpenURL(desktop.ReleasesPage) }

// windowOrigin — origin страницы окна: сборка Wails на Windows открывается
// по http://wails.localhost/, `wails dev` — на localhost. Чужому сайту,
// узнавшему случайный порт, локальный API ответы не читает: без своего origin
// в Access-Control-Allow-Origin браузер ответ отбрасывает.
func windowOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "wails.localhost", "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

func corsHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && windowOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Range, If-None-Match")
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	config, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	base := filepath.Join(config, "Wynd", "desktop")
	// Оболочка обновляет клиента вместе с сервером; служебный воркер PWA
	// внутри окна отдавал старый интерфейс из кэша — адрес сервера и
	// состояние связи выглядели чужими, а новые экраны не появлялись.
	// Убираем его до запуска окна; сессии и медиа-кэш в IndexedDB не трогаем.
	_ = os.RemoveAll(filepath.Join(base, "webview", "EBWebView", "Default", "Service Worker"))
	store, err := desktop.OpenStore(filepath.Join(base, "servers.json"))
	if err != nil {
		log.Fatal(err)
	}
	proxy := desktop.NewProxy()
	// API и поток событий страница берёт с локального порта: WebView2
	// буферизует ответы своих запросов, и SSE через него не течёт (сервер
	// выглядел «не отвечающим», хотя чтение работало). Порт случайный, только
	// на 127.0.0.1; адрес уходит в мост страницы (apiBase).
	apiLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	proxy.SetAPIBase("http://" + apiLn.Addr().String())
	go func() {
		if err := http.Serve(apiLn, corsHandler(proxy.Handler(http.NotFoundHandler()))); err != nil {
			log.Printf("api listener: %v", err)
		}
	}()
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
	shell := &App{
		store:  store,
		proxy:  proxy,
		client: &http.Client{Timeout: 12 * time.Second},
	}
	if err := shell.updateAllowed(); err != nil {
		log.Fatal(err)
	}
	app := application.New(application.Options{
		Name:        "wynd-desktop",
		Description: "Wynd Настольный",
		Services:    []application.Service{application.NewService(shell)},
		Assets:      application.AssetOptions{Handler: proxy.Handler(http.FileServer(http.FS(shellFS)))},
		Windows: application.WindowsOptions{
			// Постоянный профиль: сессия и медиа-кэш переживают перезапуск,
			// данные Wynd из системного профиля не используются.
			WebviewUserDataPath: filepath.Join(base, "webview"),
		},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "Wynd [" + desktop.Version + "]",
		// Окно открывается по ширине интерфейса: колонка клиента — до 720
		// (480 на узком окне), окно не шире неё (решение владельца 10.11.26).
		Width:     760,
		Height:    780,
		MinWidth:  500,
		MinHeight: 480,
		// Цвет бумаги (--paper): окно не мигает белым, пока грузится страница.
		BackgroundColour: application.NewRGB(0xF4, 0xF0, 0xE9),
		// Своя рамка (TitleBar), как на утверждённых кадрах; системный
		// заголовок Windows 10 в цвет бумаги не перекрашивается.
		Frameless: true,
		URL:       "/",
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
