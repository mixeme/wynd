//go:build dev || desktop || bindings

// Окно Wynd для компьютера (планы 55 и 61). Собирается через
// scripts\build-desktop.bat (`wails3 generate syso` + `go build`) или
// `wails3 dev`: без их меток сборки пакет пуст, и ворота основного сервера
// (go vet, go test ./...) его не трогают.
package main

import (
	"context"
	"embed"
	"encoding/json"
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
	"github.com/wailsapp/wails/v3/pkg/events"

	"gitea.mixdep.ru/mix/wynd/internal/desktop"
	wyndweb "gitea.mixdep.ru/mix/wynd/web"
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
	// updateDone/updateTotal — ход скачивания обновления для полосы: байты и
	// известный размер (0 — размер неизвестен).
	updateDone  int64
	updateTotal int64
	// updateNotified — версия, о которой уведомляли в трей: дедуп, один
	// toast на версию (план 66, «вышла версия»).
	updateNotified string
	// pendingInvite — адрес схемы wynd:, не дошедший до оболочки событием
	// (план 66, функция 4); оболочка забирает его при загрузке.
	pendingInvite string
	// Копии по расписанию (план 66, функция 7): настройки backups.json и
	// хранилище сессий для сборки архивов.
	backupPath string
	backupMu   sync.Mutex
	backup     desktop.BackupConfig
	secrets    *desktop.SecretsCache
	// window и tray — для «крестика»: со значком в трее окно прячется,
	// без него приложение закрывается, как раньше.
	window application.Window
	tray   *application.SystemTray
	// trayMenu — маленькое окно меню в стиле рамки, показывается правым
	// щелчком по значку (C66).
	trayMenu application.Window
	// source — источник интерфейса этого запуска: вшитый клиент из exe или
	// страницы выбранного сервера. Настройка живёт в servers.json; флаг
	// --server-ui задаёт серверный режим только на этот запуск.
	source string
}

// ServiceStartup — хук Wails v3: приложение передаёт контекст, который
// отменяется при закрытии окна.
func (a *App) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	a.ctx = ctx
	desktop.CleanOldSelf()
	// Периодическая проверка обновлений: фронтенд спрашивает один раз при
	// запуске, а окно может жить неделями спрятанным — тут оно узнаёт о новых
	// версиях без открытого интерфейса (план 66, «вышла версия»).
	go a.runUpdateToasts()
	// Копии по расписанию собираются, пока приложение запущено, — окно может
	// быть спрятано в значок (план 66, функция 7).
	go a.runBackupLoop()
	return nil
}

// DeliverInvite — адрес приглашения от схемы wynd: (план 66, функция 4).
// Событием оболочке (если она уже загрузилась) и в запас — pendingInvite:
// событие, ушедшее до привязки JS, не теряется, оболочка забирает его сама
// при старте. Второй запуск не создаёт окно — показывает это и несёт адрес.
func (a *App) DeliverInvite(arg string) {
	if _, ok := desktop.ParseInviteURL(arg); !ok {
		return
	}
	a.mu.Lock()
	a.pendingInvite = arg
	a.mu.Unlock()
	if app := application.Get(); app != nil {
		app.Event.Emit("wynd:invite", arg)
	}
	if a.window != nil {
		a.window.Show()
		a.window.Focus()
	}
}

// TakePendingInvite — запасной канал: оболочка забирает адрес при загрузке,
// если событие ушло раньше её готовности.
func (a *App) TakePendingInvite() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	arg := a.pendingInvite
	a.pendingInvite = ""
	return arg
}

// launchArgFromArgs — первый адрес схемы wynd: среди аргументов запуска.
func launchArgFromArgs(args []string) string {
	for _, arg := range args {
		if _, ok := desktop.ParseInviteURL(arg); ok {
			return arg
		}
	}
	return ""
}

