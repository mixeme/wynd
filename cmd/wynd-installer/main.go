//go:build dev || desktop || bindings

// Настольный установщик Wynd (план 45). Собирается через `wails build` или
// `wails dev`: без их меток сборки пакет пуст, и ворота основного сервера
// (go vet, go test ./...) его не трогают. Вся логика — в internal/installer.
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

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"gitea.mixdep.ru/mix/wynd/internal/installer"
	"gitea.mixdep.ru/mix/wynd/internal/version"
)

//go:embed all:frontend/dist
var assets embed.FS

// App — то, что окно может попросить. Методы видны из окна как
// window.go.main.App.*; пароль дальше Wizard не уходит.
type App struct {
	mu           sync.Mutex
	ctx          context.Context
	workshop     *installer.Workshop
	profileError error
	// update — последняя найденная новая Мастерская (10а–10в).
	update installer.SelfUpdate
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	installer.CleanOldSelf()
}

func (a *App) shutdown(context.Context) {
	if a.workshop != nil {
		a.workshop.Close()
	}
}

// HasSavedPassword — лежит ли в связке ключей пароль для этого сервера.
func (a *App) HasSavedPassword(host, user string) bool {
	return (&installer.Wizard{Vault: keyringVault{}}).HasSavedPassword(host, user)
}

// Connect подключается к серверу; незнакомый сервер вернёт отпечаток.
func (a *App) Connect(id, password string, makeKey bool) (installer.ConnectResult, error) {
	return profileCall(a, id, false, func(w *installer.Wizard) installer.ConnectResult {
		p := a.profile(id)
		if password == "" && !p.UseKeys {
			password = devPassword()
		}
		if !p.UseKeys && password == "" && (!p.Remember || !w.HasSavedPassword(p.Host, p.User)) {
			return installer.ConnectResult{Status: "password_required", Host: p.Host, Message: "Для входа нужен пароль"}
		}
		return w.Connect(a.ctx, installer.ConnectInput{Host: p.Host, Port: devPort(), User: p.User, Password: password, UseKeys: p.UseKeys, Remember: p.Remember, MakeKey: makeKey})
	})
}

// ConfirmHost — отпечаток сверен: запомнить сервер и войти.
func (a *App) ConfirmHost(id string) (installer.ConnectResult, error) {
	return profileCall(a, id, false, func(w *installer.Wizard) installer.ConnectResult { return w.ConfirmHost(a.ctx) })
}

func (a *App) ConfirmChangedHost(id string) (installer.ConnectResult, error) {
	return profileCall(a, id, false, func(w *installer.Wizard) installer.ConnectResult { return w.ConfirmChangedHost(a.ctx) })
}

// Inspect осматривает сервер, ничего не меняя.
func (a *App) Inspect(id string) (installer.InspectResult, error) {
	return profileCall(a, id, false, func(w *installer.Wizard) installer.InspectResult { return w.Inspect(a.ctx) })
}

// CheckDomain проверяет, указывает ли адрес сайта на этот сервер.
func (a *App) CheckDomain(id string, domain string) (installer.DomainCheck, error) {
	return profileCall(a, id, false, func(w *installer.Wizard) installer.DomainCheck { return w.CheckDomain(a.ctx, domain) })
}

// MakePlan — что будет сделано на сервере, до первого изменения.
func (a *App) MakePlan(id string, domain string) (installer.Plan, error) {
	return profileCall(a, id, false, func(w *installer.Wizard) installer.Plan { return w.MakePlan(a.ctx, domain) })
}

// ChooseBuild переключает способ: собрать Wynd на сервере или скачать готовый.
func (a *App) ChooseBuild(id string, on bool) (installer.Plan, error) {
	return profileCall(a, id, false, func(w *installer.Wizard) installer.Plan { return w.ChooseBuild(on) })
}

// StartInstall запускает установку (или повторяет её с места отказа).
func (a *App) StartInstall(id string) (installer.Progress, error) {
	return profileCall(a, id, false, func(w *installer.Wizard) installer.Progress { return w.StartInstall(a.ctx) })
}

// InstallProgress — как идёт установка; окно спрашивает раз в полсекунды.
func (a *App) InstallProgress(id string) (installer.Progress, error) {
	if a.profileError != nil {
		return installer.Progress{}, a.profileError
	}
	return a.workshop.Progress(id)
}

