//go:build linux

package desktop

import "github.com/godbus/dbus/v5"

// EnsureToastIdentity на Linux ничего не регистрирует: имя приложения и
// значок берёт демон уведомлений из .desktop/темы.
func EnsureToastIdentity(string, []byte) error { return nil }

// ShowToast показывает системное уведомление через
// org.freedesktop.Notifications. Без сессионного D-Bus или демона
// уведомлений возвращает ошибку — вызывающий решает, писать ли её в журнал.
func ShowToast(title, body string) error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	call := obj.Call(
		"org.freedesktop.Notifications.Notify", 0,
		"Wynd", uint32(0), "wynd",
		title, body,
		[]string{}, map[string]dbus.Variant{},
		int32(5000),
	)
	return call.Err
}