// GetBackupConfig — настройки копий для экрана (план 66, функция 7).
func (a *App) GetBackupConfig() desktop.BackupConfig {
	a.backupMu.Lock()
	defer a.backupMu.Unlock()
	return a.backup
}

// SaveBackupConfig проверяет и сохраняет настройки; форму не подменяем молча —
// ошибку показывает экран.
func (a *App) SaveBackupConfig(cfg desktop.BackupConfig) (desktop.BackupConfig, error) {
	norm, err := desktop.NormalizeBackupConfig(cfg)
	if err != nil {
		return desktop.BackupConfig{}, err
	}
	a.backupMu.Lock()
	// Поля прошлого прогона ведёт таймер, а не форма.
	norm.LastAt, norm.LastCircles = a.backup.LastAt, a.backup.LastCircles
	a.backup = norm
	a.backupMu.Unlock()
	if err := writeBackupConfig(a.backupPath, norm); err != nil {
		return desktop.BackupConfig{}, err
	}
	return norm, nil
}

// ChooseBackupFolder — родной диалог выбора папки.
func (a *App) ChooseBackupFolder() (string, error) {
	path, err := application.Get().Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:                "Папка для копий",
		CanChooseDirectories: true,
		CanChooseFiles:       false,
		CanCreateDirectories: true,
	}).PromptForSingleSelection()
	if err != nil {
		// Отмена в v3 приходит ошибкой диалога — это не сбой.
		if strings.Contains(strings.ToLower(err.Error()), "cancel") {
			return "", nil
		}
		return "", err
	}
	return path, nil
}

// OpenBackupFolder — показать папку в Проводнике.
func (a *App) OpenBackupFolder() error {
	a.backupMu.Lock()
	folder := a.backup.Folder
	a.backupMu.Unlock()
	if folder == "" {
		return errors.New("backups: папка не выбрана")
	}
	return openFolderInFileManager(folder)
}

// loadBackupConfig читает backups.json; отсутствие файла — умолчания.
func (a *App) loadBackupConfig(path string) {
	a.backupPath = path
	a.backup = desktop.DefaultBackupConfig()
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var cfg desktop.BackupConfig
	if json.Unmarshal(b, &cfg) == nil {
		a.backup = cfg
	}
}

// writeBackupConfig пишет настройки атомарно, через временный файл.
func writeBackupConfig(path string, cfg desktop.BackupConfig) error {
	b, err := json.MarshalIndent(cfg, "", "	")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// backupCheckInterval — как часто окно проверяет срок копии.
const backupCheckInterval = 30 * time.Minute

// runBackupLoop — своя проверка раз в полчаса; первая — через полминуты
// после запуска, чтобы включённые копии собрались сразу.
func (a *App) runBackupLoop() {
	first := time.NewTimer(30 * time.Second)
	select {
	case <-a.ctx.Done():
		first.Stop()
		return
	case <-first.C:
	}
	a.maybeRunBackup()
	ticker := time.NewTicker(backupCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.maybeRunBackup()
		}
	}
}

// maybeRunBackup собирает копии выбранного сервера, когда срок прошёл.
func (a *App) maybeRunBackup() {
	a.backupMu.Lock()
	cfg := a.backup
	a.backupMu.Unlock()
	var last time.Time
	if cfg.LastAt != "" {
		last, _ = time.Parse(time.RFC3339, cfg.LastAt)
	}
	now := time.Now()
	if !desktop.BackupDue(last, cfg, now) {
		return
	}
	selected, _ := a.store.Snapshot()
	if selected == "" || a.secrets == nil {
		return
	}
	blob, ok := a.secrets.Get(selected)
	if !ok {
		log.Printf("backups: нет сохранённой сессии для %s", selected)
		return
	}
	var session struct {
		Token string `json:"token"`
	}
	if json.Unmarshal(blob, &session) != nil || session.Token == "" {
		return
	}
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Minute)
	defer cancel()
	saved, err := desktop.RunBackupOnce(ctx, http.DefaultClient, selected, session.Token, cfg.Folder, cfg, now)
	if err != nil {
		log.Printf("backups: %v (собрано %d)", err, saved)
	}
	cfg.LastAt = now.UTC().Format(time.RFC3339)
	cfg.LastCircles = saved
	a.backupMu.Lock()
	a.backup = cfg
	a.backupMu.Unlock()
	if err := writeBackupConfig(a.backupPath, cfg); err != nil {
		log.Printf("backups: сохранить настройки: %v", err)
	}
}

