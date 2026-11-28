//go:build dev || desktop || bindings

// Wynd Мастерская (план 45): установка и обслуживание Wynd на сервере.
// Собирается через scripts\build-workshop.bat (`wails3 generate syso` +
// `go build`) или `wails3 dev`: без их меток сборки пакет пуст, и ворота
// основного сервера (go vet, go test ./...) его не трогают. Вся логика —
// в internal/workshop.
package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"gitea.mixdep.ru/mix/wynd/internal/version"
	"gitea.mixdep.ru/mix/wynd/internal/workshop"
)

//go:embed all:frontend/dist
var assets embed.FS

// App — то, что окно может попросить (main.App.* в биндингах); пароль дальше
// Wizard не уходит.
type App struct {
	mu           sync.Mutex
	ctx          context.Context
	workshop     *workshop.Workshop
	profileError error
	// update — последняя найденная новая Мастерская (10а–10в).
	update workshop.SelfUpdate
	// updateDone/updateTotal — ход скачивания обновления для полосы: байты и
	// известный размер (0 — размер неизвестен).
	updateDone  int64
	updateTotal int64
}

// ServiceStartup и ServiceShutdown — хуки Wails v3: приложение передаёт
// контекст, который отменяется при закрытии окна.
func (a *App) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	a.ctx = ctx
	workshop.CleanOldSelf()
	return nil
}

func (a *App) ServiceShutdown() error {
	if a.workshop != nil {
		a.workshop.Close()
	}
	return nil
}

// HasSavedPassword — лежит ли в связке ключей пароль для этого сервера.
func (a *App) HasSavedPassword(host, user string) bool {
	return (&workshop.Wizard{Vault: keyringVault{}}).HasSavedPassword(host, user)
}

// Connect подключается к серверу; незнакомый сервер вернёт отпечаток.
func (a *App) Connect(id, password string, makeKey bool) (workshop.ConnectResult, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.ConnectResult {
		p := a.profile(id)
		if password == "" && !p.UseKeys {
			password = devPassword()
		}
		if !p.UseKeys && password == "" && (!p.Remember || !w.HasSavedPassword(p.Host, p.User)) {
			return workshop.ConnectResult{Status: "password_required", Host: p.Host, Message: "Для входа нужен пароль"}
		}
		return w.Connect(a.ctx, workshop.ConnectInput{Host: p.Host, Port: devPort(), User: p.User, Password: password, UseKeys: p.UseKeys, Remember: p.Remember, MakeKey: makeKey})
	})
}

// ConfirmHost — отпечаток сверен: запомнить сервер и войти.
func (a *App) ConfirmHost(id string) (workshop.ConnectResult, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.ConnectResult { return w.ConfirmHost(a.ctx) })
}

func (a *App) ConfirmChangedHost(id string) (workshop.ConnectResult, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.ConnectResult { return w.ConfirmChangedHost(a.ctx) })
}

// Inspect осматривает сервер, ничего не меняя.
func (a *App) Inspect(id string) (workshop.InspectResult, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.InspectResult { return w.Inspect(a.ctx) })
}

// CheckDomain проверяет, указывает ли адрес сайта на этот сервер.
func (a *App) CheckDomain(id string, domain string) (workshop.DomainCheck, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.DomainCheck { return w.CheckDomain(a.ctx, domain) })
}

// MakePlan — что будет сделано на сервере, до первого изменения.
func (a *App) MakePlan(id string, domain string) (workshop.Plan, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.Plan { return w.MakePlan(a.ctx, domain) })
}

// MakePlanWithPush добавляет к плану установки свой сервер уведомлений.
func (a *App) MakePlanWithPush(id string, domain, pushDomain string) (workshop.Plan, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.Plan {
		return w.MakePlanWithPush(a.ctx, domain, pushDomain)
	})
}

// ChooseBuild переключает способ: собрать Wynd на сервере или скачать готовый.
func (a *App) ChooseBuild(id string, on bool) (workshop.Plan, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.Plan { return w.ChooseBuild(on) })
}

