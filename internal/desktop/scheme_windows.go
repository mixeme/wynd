//go:build windows

package desktop

import (
	"errors"
	"os"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

// Схема wynd: (план 66, функция 4): браузер передаёт приглашение окну через
// wynd:open?url=… Запись делается в HKCU текущего пользователя — по прецеденту
// автозапуска, без машинных прав; переживает обновления окна, потому что
// команда — путь этого exe, переписывается при каждом запуске.

const (
	schemeKey = `Software\Classes\wynd`
	// schemeArgumentPlaceholder — то, что шелл заменяет адресом приглашения.
	schemeArgumentPlaceholder = "%1"
)

// EnsureInviteScheme регистрирует схему wynd: за этим exe. Повторный запуск
// переписывает команду; сбой (реестр закрыт политикой) не мешает окну —
// уведомление пишется в журнал вызывающим.
func EnsureInviteScheme() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, schemeKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer func() { _ = key.Close() }()
	if err := key.SetStringValue("URL Protocol", ""); err != nil {
		return err
	}
	if err := key.SetStringValue("", "URL:Wynd"); err != nil {
		return err
	}
	command := `"` + exe + `" "` + schemeArgumentPlaceholder + `"`
	cmdKey, _, err := registry.CreateKey(registry.CURRENT_USER, schemeKey+`\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer func() { _ = cmdKey.Close() }()
	if err := cmdKey.SetStringValue("", command); err != nil {
		return err
	}
	return nil
}

// InviteSchemeCommand читает записанную команду: пусто — схемы нет.
func InviteSchemeCommand() (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, schemeKey+`\shell\open\command`, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
			return "", nil
		}
		return "", err
	}
	defer func() { _ = key.Close() }()
	value, _, err := key.GetStringValue("")
	if err != nil {
		if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
			return "", nil
		}
		return "", err
	}
	return value, nil
}
