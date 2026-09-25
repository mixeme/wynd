package check

import (
	"net/http"
	"strings"
	"time"
)

// ProbeHTTPRedirect checks whether plain HTTP redirects to HTTPS with 301 or 308.
func ProbeHTTPRedirect(publicURL string) bool {
	if !strings.HasPrefix(publicURL, "https://") {
		return false
	}
	host := strings.TrimPrefix(publicURL, "https://")
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	if host == "" {
		return false
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
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusMovedPermanently || resp.StatusCode == http.StatusPermanentRedirect
}