// updateCheckInterval — как часто спрятанное окно само проверяет новые версии.
const updateCheckInterval = 6 * time.Hour

func (a *App) runUpdateToasts() {
	ticker := time.NewTicker(updateCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
			update := desktop.CheckUpdate(ctx, http.DefaultClient, desktop.Version, time.Now())
			cancel()
			a.mu.Lock()
			a.update = update
			a.mu.Unlock()
			a.maybeNotifyUpdate()
		}
	}
}

// maybeNotifyUpdate — показать toast о найденной версии, если он ещё не
// показывался и интерфейса не видно; на виду полоса обновления сама.
func (a *App) maybeNotifyUpdate() {
	a.mu.Lock()
	update, notified, win := a.update, a.updateNotified, a.window
	a.mu.Unlock()
	title, body, ok := desktop.NotifyUpdateText(update, notified)
	if !ok {
		return
	}
	if win != nil {
		if visible, minimised := win.IsVisible(), win.IsMinimised(); !desktop.ShouldNotify(visible, minimised) {
			return
		}
	}
	if err := desktop.ShowToast(title, body); err != nil {
		log.Printf("update toast: %v", err)
		return
	}
	a.mu.Lock()
	a.updateNotified = update.Version
	a.mu.Unlock()
}

// RequestClose — «крестик» окна (C66): при значке в трее окно прячется и
// работа продолжается, без значка приложение закрывается, как раньше.
func (a *App) RequestClose() {
	a.mu.Lock()
	win, tray := a.window, a.tray
	a.mu.Unlock()
	if win != nil && platformCanHideToTray(tray) {
		win.Hide()
		a.maybeNotifyUpdate()
		return
	}
	application.Get().Quit()
}

// TrayOpen — пункт «Открыть» меню трея: спрятать меню, показать окно.
func (a *App) TrayOpen() {
	a.HideTrayMenu()
	a.mu.Lock()
	win := a.window
	a.mu.Unlock()
	if win != nil {
		win.Show()
		win.Focus()
	}
}

// TrayQuit — пункт «Выход»: закрыть программу полностью.
func (a *App) TrayQuit() {
	a.HideTrayMenu()
	application.Get().Quit()
}

// HideTrayMenu — закрыть меню (пункт выбран, Esc или потеря фокуса).
func (a *App) HideTrayMenu() {
	a.mu.Lock()
	menu := a.trayMenu
	a.mu.Unlock()
	if menu != nil {
		menu.Hide()
	}
}

// Notify показывает системное уведомление Windows, когда интерфейса не
// видно: окно спрятано в трей или свёрнуто (план 66, функция 2). На виду
// мост молчит — человек и так смотрит в Wynd. Кто зовёт мост и о чём —
// следующий шаг плана, решение владельца.
func (a *App) Notify(title, body string) error {
	a.mu.Lock()
	win := a.window
	a.mu.Unlock()
	visible, minimised := false, false
	if win != nil {
		visible, minimised = win.IsVisible(), win.IsMinimised()
	}
	if !desktop.ShouldNotify(visible, minimised) {
		return nil
	}
	return desktop.ShowToast(title, body)
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
// Без выбранного сервера источник возвращается к вшитому: серверных страниц
// показывать не от кого.
func (a *App) ForgetSelected() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.store == nil {
		return nil
	}
	if err := a.store.ClearSelection(); err != nil {
		return err
	}
	a.source = desktop.SourceEmbedded
	a.proxy.SetSource(desktop.SourceEmbedded)
	return a.proxy.Set("")
}

