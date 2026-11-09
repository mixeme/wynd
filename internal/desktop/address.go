// Package desktop — то, что оболочка Wynd для компьютера делает без окна:
// адрес сервера, его проверка и список добавленных серверов. Здесь нет Wails,
// чтобы правила проверялись обычными тестами.
package desktop

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrInvalidAddress — адрес не годится: пустой, с путём, параметрами или
// чужой схемой. Одна ошибка на все случаи: человеку показывается один текст.
var ErrInvalidAddress = errors.New("invalid server address")

// NormalizeAddress приводит введённый адрес к виду https://host[:port].
// Без схемы считается https. Путь, параметры, якорь и логин в адресе сервера
// не нужны: сервер живёт в корне своего адреса.
func NormalizeAddress(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", ErrInvalidAddress
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" || u.User != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidAddress, err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", ErrInvalidAddress
	}
	if u.Path != "" && u.Path != "/" {
		return "", ErrInvalidAddress
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", ErrInvalidAddress
	}
	host := strings.ToLower(u.Host)
	if strings.ContainsAny(host, " \t\r\n") {
		return "", ErrInvalidAddress
	}
	return scheme + "://" + host, nil
}

// HostOnly — адрес без схемы для подписи и путей прокси.
func HostOnly(origin string) string {
	if i := strings.Index(origin, "://"); i >= 0 {
		return origin[i+3:]
	}
	return origin
}
