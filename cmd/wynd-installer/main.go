//go:build dev || desktop || bindings

// Настольный установщик Wynd (план 45). Собирается через `wails build` или
// `wails dev`: без их меток сборки пакет пуст, и ворота основного сервера
// (go vet, go test ./...) его не трогают. Вся логика — в internal/installer.
package main

import (
	"context"
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"gitea.mixdep.ru/mix/wynd/internal/installer"
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

// Disconnect рвёт подключение и забывает пароль из памяти.
func (a *App) Disconnect() {
	a.wizard.Close()
}

func main() {
	app := &App{wizard: &installer.Wizard{Dialer: devDialer(), Vault: keyringVault{}}}
	err := wails.Run(&options.App{
		Title:       "Wynd — установка на свой сервер",
		Width:       760,
		Height:      560,
		MinWidth:    680,
		MinHeight:   480,
		AssetServer: &assetserver.Options{Assets: assets},
		// Цвет бумаги (--paper): окно не мигает белым, пока грузится страница.
		BackgroundColour: &options.RGBA{R: 0xF4, G: 0xF0, B: 0xE9, A: 0xFF},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []any{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
