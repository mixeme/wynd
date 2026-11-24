//go:build !windows && !linux

package desktop

import "errors"

// Системные уведомления живут в notify_windows.go и notify_linux.go. На
// остальных платформах окно не собирается; заглушки не выдают ложный успех.

// EnsureToastIdentity — заглушка: вне Windows реестра и AUMID нет.
func EnsureToastIdentity(dataDir string, iconICO []byte) error { return nil }

// ShowToast — на неподдерживаемой платформе уведомлений нет.
func ShowToast(title, body string) error {
	return errors.New("desktop: системные уведомления не поддерживаются")
}
