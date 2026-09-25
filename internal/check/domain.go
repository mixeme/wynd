package check

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// publicEndpoint разбирает public_url один раз для всех проб: хост без
// скобок IPv6, порт (по умолчанию 443 для https и 80 для http), признак
// https и явного нестандартного порта. Раньше три пробы разбирали адрес
// по-своему: IPv6 ломался, TLS всегда шёл на 443, а HTTP-проба стучалась в
// HTTPS-порт (план 42, CHK-4).
type publicEndpoint struct {
	Host        string
	Port        string
	HTTPS       bool
	DefaultPort bool
}

func parsePublicURL(publicURL string) (publicEndpoint, bool) {
	u, err := url.Parse(strings.TrimSpace(publicURL))
	if err != nil || u.Hostname() == "" {
		return publicEndpoint{}, false
	}
	ep := publicEndpoint{Host: u.Hostname(), Port: u.Port(), HTTPS: strings.EqualFold(u.Scheme, "https")}
	def := "80"
	if ep.HTTPS {
		def = "443"
	}
	if ep.Port == "" || ep.Port == def {
		ep.Port = def
		ep.DefaultPort = true
	}
	return ep, true
}

func publicHost(publicURL string) string {
	ep, _ := parsePublicURL(publicURL)
	return ep.Host
}

func serverEgressIP(ctx context.Context) string {
	d := net.Dialer{Timeout: 3 * time.Second}
	conn, err := d.DialContext(ctx, "udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP == nil {
		return ""
	}
	return addr.IP.String()
}

func checkDomain(ctx context.Context, in Input) Result {
	r := Result{ID: "domain", Title: "Домен"}
	if in.Loopback {
		r.Status = StatusNA
		r.Detail = "на loopback не применимо"
		return r
	}
	host := publicHost(in.PublicURL)
	if host == "" {
		r.Status = StatusWarn
		r.Detail = "public_url не задан"
		return r
	}
	egress := in.ServerEgressIP
	if egress == "" {
		egress = serverEgressIP(ctx)
	}
	if ip := net.ParseIP(host); ip != nil {
		if egress != "" && ip.String() == egress {
			r.Status = StatusOK
			r.Detail = fmt.Sprintf("%s — адрес этого сервера", host)
			return r
		}
		r.Status = StatusWarn
		if behindNAT(egress) {
			r.Detail = fmt.Sprintf("%s; сервер за NAT (%s) — сверьте с внешним адресом вручную", host, egress)
			return r
		}
		r.Detail = fmt.Sprintf("%s не совпадает с адресом сервера %s", host, egress)
		return r
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		r.Status = StatusFail
		r.Detail = fmt.Sprintf("%s не резолвится", host)
		return r
	}
	if egress == "" {
		r.Status = StatusWarn
		r.Detail = fmt.Sprintf("%s → %s; не удалось определить адрес сервера", host, ips[0].IP)
		return r
	}
	for _, rec := range ips {
		if rec.IP.String() == egress {
			r.Status = StatusOK
			r.Detail = fmt.Sprintf("%s ведёт на %s — адрес этого сервера", host, egress)
			return r
		}
	}
	// Адрес исходящего сокета за NAT — приватный: с публичной A-записью он не
	// совпадёт никогда, и проверка краснела у каждого домашнего инстанса
	// (план 42, CHK-1). Доказать по нему ничего нельзя — только предупредить.
	if behindNAT(egress) {
		r.Status = StatusWarn
		r.Detail = fmt.Sprintf("%s ведёт на %s; сервер за NAT (%s) — сверьте с внешним адресом вручную", host, ips[0].IP, egress)
		return r
	}
	r.Status = StatusFail
	r.Detail = fmt.Sprintf("%s ведёт на %s, а не на %s", host, ips[0].IP, egress)
	return r
}

// behindNAT — адрес не маршрутизируется из интернета: сервер выходит наружу
// через NAT, и свой публичный адрес по сокету не узнать.
func behindNAT(egress string) bool {
	ip := net.ParseIP(egress)
	if ip == nil {
		return false
	}
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || isCGNAT(ip)
}

// isCGNAT — 100.64.0.0/10, адреса провайдерского NAT; IsPrivate их не знает.
func isCGNAT(ip net.IP) bool {
	v4 := ip.To4()
	return v4 != nil && v4[0] == 100 && v4[1]&0xC0 == 64
}
