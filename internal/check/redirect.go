package check

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"
)

// ProbeHTTPRedirect returns the HTTP status of a permanent HTTP→HTTPS redirect (301 or 308).
// Other outcomes return 0.
//
// Проба идёт на порт 80 хоста из public_url. При нестандартном HTTPS-порту
// проверка неприменима и не выполняется: HTTP-запрос в HTTPS-порт давал
// ложное «не перенаправляет» (план 42, CHK-4).
func ProbeHTTPRedirect(ctx context.Context, publicURL string) int {
	ep, ok := parsePublicURL(publicURL)
	if !ok || !ep.HTTPS || !ep.DefaultPort {
		return 0
	}
	return probeHTTPRedirectAt(ctx, net.JoinHostPort(ep.Host, "80"))
}

// RedirectApplies reports whether the HTTP→HTTPS row makes sense for publicURL.
func RedirectApplies(publicURL string) bool {
	ep, ok := parsePublicURL(publicURL)
	return ok && ep.HTTPS && ep.DefaultPort
}

func probeHTTPRedirectAt(ctx context.Context, hostport string) int {
	target := "http://" + hostport + "/api/v1/instance"
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMovedPermanently && resp.StatusCode != http.StatusPermanentRedirect {
		return 0
	}
	// Постоянный редирект куда угодно — ещё не HTTPS: заглушка хостера или
	// редирект на http того же хоста проходили проверку (CHK-5).
	if !strings.HasPrefix(strings.ToLower(resp.Header.Get("Location")), "https://") {
		return 0
	}
	return resp.StatusCode
}
