package main

import (
	"net"
	"strings"
)

// listensOnLoopback reports whether the listen address binds only the
// loopback interface. ":7676" and "0.0.0.0:7676" answer every interface.
func listensOnLoopback(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return false
	}
	host = strings.Trim(host, "[]")
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
