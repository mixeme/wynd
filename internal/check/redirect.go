package check

import (
	"net/http"
	"strings"
	"time"
)

// ProbeHTTPRedirect returns the HTTP status of a permanent HTTP→HTTPS redirect (301 or 308).
// Other outcomes return 0.
func ProbeHTTPRedirect(publicURL string) int {
	if !strings.HasPrefix(publicURL, "https://") {
		return 0
	}
	host := strings.TrimPrefix(publicURL, "https://")
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	if host == "" {
		return 0
	}
	target := "http://" + host + "/api/v1/instance"
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(target)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusMovedPermanently || resp.StatusCode == http.StatusPermanentRedirect {
		return resp.StatusCode
	}
	return 0
}
