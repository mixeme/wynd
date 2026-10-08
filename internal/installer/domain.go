package installer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// DomainCheck — указывает ли адрес будущего сайта на этот сервер.
type DomainCheck struct {
	Domain string `json:"domain"`
	Level  Level  `json:"level"`
	Text   string `json:"text"`
	Advice string `json:"advice,omitempty"`
}

// Resolver — то, что нужно от DNS; в тестах подставной.
type Resolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
}

// NormalizeDomain приводит введённое к имени сайта: человек вставляет и
// «https://family.example.ru/», и с пробелами. Пустая строка — не имя.
func NormalizeDomain(raw string) string {
	d := strings.ToLower(strings.TrimSpace(raw))
	d = strings.TrimPrefix(d, "https://")
	d = strings.TrimPrefix(d, "http://")
	d, _, _ = strings.Cut(d, "/")
	d = strings.TrimSuffix(d, ".")
	if d == "" || len(d) > 253 || !strings.Contains(d, ".") || net.ParseIP(d) != nil {
		return ""
	}
	for _, label := range strings.Split(d, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return ""
		}
		for _, r := range label {
			ok := r == '-' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || r > 127
			if !ok {
				return ""
			}
		}
	}
	return d
}

// CheckDomain сверяет адреса, на которые указывает имя, с адресами сервера.
// serverAddrs — адрес, по которому подключились, и адреса из осмотра.
func CheckDomain(ctx context.Context, r Resolver, raw string, serverAddrs []string) DomainCheck {
	domain := NormalizeDomain(raw)
	if domain == "" {
		return DomainCheck{
			Domain: strings.TrimSpace(raw), Level: LevelBlock,
			Text:   "Это не похоже на адрес сайта",
			Advice: "Нужно имя вида family.example.ru — его покупают у регистратора доменов. По голому адресу из цифр сертификат не выдадут.",
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	found, err := r.LookupHost(ctx, domain)
	target := firstPublic(serverAddrs)
	advice := fmt.Sprintf("У регистратора домена добавьте запись типа A с адресом %s. Новая запись расходится по интернету до часа — потом нажмите «Проверить снова».", target)
	// Имя не нашлось — дело в домене. Любой другой отказ — в связи этого
	// компьютера со службой имён: винить домен тогда нельзя.
	var dnsErr *net.DNSError
	if err != nil && !(errors.As(err, &dnsErr) && dnsErr.IsNotFound) && !isNotFoundText(err) {
		return DomainCheck{
			Domain: domain, Level: LevelBlock,
			Text:   "Не удалось проверить адрес сайта",
			Advice: "Этот компьютер сейчас не может узнавать адреса сайтов — так бывает без интернета или при включённом VPN. Проверьте связь и нажмите «Проверить снова».",
		}
	}
	if err != nil || len(found) == 0 {
		return DomainCheck{
			Domain: domain, Level: LevelBlock,
			Text:   domain + " пока никуда не указывает",
			Advice: advice,
		}
	}
	mine := map[string]bool{}
	for _, a := range serverAddrs {
		if ip := net.ParseIP(strings.TrimSpace(a)); ip != nil {
			mine[ip.String()] = true
		}
	}
	for _, a := range found {
		if ip := net.ParseIP(a); ip != nil && mine[ip.String()] {
			return DomainCheck{Domain: domain, Level: LevelOK, Text: "указывает на этот сервер"}
		}
	}
	return DomainCheck{
		Domain: domain, Level: LevelBlock,
		Text:   fmt.Sprintf("%s ещё не указывает на этот сервер", domain),
		Advice: fmt.Sprintf("Сейчас он ведёт на %s. ", found[0]) + advice,
	}
}

// firstPublic — адрес сервера для совета: внешний, если он есть среди известных.
func firstPublic(addrs []string) string {
	fallback := ""
	for _, a := range addrs {
		ip := net.ParseIP(strings.TrimSpace(a))
		if ip == nil {
			continue
		}
		if fallback == "" {
			fallback = ip.String()
		}
		if !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
			return ip.String()
		}
	}
	if fallback == "" {
		return "вашего сервера"
	}
	return fallback
}

// isNotFoundText — «такого имени нет» от резолвера, который не размечает
// ошибку (подставной в тестах, часть системных на Windows).
func isNotFoundText(err error) bool {
	return strings.Contains(err.Error(), "no such host")
}
