//go:build windows

package desktop

import (
	"errors"
	"os"
	"strconv"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const (
	autostartRunKey = `Software\Microsoft\Windows\CurrentVersion\Run`
	autostartValue  = "Wynd"
)

// AutostartStatus — что записано в Run и совпадает ли команда с этим exe.
type AutostartStatus struct {
	Enabled bool   `json:"enabled"`
	Stale   bool   `json:"stale"`
	Command string `json:"command"`
}

// AutostartCommand — точный путь exe в кавычках и отметка запуска. Кавычки
// обязательны: путь с пробелами иначе режется на аргументы.
func AutostartCommand() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return strconv.Quote(exe) + " --from-startup", nil
}

func autostartStale(stored, current string) bool {
	return stored != "" && stored != current
}

// AutostartState читает запись текущего пользователя. Её отсутствие — не
// ошибка, а выключенный автозапуск.
func AutostartState() (AutostartStatus, error) {
	current, err := AutostartCommand()
	if err != nil {
		return AutostartStatus{}, err
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, autostartRunKey, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
			return AutostartStatus{}, nil
		}
		return AutostartStatus{}, err
	}
	defer func() { _ = key.Close() }()
	stored, _, err := key.GetStringValue(autostartValue)
	if err != nil {
		if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
			return AutostartStatus{}, nil
		}
		return AutostartStatus{}, err
	}
	return AutostartStatus{Enabled: true, Stale: autostartStale(stored, current), Command: stored}, nil
}

// SetAutostart включает или выключает запись. После записи состояние читается
// обратно: подтверждение — прочитанное значение, а не намерение.
func SetAutostart(enabled bool) (AutostartStatus, error) {
	current, err := AutostartCommand()
	if err != nil {
		return AutostartStatus{}, err
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, autostartRunKey, registry.SET_VALUE)
	if err != nil {
		return AutostartStatus{}, err
	}
	defer func() { _ = key.Close() }()
	if enabled {
		if err := key.SetStringValue(autostartValue, current); err != nil {
			return AutostartStatus{}, err
		}
	} else if err := key.DeleteValue(autostartValue); err != nil && !errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
		return AutostartStatus{}, err
	}
	return AutostartState()
}
