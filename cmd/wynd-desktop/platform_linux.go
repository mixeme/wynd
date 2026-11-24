//go:build linux && (dev || desktop || bindings)

package main

import (
	_ "embed"
	"errors"
	"sync"

	"github.com/godbus/dbus/v5"
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

// trayAvailableOnce кэширует проверку StatusNotifierWatcher: без хоста
// значок не зарегистрируется, и прятать окно нельзя.
var (
	trayAvailableOnce sync.Once
	trayAvailable     bool
)

func linuxTrayAvailable() bool {
	trayAvailableOnce.Do(func() {
		conn, err := dbus.SessionBus()
		if err != nil {
			return
		}
		var has bool
		if err := conn.BusObject().Call(
			"org.freedesktop.DBus.NameHasOwner", 0,
			"org.kde.StatusNotifierWatcher",
		).Store(&has); err != nil {
			return
		}
		trayAvailable = has
	})
	return trayAvailable
}

// platformCanHideToTray — прятать окно можно только при живом хосте трея.
func platformCanHideToTray(tray *application.SystemTray) bool {
	return tray != nil && linuxTrayAvailable()
}

// platformCanStartHidden — автозапуск прячется только при живом трее.
func platformCanStartHidden() bool { return linuxTrayAvailable() }