// InterfaceSource — текущий источник интерфейса окна: embedded или server.
func (a *App) InterfaceSource() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.source == desktop.SourceServer {
		return desktop.SourceServer
	}
	return desktop.SourceEmbedded
}

// SelectEmbedded возвращает окно к вшитому интерфейсу, не меняя выбранный
// сервер: он остаётся источником данных.
func (a *App) SelectEmbedded() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.storeErr != nil {
		return a.storeErr
	}
	if err := a.store.SetSource(desktop.SourceEmbedded); err != nil {
		return err
	}
	a.source = desktop.SourceEmbedded
	a.proxy.SetSource(desktop.SourceEmbedded)
	return nil
}

// SelectServer переключает окно на другой источник интерфейса: страницы
// открываются с выбранного сервера, вшитый клиент остаётся для возврата.
func (a *App) SelectServer(origin string) (desktop.Server, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.storeErr != nil {
		return desktop.Server{}, a.storeErr
	}
	if err := a.store.Select(origin); err != nil {
		return desktop.Server{}, err
	}
	if err := a.store.SetSource(desktop.SourceServer); err != nil {
		return desktop.Server{}, err
	}
	if err := a.proxy.Set(origin); err != nil {
		return desktop.Server{}, err
	}
	a.source = desktop.SourceServer
	a.proxy.SetSource(desktop.SourceServer)
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

// SelfUpdateProgress — сколько байт обновления уже скачано: полоса обновления
// показывает ход установки, пока кнопка «Обновляем…» (Total = 0 — ещё не
// начали или размер неизвестен).
func (a *App) SelfUpdateProgress() desktop.UpdateProgress {
	a.mu.Lock()
	defer a.mu.Unlock()
	return desktop.UpdateProgress{Done: a.updateDone, Total: a.updateTotal}
}

// ApplySelfUpdate — «Обновить и перезапустить»: подпись не сошлась —
// "signature"; вышло — новая версия запускается, эта закрывается.
func (a *App) ApplySelfUpdate() string {
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
	defer cancel()
	a.mu.Lock()
	update := a.update
	a.updateDone, a.updateTotal = 0, 0
	a.mu.Unlock()
	path, err := desktop.ApplyUpdate(ctx, http.DefaultClient, update, func(done, total int64) {
		a.mu.Lock()
		a.updateDone, a.updateTotal = done, total
		a.mu.Unlock()
	})
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
			// Загрузка кусками читает и присылает Upload-Offset; без явного
			// разрешения браузер блокирует PUT вложения (C66, «дозагрузка»).
			w.Header().Set("Access-Control-Expose-Headers", "Upload-Offset, Upload-Length")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Range, If-None-Match, Upload-Offset")
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
	// Флаги запуска читаем до сборки локального API: --smoke-test и
	// --from-startup влияют на окно, --server-ui — на источник интерфейса.
	startInTray := false
	smokeTest := false
	forceServerUI := false
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--from-startup":
			startInTray = true
		case "--smoke-test":
			smokeTest = true
		case "--server-ui":
			// Откат и проверка: вшитый клиент выключен только на этот запуск.
			forceServerUI = true
		case "--embedded-ui":
			// Прежний ключ (до перехода на вшитый интерфейс по умолчанию);
			// оставлен, чтобы старые ярлыки и проверки не ломались.
		}
	}
	startInTray = startInTray && platformCanStartHidden()
	// Оболочка обновляет клиента вместе с сервером; служебный воркер PWA
	// внутри окна отдавал старый интерфейс из кэша — адрес сервера и
	// состояние связи выглядели чужими, а новые экраны не появлялись.
	// Windows-путь убирает его до запуска окна; на Linux воркера PWA нет.
	preparePlatformProfile(base)
	store, err := desktop.OpenStore(filepath.Join(base, "servers.json"))
	if err != nil {
		log.Fatal(err)
	}
	// Системные уведомления (план 66, функция 2): AUMID и значок в HKCU,
	// чтобы всплывающее уведомление показывалось от «Wynd Настольный», а не
	// от PowerShell. Сбой не мешает окну — уведомления деградируют к имени.
	if err := desktop.EnsureToastIdentity(base, trayIcon); err != nil {
		log.Printf("toast identity: %v", err)
	}
	// Схема wynd: (план 66, функция 4): браузер передаёт приглашение окну.
	// Команда — путь этого exe, переписывается при каждом запуске; сбой
	// (политика реестра) не мешает окну.
	if err := desktop.EnsureInviteScheme(); err != nil {
		log.Printf("invite scheme: %v", err)
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
	// Вшитый интерфейс (план 66, функция 9) — основной: страницы клиента
	// лежат в exe, данные идут с выбранного сервера. Origin окна не меняется —
	// сессии, снимки и очередь остаются на месте. Серверный интерфейс
	// остаётся явным выбором для отката (source в servers.json или --server-ui).
	dist, err := fs.Sub(wyndweb.Build, "dist")
	if err != nil {
		log.Fatalf("embedded ui: %v", err)
	}
	proxy.SetEmbeddedUI(desktop.EmbeddedHandler(dist, proxy.Target, "http://"+apiLn.Addr().String()))
	source := store.Source()
	if forceServerUI {
		source = desktop.SourceServer
	}
	proxy.SetSource(source)
	if source == desktop.SourceEmbedded {
		log.Printf("embedded ui: интерфейс из exe")
	} else {
		log.Printf("server ui: интерфейс с сервера")
	}
	// Сессии окна (план 66, функция 3): токены в Credential Manager, клиент
	// работает с ними по apiBase тем же CORS-контуром, что и за API.
	secrets := desktop.NewSecretsCache(desktop.NewSecretStore())
	apiMux := http.NewServeMux()
	apiMux.Handle("/__secrets", desktop.SessionsHandler(secrets))
	apiMux.Handle("/__secrets/", desktop.SessionsHandler(secrets))
	apiMux.Handle("/", proxy.Handler(http.NotFoundHandler()))
	go func() {
		if err := http.Serve(apiLn, corsHandler(apiMux)); err != nil {
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
		store:   store,
		proxy:   proxy,
		client:  &http.Client{Timeout: 12 * time.Second},
		secrets: secrets,
		source:  source,
	}
	shell.loadBackupConfig(filepath.Join(base, "backups.json"))
	if err := shell.updateAllowed(); err != nil {
		log.Fatal(err)
	}
	// Приглашение из схемы wynd: при первом запуске: событие оболочке уйдёт
	// раньше её загрузки, поэтому адрес ждёт её в pendingInvite — она забирает
	// сама при старте (план 66, функция 4).
	if arg := launchArgFromArgs(os.Args[1:]); arg != "" {
		shell.pendingInvite = arg
		startInTray = false // человека позвали — окно показываем
	}
	// Самообновление запускает новый файл, пока прежний процесс ещё выходит.
	// Wails берёт замок одного экземпляра в application.New: без паузы новый
	// запуск уходит в прежний и завершается, а полоса остаётся на
	// «Обновляем…» (живая проверка 27.11, 0.5.2 → 0.5.5). Признак подмены —
	// прежний образ рядом (.old): ждём, пока прежний процесс отпустит замок.
	// У Linux-окна путь — SelfPath(): у AppImage это сам .AppImage.
	if desktop.FreshlyReplaced(desktop.SelfPath()) {
		time.Sleep(2 * time.Second)
	}
	// Второй запуск (ярлык, автозапуск при уже работающей программе) не
	// создаёт второй процесс и второй значок: окно первого показывается.
	var mainWindow application.Window
	app := application.New(application.Options{
		Name:        "wynd-desktop",
		Description: "Wynd Настольный",
		// Ошибки без экрана (отказ горячей клавиши, внутренние) — в журнал,
		// иначе они пропадают молча (план 66, живая проверка не прошла).
		ErrorHandler: func(err error) { log.Printf("wails: %v", err) },
		Services:     []application.Service{application.NewService(shell)},
		Assets:       application.AssetOptions{Handler: proxy.Handler(http.FileServer(http.FS(shellFS)))},
		Windows:      platformWindowsOptions(base),
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.wynd.desktop",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				// Запуск с адресом приглашения (схема wynd:, план 66) несёт
				// его первому процессу аргументом.
				if arg := launchArgFromArgs(data.Args); arg != "" {
					shell.DeliverInvite(arg)
					return
				}
				if mainWindow != nil {
					mainWindow.Show()
					mainWindow.Focus()
				}
			},
		},
	})
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "Wynd [" + desktop.Version + "]",
		// Окно открывается сразу широким: список 340 и содержимое 720 —
		// 1060 px (план 76). Уже 1060 — одна колонка, как на телефоне.
		Width:     1060,
		Height:    780,
		MinWidth:  500,
		MinHeight: 480,
		// Цвет бумаги (--paper): окно не мигает белым, пока грузится страница.
		BackgroundColour: application.NewRGB(0xF4, 0xF0, 0xE9),
		// Своя рамка (TitleBar), как на утверждённых кадрах; системный
		// заголовок Windows 10 в цвет бумаги не перекрашивается.
		Frameless: true,
		Hidden:    startInTray,
		URL:       "/",
	})
	mainWindow = win
	// Меню в трее: на Windows — своё маленькое окно в стиле рамки (C66);
	// на Linux — нативное меню StatusNotifierItem (см. platform-файлы).
	menuWin := newTrayMenuWindow(app)
	// Значок в области уведомлений (C66): открыть окно или выйти совсем.
	tray := app.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("Wynd Настольный")
	setupPlatformTray(app, tray, win, shell)
	// Закрытие окна любым путём (крестик, Alt+F4, задача) с треем не
	// завершает программу, а прячет окно: работа продолжается в значке.
	// На Linux без подтверждённого трея окно закрывается обычным образом.
	if platformCanHideToTray(tray) {
		win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
			e.Cancel()
			win.Hide()
			shell.maybeNotifyUpdate()
		})
	}
	// Общая горячая клавиша (C66, план 66): Ctrl+Alt+W показывает и
	// фокусирует окно из любого места Windows, как второй запуск. Регистрация
	// до app.Run допустима — Wails привяжет клавишу при старте; если комбинацию
	// успела занять другая программа, отказ уйдёт в журнал, окно продолжит
	// работать без горячей клавиши.
	if err := app.GlobalShortcut.Register("Ctrl+Alt+W", func() {
		win.Show()
		win.Focus()
	}); err != nil {
		log.Printf("global shortcut: %v", err)
	}
	shell.mu.Lock()
	shell.window = win
	shell.tray = tray
	shell.trayMenu = menuWin
	shell.mu.Unlock()
	// Прогрев меню Windows: первое создание WebView2 занимает секунды, а
	// правый щелчок должен показывать меню сразу. Окно 1×1 за краем экрана,
	// затем прячется; на показе размер возвращается. На Linux окна меню нет.
	if menuWin != nil {
		go func() {
			time.Sleep(1500 * time.Millisecond)
			application.InvokeSync(func() {
				menuWin.SetSize(1, 1)
				menuWin.SetPosition(-10000, -10000)
				menuWin.Show()
			})
			time.Sleep(2 * time.Second)
			application.InvokeSync(func() { menuWin.Hide() })
		}()
	}
	if smokeTest {
		go func() {
			time.Sleep(5 * time.Second)
			log.Printf("smoke: окно запущено")
			app.Quit()
		}()
	}
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
