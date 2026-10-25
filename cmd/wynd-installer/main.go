//go:build dev || desktop || bindings

// Настольный установщик Wynd (план 45). Собирается через `wails build` или
// `wails dev`: без их меток сборки пакет пуст, и ворота основного сервера
// (go vet, go test ./...) его не трогают. Вся логика — в internal/installer.
package main

import (
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"

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
	ctx    context.Context
	wizard *installer.Wizard
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

func (a *App) shutdown(context.Context) { a.wizard.Close() }

// HasSavedPassword — лежит ли в связке ключей пароль для этого сервера.
func (a *App) HasSavedPassword(host, user string) bool {
	return a.wizard.HasSavedPassword(host, user)
}

// Connect подключается к серверу; незнакомый сервер вернёт отпечаток.
func (a *App) Connect(in installer.ConnectInput) installer.ConnectResult {
	// Порт окно не спрашивает и задать не может.
	in.Port = devPort()
	if in.Password == "" && !in.UseKeys {
		in.Password = devPassword()
	}
	return a.wizard.Connect(a.ctx, in)
}

// ConfirmHost — отпечаток сверен: запомнить сервер и войти.
func (a *App) ConfirmHost() installer.ConnectResult {
	return a.wizard.ConfirmHost(a.ctx)
}

// Inspect осматривает сервер, ничего не меняя.
func (a *App) Inspect() installer.InspectResult {
	return a.wizard.Inspect(a.ctx)
}

// CheckDomain проверяет, указывает ли адрес сайта на этот сервер.
func (a *App) CheckDomain(domain string) installer.DomainCheck {
	return a.wizard.CheckDomain(a.ctx, domain)
}

// MakePlan — что будет сделано на сервере, до первого изменения.
func (a *App) MakePlan(domain string) installer.Plan {
	return a.wizard.MakePlan(a.ctx, domain)
}

// ChooseBuild переключает способ: собрать Wynd на сервере или скачать готовый.
func (a *App) ChooseBuild(on bool) installer.Plan {
	return a.wizard.ChooseBuild(on)
}

// StartInstall запускает установку (или повторяет её с места отказа).
func (a *App) StartInstall() installer.Progress {
	return a.wizard.StartInstall(a.ctx)
}

// InstallProgress — как идёт установка; окно спрашивает раз в полсекунды.
func (a *App) InstallProgress() installer.Progress {
	return a.wizard.InstallProgress()
}

// Rollback убирает поставленное; keepData — оставить данные Wynd на сервере.
func (a *App) Rollback(keepData bool) installer.RollbackResult {
	return a.wizard.Rollback(a.ctx, keepData)
}

// ServerPage — страница сервера, где Wynd уже стоит (пульт, кадры 7а–7е).
func (a *App) ServerPage() installer.ServerPage {
	return a.wizard.ServerPage(a.ctx)
}

// Service — «Запустить», «Остановить», «Перезапустить».
func (a *App) Service(action string) installer.ServiceResult {
	return a.wizard.Service(a.ctx, action)
}

// StartUpdate обновляет Wynd; ход — InstallProgress.
func (a *App) StartUpdate(opt installer.UpdateOptions) installer.Progress {
	return a.wizard.StartUpdate(a.ctx, opt)
}

// PlanBackup — сколько скачать в папку folder (8а).
func (a *App) PlanBackup(folder string) installer.LocalBackupPlan {
	return a.wizard.PlanBackup(a.ctx, folder)
}

// StartBackup — бэкап на этот компьютер; ход — InstallProgress (8б).
func (a *App) StartBackup(opt installer.BackupOptions) installer.Progress {
	return a.wizard.StartBackup(a.ctx, opt)
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
func (a *App) ReturnPrevious() installer.ServiceResult {
	return a.wizard.ReturnPrevious(a.ctx)
}

// MoveConnect — подключение к новому серверу (9а).
func (a *App) MoveConnect(in installer.ConnectInput) installer.ConnectResult {
	return a.wizard.MoveConnect(a.ctx, in)
}

// MoveConfirmHost — отпечаток нового сервера сверен.
func (a *App) MoveConfirmHost() installer.ConnectResult {
	return a.wizard.MoveConfirmHost(a.ctx)
}

// MovePlan — план переезда (9б).
func (a *App) MovePlan() installer.MovePlan { return a.wizard.MovePlan(a.ctx) }

// StartMove — «Переехать»; ход — InstallProgress (9в).
func (a *App) StartMove(removeOld bool) installer.Progress {
	return a.wizard.StartMove(a.ctx, removeOld)
}

// MoveCheckNow — «Проверить сейчас» (9в).
func (a *App) MoveCheckNow() { a.wizard.MoveCheckNow() }

// CancelMove — «Отменить переезд» (9в).
func (a *App) CancelMove() installer.ServiceResult { return a.wizard.CancelMove(a.ctx) }

// MoveAdopt — после переезда — страница нового сервера (9г).
func (a *App) MoveAdopt() installer.ServerPage { return a.wizard.MoveAdopt(a.ctx) }

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
func (a *App) Disconnect() {
	a.wizard.Close()
}

func main() {
	app := &App{wizard: &installer.Wizard{
		Dialer:  devDialer(),
		Vault:   keyringVault{},
		// Версия Wynd, которую Мастерская знает со сборки; ставит она свежую
		// из хранилища, эта — когда хранилище не ответило.
		Version: version.Number,
		Image:   imageSource(),
		Source:  installer.SourceImage{Repo: installer.DefaultSource},
		// В собранном приложении всегда ложно.
		TestCert: devTestCert(),
	}}
	err := wails.Run(&options.App{
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
		Frameless: true,
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []any{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