// StartInstall запускает установку (или повторяет её с места отказа).
func (a *App) StartInstall(id string) (workshop.Progress, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.Progress { return w.StartInstall(a.ctx) })
}

// InstallProgress — как идёт установка; окно спрашивает раз в полсекунды.
func (a *App) InstallProgress(id string) (workshop.Progress, error) {
	if a.profileError != nil {
		return workshop.Progress{}, a.profileError
	}
	return a.workshop.Progress(id)
}

// Rollback убирает поставленное; keepData — оставить данные Wynd на сервере.
func (a *App) Rollback(id string, keepData bool) (workshop.RollbackResult, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.RollbackResult { return w.Rollback(a.ctx, keepData) })
}

// ServerPage — страница сервера, где Wynd уже стоит (пульт, кадры 7а–7е).
func (a *App) ServerPage(id string) (workshop.ServerPage, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.ServerPage { return w.ServerPage(a.ctx) })
}

// Service — «Запустить», «Остановить», «Перезапустить».
func (a *App) Service(id string, action string) (workshop.ServiceResult, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.ServiceResult { return w.Service(a.ctx, action) })
}

// PushControl — команды сервера уведомлений для текущего профиля.
func (a *App) PushControl(id string, action string, cache string) (workshop.ServiceResult, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.ServiceResult {
		return w.PushControl(a.ctx, action, cache)
	})
}

// PlanPushAdd проверяет домен и действующую установку перед добавлением Push.
func (a *App) PlanPushAdd(id string, domain string) (workshop.PushAddPlan, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.PushAddPlan {
		return w.PlanPushAdd(a.ctx, domain)
	})
}

// BrowseInstallDirectories lists folders on the SSH server of the selected profile.
func (a *App) BrowseInstallDirectories(id string, directory string) (workshop.RemoteDirectoryList, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.RemoteDirectoryList {
		return w.BrowseInstallDirectories(a.ctx, directory)
	})
}

// CreateInstallDirectory creates one directory on the SSH server of the selected profile.
func (a *App) CreateInstallDirectory(id string, parent string, name string) (workshop.RemoteDirectoryCreation, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.RemoteDirectoryCreation {
		return w.CreateInstallDirectory(a.ctx, parent, name)
	})
}

// DeleteStaleCert убирает сертификат с сломанным продлением (кнопка у находки осмотра).
func (a *App) DeleteStaleCert(id string, domain string) (workshop.ServiceResult, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.ServiceResult {
		return w.DeleteStaleCert(a.ctx, domain)
	})
}

// BuildLogChunk отдаёт порцию полного вывода сборки с сервера (C71).
func (a *App) BuildLogChunk(id string, offset int64, limit int) (workshop.BuildLogChunk, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.BuildLogChunk {
		return w.BuildLogChunk(a.ctx, offset, limit)
	})
}

// StartUpdate обновляет Wynd; ход — InstallProgress.
func (a *App) StartUpdate(id string, opt workshop.UpdateOptions) (workshop.Progress, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.Progress { return w.StartUpdate(a.ctx, opt) })
}

// PlanBackup — сколько скачать в папку folder (8а).
func (a *App) PlanBackup(id string, folder string) (workshop.LocalBackupPlan, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.LocalBackupPlan { return w.PlanBackup(a.ctx, folder) })
}

// StartBackup — бэкап на этот компьютер; ход — InstallProgress (8б).
func (a *App) StartBackup(id string, opt workshop.BackupOptions) (workshop.Progress, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.Progress { return w.StartBackup(a.ctx, opt) })
}

// DefaultBackupFolder — папка бэкапа по умолчанию: «Документы\Бэкапы Wynd\<адрес>».
func (a *App) DefaultBackupFolder(domain string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Documents", "Бэкапы Wynd", domain)
}

// ReturnPrevious возвращает прежнюю версию с данными из копии.
func (a *App) ReturnPrevious(id string) (workshop.ServiceResult, error) {
	return profileCall(a, id, false, func(w *workshop.Wizard) workshop.ServiceResult { return w.ReturnPrevious(a.ctx) })
}

