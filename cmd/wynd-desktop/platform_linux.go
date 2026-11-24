//go:build linux && (dev || desktop || bindings)

package main

import (
	_ "embed"
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed build/appicon.png
var trayIcon []byte

// platformWindowsOptions — на Linux настроек WebView2 нет.
func platformWindowsOptions(string) application.WindowsOptions { return application.WindowsOptions{} }

// preparePlatformProfile — на Linux профиль WebKitGTK общий, чистить нечего.
func preparePlatformProfile(string) {}

// openFolderInFileManager открывает папку системным файловым менеджером.
func openFolderInFileManager(folder string) error {
	app := application.Get()
	if app == nil {
		return errors.New("desktop: приложение не запущено")
	}
	return app.Env.OpenFileManager(folder, false)
}

// newTrayMenuWindow — на Linux меню трея нативное, второго окна нет.
func newTrayMenuWindow(*application.App) application.Window { return nil }

// showTrayMenu — на Linux меню показывает StatusNotifierItem.
func (a *App) showTrayMenu() {}

// setupPlatformTray добавляет нативное меню значка.
func setupPlatformTray(_ *application.App, tray *application.SystemTray, win application.Window, shell *App) {
	tray.SetLabel("Wynd")
	menu := application.NewMenu()
	menu.Add("Открыть").OnClick(func(*application.Context) {
		application.InvokeSync(shell.TrayOpen)
	})
	menu.Add("Выход").OnClick(func(*application.Context) {
		application.InvokeSync(shell.TrayQuit)
	})
	tray.SetMenu(menu)
	tray.OnClick(func() {
		win.Show()
		win.Focus()
	})
}

// platformCanHideToTray — на Linux скрытие в трей включим после проверки
// реального StatusNotifier-хоста; до тех пор крестик закрывает окно.
func platformCanHideToTray(*application.SystemTray) bool { return false }

// platformCanStartHidden — до проверки трея автозапуск показывает окно.
func platformCanStartHidden() bool { return false }
