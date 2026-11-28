package config

import (
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"gitea.mixdep.ru/mix/wynd/internal/netaddr"
)

var dnsLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// NormalizePushServer accepts an HTTPS origin. DNS and TLS are checked by the
// Workshop when it installs the notification server.
func NormalizePushServer(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil ||
		u.ForceQuery || u.RawQuery != "" || strings.Contains(raw, "#") ||
		(u.EscapedPath() != "" && u.EscapedPath() != "/") || strings.HasSuffix(u.Host, ":") {
		return "", false
	}

	host := strings.ToLower(u.Hostname())
	bracketed := strings.HasPrefix(u.Host, "[")
	ip, ipErr := netip.ParseAddr(host)
	if ipErr == nil {
		if bracketed != ip.Is6() || !netaddr.IsPublic(ip) {
			return "", false
		}
		host = ip.String()
	} else {
		host = strings.TrimSuffix(host, ".")
		if bracketed || !publicDNSName(host) {
			return "", false
		}
	}

	port := u.Port()
	if port != "" {
		number, err := strconv.ParseUint(port, 10, 16)
		if err != nil || number == 0 {
			return "", false
		}
		if number != 443 {
			return "https://" + net.JoinHostPort(host, strconv.FormatUint(number, 10)), true
		}
	}
	if ip.Is6() {
		host = "[" + host + "]"
	}
	return "https://" + host, true
}

func publicDNSName(host string) bool {
	labels := strings.Split(host, ".")
	last := labels[len(labels)-1]
	if len(host) > 253 || len(labels) < 2 || last == "" || last[0] < 'a' || last[0] > 'z' {
		return false
	}
	switch last {
	case "alt", "example", "internal", "invalid", "local", "localhost", "onion", "test":
		return false
	case "arpa":
		return false
	}
	for _, label := range labels {
		if !dnsLabel.MatchString(label) {
			return false
		}
	}
	return true
}
