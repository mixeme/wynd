package push

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SubscriberMailto is the VAPID `sub` claim. Строгие push-сервисы отвечают 400
// на голый адрес без схемы, и подписка молча не работает (AUTH-5).
const SubscriberMailto = "mailto:admin@wynd.local"

const (
	deliverTimeout = 10 * time.Second
	// Имя, которое не резолвится, не должно держать запрос: отличить
	// временный сбой DNS от подделки всё равно нельзя.
	resolveTimeout = 2 * time.Second
)

// newDeliveryClient — свой клиент вместо http.DefaultClient: сервер ходит по
// адресу, который назвал участник, поэтому ни бесконечного ожидания, ни
// переходов по редиректам быть не должно (SEC-4).
func newDeliveryClient() *http.Client {
	return &http.Client{
		Timeout: deliverTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// validateEndpoint отвергает адреса, по которым сервер не должен ходить:
// не https и всё, что резолвится во внутреннюю сеть. Без проверки endpoint
// подписки был однобитным оракулом сканирования сети для любого участника:
// 404/410 удаляет подписку, остальное — нет.
//
// Если имя не резолвится, подписка принимается: отличить временный сбой DNS
// от подделки нельзя, а список разрешённых push-сервисов решением раздела B
// не вводится.
func validateEndpoint(ctx context.Context, raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return ErrInvalid
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return ErrInvalid
	}
	host := u.Hostname()
	if host == "" {
		return ErrInvalid
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return ErrInvalid
		}
		return nil
	}
	lookupCtx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
	if err != nil || len(addrs) == 0 {
		return nil
	}
	for _, a := range addrs {
		if !isPublicIP(a.IP) {
			return ErrInvalid
		}
	}
	return nil
}

func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return false
	}
	// 100.64.0.0/10 (CGNAT) и 0.0.0.0/8 не покрыты IsPrivate.
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return false
		}
		if v4[0] == 0 {
			return false
		}
	}
	return true
}
