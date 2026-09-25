package check

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

func publicHost(publicURL string) string {
	u := strings.TrimSpace(publicURL)
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
	}
	if i := strings.Index(u, "/"); i >= 0 {
		u = u[:i]
	}
	if j := strings.LastIndex(u, ":"); j >= 0 && strings.Count(u, ":") == 1 {
		u = u[:j]
	}
	return u
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
	r.Status = StatusFail
	r.Detail = fmt.Sprintf("%s ведёт на %s, а не на %s", host, ips[0].IP, egress)
	return r
}
