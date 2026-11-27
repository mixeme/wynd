//go:build linux

package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// AutostartStatus — что записано в XDG-файле и совпадает ли команда с этим
// бинарником.
type AutostartStatus struct {
	Enabled bool   `json:"enabled"`
	Stale   bool   `json:"stale"`
	Command string `json:"command"`
}

// autostartFile — XDG-файл автозапуска текущего пользователя.
func autostartFile() (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(config, "autostart", "wynd.desktop"), nil
}

// AutostartCommand — команда запуска без окна: путь файла приложения в
// кавычках и отметка --from-startup. У AppImage это сам .AppImage, а не
// временный бинарник внутри смонтированного образа.
func AutostartCommand() (string, error) {
	exe := SelfPath()
	if exe == "" {
		return "", os.ErrNotExist
	}
	return autostartExec(exe), nil
}

func autostartExec(exe string) string {
	return `"` + strings.ReplaceAll(exe, `"`, `\"`) + `" --from-startup`
}

func autostartStale(stored, current string) bool {
	return stored != "" && stored != current
}

// AutostartState читает XDG-файл; отсутствие — выключенный автозапуск.
func AutostartState() (AutostartStatus, error) {
	current, err := AutostartCommand()
	if err != nil {
		return AutostartStatus{}, err
	}
	path, err := autostartFile()
	if err != nil {
		return AutostartStatus{}, err
	}
	body, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return AutostartStatus{}, nil
	}
	if err != nil {
		return AutostartStatus{}, err
	}
	for _, line := range strings.Split(string(body), "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "Exec="); ok {
			value = strings.TrimSpace(value)
			return AutostartStatus{Enabled: true, Stale: autostartStale(value, current), Command: value}, nil
		}
	}
	return AutostartStatus{Enabled: true}, nil
}

// SetAutostart включает или выключает XDG-автозапуск. После записи состояние
// читается обратно: подтверждение — прочитанное значение.
func SetAutostart(enabled bool) (AutostartStatus, error) {
	path, err := autostartFile()
	if err != nil {
		return AutostartStatus{}, err
	}
	if !enabled {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return AutostartStatus{}, err
		}
		return AutostartState()
	}
	current, err := AutostartCommand()
	if err != nil {
		return AutostartStatus{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return AutostartStatus{}, err
	}
	entry := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=Wynd\n" +
		"Comment=Wynd Настольный\n" +
		"Exec=" + current + "\n" +
		"Terminal=false\n" +
		"X-GNOME-Autostart-enabled=true\n"
	if err := os.WriteFile(path, []byte(entry), 0o644); err != nil {
		return AutostartStatus{}, err
	}
	return AutostartState()
}
