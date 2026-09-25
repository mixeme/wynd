package config

import (
	"net"
	"net/url"
	"strings"
)

// IsLoopback reports whether publicURL points at this machine.
func IsLoopback(publicURL string) bool {
	u, err := url.Parse(publicURL)
	if err != nil {
		return false
	}
	host := strings.Trim(u.Hostname(), "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
