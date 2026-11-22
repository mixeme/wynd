package desktop

import (
	"net/url"
	"strings"
)

// ParseInviteURL разбирает адрес схемы wynd: (план 66, функция 4), которым
// браузер передаёт приглашение окну: wynd:open?url=<полный https-адрес
// приглашения, закодированный>. Форма wynd:open?url= выбрана вместо
// wynd:https://… потому что шелл Windows при запуске приводит адрес к нижнему
// регистру и может перекодировать проценты в «прозрачной» части URI —
// параметр со значением в query надёжнее. Возврат — https-адрес приглашения.
func ParseInviteURL(arg string) (string, bool) {
	if len(arg) < 5 || !strings.EqualFold(arg[:5], "wynd:") {
		return "", false
	}
	rest := arg[5:]
	// Допускаем и «//open?url=…» — варианты записи шелла.
	rest = strings.TrimPrefix(rest, "//")
	if !strings.HasPrefix(strings.ToLower(rest), "open?") {
		return "", false
	}
	query, err := url.ParseQuery(rest[len("open?"):])
	if err != nil {
		return "", false
	}
	raw := query.Get("url")
	if raw == "" {
		return "", false
	}
	target, err := url.Parse(raw)
	if err != nil || target.Scheme != "https" || target.Host == "" {
		return "", false
	}
	if target.Path == "" {
		target.Path = "/"
	}
	return target.String(), true
}
