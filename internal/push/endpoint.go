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
	dialTimeout    = 5 * time.Second
	// Имя, которое не резолвится, не должно держать запрос: отличить
	// временный сбой DNS от подделки всё равно нельзя.
	resolveTimeout = 2 * time.Second
)

// newDeliveryClient — свой клиент вместо http.DefaultClient: сервер ходит по
// адресу, который назвал участник, поэтому ни бесконечного ожидания, ни
// переходов по редиректам быть не должно (SEC-4).
//
// DialContext резолвит хост сам и отвергает непубличные адреса на каждой
// доставке: проверка при подписке смотрит DNS один раз, а имя можно
// перевести на 127.0.0.1 после неё (DNS rebinding, аудит 2026-09-22, SEC-7).
// Прокси намеренно не берётся из окружения — через него проверка адреса
// ничего не значила бы.
func newDeliveryClient(allowPrivate bool) *http.Client {
	dialer := &net.Dialer{Timeout: dialTimeout}
	dial := dialPublicOnly(dialer)
	if allowPrivate {
		dial = dialer.DialContext
	}
	return &http.Client{
		Timeout: deliverTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           dial,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          32,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
	}
}

// dialPublicOnly резолвит имя сам и соединяется только с публичным адресом.
// Имя, у которого хотя бы один адрес непубличный, не набирается вовсе:
// иначе порядок в ответе DNS решал бы, куда уйдёт запрос.
func dialPublicOnly(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if ip := net.ParseIP(host); ip != nil {
			if !isPublicIP(ip) {
				return nil, ErrInvalid
			}
			return dialer.DialContext(ctx, network, addr)
		}
		lookupCtx, cancel := context.WithTimeout(ctx, resolveTimeout)
		defer cancel()
		addrs, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
		if err != nil {
			return nil, err
		}
		if len(addrs) == 0 {
			return nil, ErrInvalid
		}
		for _, a := range addrs {
			if !isPublicIP(a.IP) {
				return nil, ErrInvalid
			}
		}
		var lastErr error
		for _, a := range addrs {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(a.IP.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		return nil, lastErr
	}
}

// endpointHost — хост без пути: путь endpoint равносилен ключу от устройства,
// в лог он попадать не должен (аудит 2026-09-22, SEC-7).
func endpointHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "?"
	}
	return u.Host
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