// Rollback убирает поставленное; keepData — оставить данные Wynd на сервере.
func (a *App) Rollback(id string, keepData bool) (installer.RollbackResult, error) {
	return profileCall(a, id, false, func(w *installer.Wizard) installer.RollbackResult { return w.Rollback(a.ctx, keepData) })
}

// ServerPage — страница сервера, где Wynd уже стоит (пульт, кадры 7а–7е).
func (a *App) ServerPage(id string) (installer.ServerPage, error) {
	return profileCall(a, id, false, func(w *installer.Wizard) installer.ServerPage { return w.ServerPage(a.ctx) })
}

// Service — «Запустить», «Остановить», «Перезапустить».
func (a *App) Service(id string, action string) (installer.ServiceResult, error) {
	return profileCall(a, id, false, func(w *installer.Wizard) installer.ServiceResult { return w.Service(a.ctx, action) })
}

// StartUpdate обновляет Wynd; ход — InstallProgress.
func (a *App) StartUpdate(id string, opt installer.UpdateOptions) (installer.Progress, error) {
	return profileCall(a, id, false, func(w *installer.Wizard) installer.Progress { return w.StartUpdate(a.ctx, opt) })
}

// PlanBackup — сколько скачать в папку folder (8а).
func (a *App) PlanBackup(id string, folder string) (installer.LocalBackupPlan, error) {
	return profileCall(a, id, false, func(w *installer.Wizard) installer.LocalBackupPlan { return w.PlanBackup(a.ctx, folder) })
}

