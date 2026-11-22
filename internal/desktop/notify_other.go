//go:build !windows

package desktop

// Системные уведомления живут в notify_windows.go. Окно собирается только
// под Windows (desktop-reference, «Границы»); заглушки нужны, чтобы
// серверные тесты на Linux собирались без реестра и PowerShell.

// EnsureToastIdentity — заглушка: вне Windows реестра и AUMID нет.
func EnsureToastIdentity(dataDir string, iconICO []byte) error { return nil }

// ShowToast — заглушка: вне Windows всплывающих уведомлений нет.
func ShowToast(title, body string) error { return nil }
