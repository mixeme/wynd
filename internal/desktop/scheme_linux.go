//go:build linux

package desktop

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// schemeFileName — пользовательский обработчик x-scheme-handler/wynd.
const schemeFileName = "wynd.desktop"

func xdgDataHome() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}

func schemeDesktopPath() (string, error) {
	data, err := xdgDataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(data, "applications", schemeFileName), nil
}

func schemeDesktopEntry(exe string) string {
	command := `"` + strings.ReplaceAll(exe, `"`, `\"`) + `" %u`
	return "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=Wynd\n" +
		"Comment=Wynd Настольный\n" +
		"Exec=" + command + "\n" +
		"Icon=wynd\n" +
		"Terminal=false\n" +
		"NoDisplay=true\n" +
		"MimeType=x-scheme-handler/wynd;\n" +
		"Categories=Network;\n"
}

// EnsureInviteScheme регистрирует wynd: на текущего пользователя: файл в
// ~/.local/share/applications и xdg-mime. Ошибки xdg-mime не мешают окну.
func EnsureInviteScheme() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	path, err := schemeDesktopPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(schemeDesktopEntry(exe)), 0o644); err != nil {
		return err
	}
	_ = exec.Command("xdg-mime", "default", schemeFileName, "x-scheme-handler/wynd").Run()
	_ = exec.Command("update-desktop-database", filepath.Dir(path)).Run()
	return nil
}

// InviteSchemeCommand читает зарегистрированную команду обработчика.
func InviteSchemeCommand() (string, error) {
	path, err := schemeDesktopPath()
	if err != nil {
		return "", err
	}
	body, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(body), "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "Exec="); ok {
			return strings.TrimSpace(value), nil
		}
	}
	return "", nil
}