// StartBackup — бэкап на этот компьютер; ход — InstallProgress (8б).
func (a *App) StartBackup(id string, opt installer.BackupOptions) (installer.Progress, error) {
	return profileCall(a, id, false, func(w *installer.Wizard) installer.Progress { return w.StartBackup(a.ctx, opt) })
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
func (a *App) ReturnPrevious(id string) (installer.ServiceResult, error) {
	return profileCall(a, id, false, func(w *installer.Wizard) installer.ServiceResult { return w.ReturnPrevious(a.ctx) })
}

// MoveConnect — подключение к новому серверу (9а).
func (a *App) MoveSelect(id, targetID, password string) (installer.ConnectResult, error) {
	if a.profileError != nil {
		return installer.ConnectResult{}, a.profileError
	}
	_, err := a.workshop.ReserveMove(id, targetID)
	if err != nil {
		return installer.ConnectResult{}, err
	}
	return profileMoveCall(a, id, false, func(w *installer.Wizard) installer.ConnectResult {
		in, err := a.workshop.MoveInput(id, targetID, password)
		if err != nil {
			return installer.ConnectResult{Status: "error", Message: err.Error()}
		}
		if in.Password == "" && !in.UseKeys {
			in.Password = devPassword()
		}
		if !in.UseKeys && in.Password == "" && (!in.Remember || !w.HasSavedPassword(in.Host, in.User)) {
			return installer.ConnectResult{Status: "password_required", Host: in.Host, Message: "Для входа на новый сервер нужен пароль"}
		}
		in.Port = devPort()
		w.NewRoot = in.Root
		return w.MoveConnect(a.ctx, in)
	})
}

// MoveConfirmHost — отпечаток нового сервера сверен.
func (a *App) MoveConfirmHost(id string) (installer.ConnectResult, error) {
	return profileMoveCall(a, id, false, func(w *installer.Wizard) installer.ConnectResult { return w.MoveConfirmHost(a.ctx) })
}

func (a *App) MoveConfirmChangedHost(id string) (installer.ConnectResult, error) {
	return profileMoveCall(a, id, false, func(w *installer.Wizard) installer.ConnectResult { return w.MoveConfirmChangedHost(a.ctx) })
}

// MovePlan — план переезда (9б).
func (a *App) MovePlan(id string) (installer.MovePlan, error) {
	if err := a.moveReady(id); err != nil {
		return installer.MovePlan{}, err
	}
	return profileMoveCall(a, id, false, func(w *installer.Wizard) installer.MovePlan { return w.MovePlan(a.ctx) })
}

// StartMove — «Переехать»; ход — InstallProgress (9в).
func (a *App) StartMove(id string, removeOld bool) (installer.Progress, error) {
	if err := a.moveReady(id); err != nil {
		return installer.Progress{}, err
	}
	return profileMoveCall(a, id, false, func(w *installer.Wizard) installer.Progress { return w.StartMove(a.ctx, removeOld) })
}

// MoveCheckNow — «Проверить сейчас» (9в).
func (a *App) MoveCheckNow(id string) error {
	_, err := profileMoveCall(a, id, true, func(w *installer.Wizard) bool { w.MoveCheckNow(); return true })
	return err
}

// CancelMove — «Отменить переезд» (9в).
func (a *App) CancelMove(id string) (installer.ServiceResult, error) {
	if err := a.moveReady(id); err != nil {
		return installer.ServiceResult{}, err
	}
	return profileMoveCall(a, id, true, func(w *installer.Wizard) installer.ServiceResult { return w.CancelMove(a.ctx) })
}

// MoveAdopt — после переезда — страница нового сервера (9г).
func (a *App) FinishMove(id string) (string, error) {
	if a.profileError != nil {
		return "", a.profileError
	}
	return a.workshop.ReleaseMove(id)
}

// CheckSelfUpdate — есть ли новая Мастерская (10а).
func (a *App) CheckSelfUpdate() installer.SelfUpdate {
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	update := installer.CheckSelfUpdate(ctx, http.DefaultClient, installer.WorkshopVersion, time.Now())
	a.mu.Lock()
	a.update = update
	a.mu.Unlock()
	return update
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
	a.mu.Unlock()
	path, err := installer.ApplySelfUpdate(ctx, http.DefaultClient, update)
	if errors.Is(err, installer.ErrBadSignature) {
		return "signature"
	}
	if err != nil {
		return err.Error()
	}
	if err := exec.Command(path).Start(); err != nil {
		return err.Error()
	}
	runtime.Quit(a.ctx)
	return ""
}

// ReleasesPage — страница выпусков Мастерской.
func (a *App) ReleasesPage() { runtime.BrowserOpenURL(a.ctx, installer.ReleasesPage) }

// OpenLink открывает ссылку первого запуска в браузере человека.
func (a *App) OpenLink(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

// CopyText кладёт текст в буфер обмена.
func (a *App) CopyText(text string) bool {
	return runtime.ClipboardSetText(a.ctx, text) == nil
}

// Version — номер версии Мастерской для полосы заголовка (своя рамка окна).
func (a *App) Version() string { return installer.WorkshopVersion }

// Disconnect рвёт подключение и забывает пароль из памяти.
func (a *App) Disconnect(id string) error {
	_, err := profileCall(a, id, false, func(w *installer.Wizard) bool { w.Close(); return true })
	return err
}

func main() {
	factory := func() *installer.Wizard {
		return &installer.Wizard{
			Dialer: devDialer(),
			Vault:  keyringVault{},
			// Версия Wynd, которую Мастерская знает со сборки; ставит она свежую
			// из хранилища, эта — когда хранилище не ответило.
			Version: version.Number,
			Image:   imageSource(),
			Source:  installer.SourceImage{Repo: installer.DefaultSource},
			// В собранном приложении всегда ложно.
			TestCert: devTestCert(),
		}
	}
	config, err := os.UserConfigDir()
	var workshop *installer.Workshop
	if err == nil {
		workshop, err = installer.OpenWorkshop(filepath.Join(config, "Wynd", "workshop", "servers.json"), factory)
	}
	if err != nil {
		err = fmt.Errorf("Не удалось открыть список серверов: %w", err)
	}
	app := &App{workshop: workshop, profileError: err}
	err = wails.Run(&options.App{
		// Своя версия Мастерской, не Wynd (план 45, 2026-10-24).
		Title:       "Wynd Мастерская [" + installer.WorkshopVersion + "]",
		Width:       760,
		Height:      560,
		MinWidth:    680,
		MinHeight:   480,
		AssetServer: &assetserver.Options{Assets: assets},
		// Цвет бумаги (--paper): окно не мигает белым, пока грузится страница.
		BackgroundColour: &options.RGBA{R: 0xF4, G: 0xF0, B: 0xE9, A: 0xFF},
		// Своя рамка, как на макете (frontend/src/Window.svelte): системный
		// заголовок на Windows 10 в цвет не перекрашивается. Название окна
		// выше остаётся — его видно на панели задач.
		Frameless:  true,
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind:       []any{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
