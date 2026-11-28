package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/netaddr"
)

var moveTokenRe = regexp.MustCompile(`^[A-Za-z0-9-]{8,64}$`)

// Parse the prefix as a URL path, never as concatenated URL syntax.
func parseMoveLink(link string) (base, token string, ok bool) {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery {
		return "", "", false
	}
	prefix, token, found := strings.Cut(u.Path, "/move/")
	if !found || !moveTokenRe.MatchString(token) || strings.ContainsAny(prefix, "?#\\") {
		return "", "", false
	}
	for _, part := range strings.Split(prefix, "/") {
		if part == "." || part == ".." {
			return "", "", false
		}
	}
	u.Path, u.RawPath = strings.TrimRight(prefix, "/"), ""
	return u.String(), token, true
}

func moveAddressAllowed(addr netip.Addr, allowed []netip.Prefix) bool {
	if !addr.IsValid() || addr.Zone() != "" {
		return false
	}
	addr = addr.Unmap()
	for _, prefix := range allowed {
		if prefix.Contains(addr) {
			return true
		}
	}
	return netaddr.IsPublic(addr)
}

type moveLookup func(context.Context, string, string) ([]netip.Addr, error)
type moveDial func(context.Context, string, string) (net.Conn, error)

func guardedMoveDial(allowed []netip.Prefix, lookup moveLookup, dial moveDial) moveDial {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := lookup(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("move: source has no addresses")
		}
		// Reject the entire DNS answer before any connection, including mixed answers.
		for _, ip := range ips {
			if !moveAddressAllowed(ip, allowed) {
				return nil, fmt.Errorf("move: source address is not allowed")
			}
		}
		var last error
		for _, ip := range ips {
			conn, err := dial(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			last = err
		}
		return nil, last
	}
}

func (s *Server) moveClient() *http.Client {
	s.moveClientOnce.Do(func() {
		if s.MoveClient != nil {
			// Explicit transport injection is a trusted embedding/test hook.
			copy := *s.MoveClient
			s.moveHTTP = &copy
		} else {
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.Proxy = nil // Environment proxies must not bypass address checks.
			dialer := &net.Dialer{Timeout: 20 * time.Second, KeepAlive: 30 * time.Second}
			transport.DialContext = guardedMoveDial(s.MoveAllowedNetworks, net.DefaultResolver.LookupNetIP, dialer.DialContext)
			s.moveHTTP = &http.Client{Transport: transport}
		}
		s.moveHTTP.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	})
	return s.moveHTTP
}