// MoveConnect — подключение к новому серверу (9а).
func (a *App) MoveSelect(id, targetID, password string) (workshop.ConnectResult, error) {
	if a.profileError != nil {
		return workshop.ConnectResult{}, a.profileError
	}
	_, err := a.workshop.ReserveMove(id, targetID)
	if err != nil {
		return workshop.ConnectResult{}, err
	}
	return profileMoveCall(a, id, false, func(w *workshop.Wizard) workshop.ConnectResult {
		in, err := a.workshop.MoveInput(id, targetID, password)
		if err != nil {
			return workshop.ConnectResult{Status: "error", Message: err.Error()}
		}
		if in.Password == "" && !in.UseKeys {
			in.Password = devPassword()
		}
		if !in.UseKeys && in.Password == "" && (!in.Remember || !w.HasSavedPassword(in.Host, in.User)) {
			return workshop.ConnectResult{Status: "password_required", Host: in.Host, Message: "Для входа на новый сервер нужен пароль"}
		}
		in.Port = devPort()
		w.NewRoot = in.Root
		return w.MoveConnect(a.ctx, in)
	})
}

// MoveConfirmHost — отпечаток нового сервера сверен.
func (a *App) MoveConfirmHost(id string) (workshop.ConnectResult, error) {
	return profileMoveCall(a, id, false, func(w *workshop.Wizard) workshop.ConnectResult { return w.MoveConfirmHost(a.ctx) })
}

func (a *App) MoveConfirmChangedHost(id string) (workshop.ConnectResult, error) {
	return profileMoveCall(a, id, false, func(w *workshop.Wizard) workshop.ConnectResult { return w.MoveConfirmChangedHost(a.ctx) })
}

// MovePlan — план переезда (9б).
func (a *App) MovePlan(id string) (workshop.MovePlan, error) {
	if err := a.moveReady(id); err != nil {
		return workshop.MovePlan{}, err
	}
	return profileMoveCall(a, id, false, func(w *workshop.Wizard) workshop.MovePlan { return w.MovePlan(a.ctx) })
}

// StartMove — «Переехать»; ход — InstallProgress (9в).
func (a *App) StartMove(id string, removeOld bool) (workshop.Progress, error) {
	if err := a.moveReady(id); err != nil {
		return workshop.Progress{}, err
	}
	return profileMoveCall(a, id, false, func(w *workshop.Wizard) workshop.Progress { return w.StartMove(a.ctx, removeOld) })
}

// MoveCheckNow — «Проверить сейчас» (9в).
func (a *App) MoveCheckNow(id string) error {
	_, err := profileMoveCall(a, id, true, func(w *workshop.Wizard) bool { w.MoveCheckNow(); return true })
	return err
}

// CancelMove — «Отменить переезд» (9в).
func (a *App) CancelMove(id string) (workshop.ServiceResult, error) {
	if err := a.moveReady(id); err != nil {
		return workshop.ServiceResult{}, err
	}
	return profileMoveCall(a, id, true, func(w *workshop.Wizard) workshop.ServiceResult { return w.CancelMove(a.ctx) })
}

// MoveAdopt — после переезда — страница нового сервера (9г).
func (a *App) FinishMove(id string) (string, error) {
	if a.profileError != nil {
		return "", a.profileError
	}
	return a.workshop.ReleaseMove(id)
}

// CheckSelfUpdate — есть ли новая Мастерская (10а).
func (a *App) CheckSelfUpdate() workshop.SelfUpdate {
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	update := workshop.CheckSelfUpdate(ctx, http.DefaultClient, workshop.WorkshopVersion, time.Now())
	a.mu.Lock()
	a.update = update
	a.mu.Unlock()
	return update
}

