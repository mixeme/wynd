//go:build windows && (dev || desktop || bindings)

package main

import (
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/w32"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

// Размер меню в трее (C66): ширина по тексту «Открыть» с полями 12+12.
const (
	trayMenuWidth  = 92
	trayMenuHeight = 56
)

// platformWindowsOptions — настройки WebView2: постоянный профиль и отключение
// троттлинга скрытого окна.
func platformWindowsOptions(base string) application.WindowsOptions {
	return application.WindowsOptions{
		WebviewUserDataPath: filepath.Join(base, "webview"),
		AdditionalBrowserArgs: []string{
			"--disable-background-timer-throttling",
			"--disable-backgrounding-occluded-windows",
			"--disable-renderer-backgrounding",
		},
	}
}

// preparePlatformProfile убирает служебный воркер PWA старого WebView2.
func preparePlatformProfile(base string) {
	_ = os.RemoveAll(filepath.Join(base, "webview", "EBWebView", "Default", "Service Worker"))
}

// openFolderInFileManager открывает папку в Проводнике.
func openFolderInFileManager(folder string) error {
	return exec.Command("explorer.exe", folder).Start()
}

// newTrayMenuWindow — второе маленькое окно меню в стиле рамки (C66).
func newTrayMenuWindow(app *application.App) application.Window {
	menu := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "tray-menu",
		Width:            trayMenuWidth,
		Height:           trayMenuHeight,
		AlwaysOnTop:      true,
		Frameless:        true,
		Hidden:           true,
		DisableResize:    true,
		BackgroundColour: application.NewRGB(0xF4, 0xF0, 0xE9),
		Windows: application.WindowsWindow{
			HiddenOnTaskbar:                   true,
			DisableFramelessWindowDecorations: true,
		},
		URL: "/?tray-menu=1",
	})
	menu.RegisterHook(events.Common.WindowLostFocus, func(*application.WindowEvent) {
		menu.Hide()
	})
	return menu
}

// showTrayMenu показывает меню у курсора — правый щелчок по значку.
func (a *App) showTrayMenu() {
	a.mu.Lock()
	menu := a.trayMenu
	a.mu.Unlock()
	if menu == nil {
		return
	}
	x, y, ok := w32.GetCursorPos()
	if !ok {
		return
	}
	menu.SetSize(trayMenuWidth, trayMenuHeight)
	menu.SetPosition(x, y)
	menu.Show()
	menu.Focus()
}

// setupPlatformTray добавляет Windows-поведение значка.
func setupPlatformTray(_ *application.App, tray *application.SystemTray, win application.Window, shell *App) {
	tray.OnRightClick(func() { application.InvokeSync(shell.showTrayMenu) })
	tray.OnClick(func() {
		win.Show()
		win.Focus()
	})
}

// platformCanHideToTray — на Windows окно можно спрятать в значок.
func platformCanHideToTray(tray *application.SystemTray) bool { return tray != nil }

// platformCanStartHidden — автозапуск Windows стартует в значке.
func platformCanStartHidden() bool { return true }
