//go:build linux

package desktop

import (
	"os"
	"path/filepath"
)

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

// selfPathTarget — путь для подмены файла обновлением: у AppImage снимаем
// символические ссылки, чтобы заменить настоящий файл, а не ссылку на него.
func selfPathTarget() string {
	path := SelfPath()
	if path == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}