// SelfUpdateProgress — сколько байт обновления уже скачано: полоса обновления
// показывает ход установки, пока кнопка «Обновляем…» (Total = 0 — ещё не
// начали или размер неизвестен).
func (a *App) SelfUpdateProgress() workshop.UpdateProgress {
	a.mu.Lock()
	defer a.mu.Unlock()
	return workshop.UpdateProgress{Done: a.updateDone, Total: a.updateTotal}
}

// ApplySelfUpdate — «Обновить и перезапустить» (10б): подпись не сошлась —
// "signature" (10в); вышло — новая Мастерская запускается, эта закрывается.
func (a *App) ApplySelfUpdate() string {
	if a.profileError != nil {
		return a.profileError.Error()
	}
	if err := a.workshop.BeginSelfUpdate(); err != nil {
		return err.Error()
	}
	defer a.workshop.EndSelfUpdate()
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
	defer cancel()
	a.mu.Lock()
	update := a.update
	a.updateDone, a.updateTotal = 0, 0
	a.mu.Unlock()
	path, err := workshop.ApplySelfUpdate(ctx, http.DefaultClient, update, func(done, total int64) {
		a.mu.Lock()
		a.updateDone, a.updateTotal = done, total
		a.mu.Unlock()
	})
	if errors.Is(err, workshop.ErrBadSignature) {
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

// ReleasesPage — страница выпусков Мастерской.
func (a *App) ReleasesPage() { _ = application.Get().Browser.OpenURL(workshop.ReleasesPage) }

// OpenLink открывает ссылку первого запуска в браузере человека.
func (a *App) OpenLink(url string) {
	_ = application.Get().Browser.OpenURL(url)
}

// CopyText кладёт текст в буфер обмена.
func (a *App) CopyText(text string) bool {
	return application.Get().Clipboard.SetText(text)
}

// Version — номер версии Мастерской для полосы заголовка (своя рамка окна).
func (a *App) Version() string { return workshop.WorkshopVersion }

// Disconnect рвёт подключение и забывает пароль из памяти.
func (a *App) Disconnect(id string) error {
	_, err := profileCall(a, id, false, func(w *workshop.Wizard) bool { w.Close(); return true })
	return err
}

func main() {
	factory := func() *workshop.Wizard {
		return &workshop.Wizard{
			Dialer: devDialer(),
			Vault:  keyringVault{},
			// Версия Wynd, которую Мастерская знает со сборки; ставит она свежую
			// из хранилища, эта — когда хранилище не ответило.
			Version: version.Number,
			Image:   imageSource(),
			Source:  workshop.SourceImage{Repo: workshop.DefaultSource},
			// В собранном приложении всегда ложно.
			TestCert: devTestCert(),
		}
	}
	config, err := os.UserConfigDir()
	var store *workshop.Workshop
	if err == nil {
		store, err = workshop.OpenWorkshop(filepath.Join(config, "Wynd", "workshop", "servers.json"), factory)
	}
	if err != nil {
		err = fmt.Errorf("Не удалось открыть список серверов: %w", err)
	}
	app := &App{workshop: store, profileError: err}
	program := application.New(application.Options{
		Name:        "wynd-workshop",
		Description: "Wynd Мастерская",
		Services:    []application.Service{application.NewService(app)},
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
	})
	program.Window.NewWithOptions(application.WebviewWindowOptions{
		// Своя версия Мастерской, не Wynd (план 45, 2026-10-24).
		Title:     "Wynd Мастерская [" + workshop.WorkshopVersion + "]",
		Width:     760,
		Height:    560,
		MinWidth:  680,
		MinHeight: 480,
		// Цвет бумаги (--paper): окно не мигает белым, пока грузится страница.
		BackgroundColour: application.NewRGB(0xF4, 0xF0, 0xE9),
		// Своя рамка, как на макете (frontend/src/Window.svelte): системный
		// заголовок на Windows 10 в цвет не перекрашивается. Название окна
		// выше остаётся — его видно на панели задач.
		Frameless: true,
		URL:       "/",
	})
	if err := program.Run(); err != nil {
		log.Fatal(err)
	}
}
