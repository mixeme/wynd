//go:build linux

package desktop

import "os"

// SelfPath — файл, который запустил человек. У AppImage это сам .AppImage:
// os.Executable() внутри указывает на временный смонтированный бинарник
// (/tmp/.mount_…/usr/bin/wynd-desktop), который исчезает после выхода.
// По этому пути регистрируется схема wynd:, пишется автозапуск и
// подменяется обновление.
func SelfPath() string {
	if path := os.Getenv("APPIMAGE"); path != "" {
		return path
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}
