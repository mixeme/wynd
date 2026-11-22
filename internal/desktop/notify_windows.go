//go:build windows

package desktop

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"

	"github.com/go-toast/toast"
	"golang.org/x/sys/windows/registry"
)

// ToastAppID — AUMID всплывающих уведомлений Windows. Без записи в
// HKCU\Software\Classes\AppUserModelId уведомление показывалось бы от имени
// PowerShell: скрипт уведомления запускает именно он. AUMID тот же, что у
// SingleInstance (com.wynd.desktop).
const ToastAppID = "com.wynd.desktop"

const toastDisplayName = "Wynd Настольный"

// EnsureToastIdentity регистрирует AUMID окна: имя «Wynd Настольный» и значок
// (крупнейший PNG-слой icon.ico) рядом с данными окна. Права — только HKCU
// текущего пользователя, как у записи автозапуска. Повторный запуск
// переписывает значения; сбой значка не мешает имени — уведомление без
// значка всё равно показывается.
func EnsureToastIdentity(dataDir string, iconICO []byte) error {
	iconPath := ""
	if png, err := extractIconPNG(iconICO); err == nil {
		iconPath = filepath.Join(dataDir, "toast-icon.png")
		if err := writeFileIfChanged(iconPath, png); err != nil {
			iconPath = ""
		}
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\AppUserModelId\`+ToastAppID, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer func() { _ = key.Close() }()
	if err := key.SetStringValue("DisplayName", toastDisplayName); err != nil {
		return err
	}
	if iconPath != "" {
		if err := key.SetStringValue("IconUri", iconPath); err != nil {
			return err
		}
	}
	return nil
}

// ShowToast показывает всплывающее уведомление от имени Wynd. Правило «на
// виду — молча» применяет мост (App.Notify через ShouldNotify), сам механизм
// показывает всегда — им пользуется и живая проверка.
func ShowToast(title, body string) error {
	n := toast.Notification{
		AppID:   ToastAppID,
		Title:   sanitizeToastText(title),
		Message: sanitizeToastText(body),
		Icon:    toastIconPath(),
	}
	return n.Push()
}

// toastIconPath — путь значка из записи AUMID; без записи значка у
// уведомления не будет, имя останется.
func toastIconPath() string {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Classes\AppUserModelId\`+ToastAppID, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer func() { _ = key.Close() }()
	path, _, err := key.GetStringValue("IconUri")
	if err != nil {
		return ""
	}
	return path
}

// extractIconPNG достаёт самый крупный PNG-слой из .ico: слои значка хранятся
// как PNG, его можно показать в уведомлении как есть, без конвертации.
func extractIconPNG(ico []byte) ([]byte, error) {
	if len(ico) < 6 || ico[0] != 0 || ico[1] != 0 || ico[2] != 1 || ico[3] != 0 {
		return nil, errors.New("toast: not an ICO file")
	}
	best := []byte(nil)
	for i, count := 0, int(binary.LittleEndian.Uint16(ico[4:6])); i < count; i++ {
		off := 6 + i*16
		if off+16 > len(ico) {
			break
		}
		size := int(binary.LittleEndian.Uint32(ico[off+8 : off+12]))
		imgOff := int(binary.LittleEndian.Uint32(ico[off+12 : off+16]))
		if size < 8 || imgOff+size > len(ico) || imgOff+size < imgOff {
			continue
		}
		layer := ico[imgOff : imgOff+size]
		if len(best) < size && layer[0] == 0x89 && layer[1] == 0x50 && layer[2] == 0x4E && layer[3] == 0x47 {
			best = layer
		}
	}
	if best == nil {
		return nil, errors.New("toast: icon has no PNG layer")
	}
	return best, nil
}

// writeFileIfChanged переписывает файл только при другом содержимом —
// значок не переписывается при каждом запуске зря.
func writeFileIfChanged(path string, data []byte) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		return nil
	}
	return os.WriteFile(path, data, 0o600)
}
