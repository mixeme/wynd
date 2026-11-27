//go:build !linux

package desktop

import "os"

// SelfPath — файл программы: на Windows и других платформах это сам exe,
// отдельного образа вокруг нет.
func SelfPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}
