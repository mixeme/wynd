package check

import "context"

// ProbeHTTPRedirectAt открывает тестам пробу по явному адресу: httptest
// слушает не порт 80.
func ProbeHTTPRedirectAt(ctx context.Context, hostport string) int {
	return probeHTTPRedirectAt(ctx, hostport)
}

// ParsePublicURL открывает тестам разбор public_url.
func ParsePublicURL(publicURL string) (host, port string, https, defaultPort, ok bool) {
	ep, ok := parsePublicURL(publicURL)
	return ep.Host, ep.Port, ep.HTTPS, ep.DefaultPort, ok
}
