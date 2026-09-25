package check_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/check"
)

func TestProbeHTTPRedirectSkipsNonHTTPS(t *testing.T) {
	if check.ProbeHTTPRedirect(t.Context(), "http://example.org") != 0 {
		t.Fatal("http public URL should not probe")
	}
	if check.ProbeHTTPRedirect(t.Context(), "") != 0 {
		t.Fatal("empty public URL should not probe")
	}
}

// Инвариант (план 42, CHK-4): при HTTPS на нестандартном порту строка
// «HTTP → HTTPS» неприменима — проба не стучится HTTP-запросом в HTTPS-порт.
func TestProbeHTTPRedirectSkipsNonDefaultPort(t *testing.T) {
	if check.RedirectApplies("https://home.example.org:8443") {
		t.Fatal("non-default port must not apply")
	}
	if !check.RedirectApplies("https://home.example.org") || !check.RedirectApplies("https://home.example.org:443/") {
		t.Fatal("default port must apply")
	}
	if check.ProbeHTTPRedirect(t.Context(), "https://127.0.0.1:1") != 0 {
		t.Fatal("non-default port must not probe")
	}
}

func redirectServer(t *testing.T, status int, location string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/instance" {
			t.Errorf("path %s", r.URL.Path)
		}
		w.Header().Set("Location", location)
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}

func TestProbeHTTPRedirectPermanent(t *testing.T) {
	host := redirectServer(t, http.StatusPermanentRedirect, "https://example.org/api/v1/instance")
	if got := check.ProbeHTTPRedirectAt(t.Context(), host); got != http.StatusPermanentRedirect {
		t.Fatalf("308 from HTTP: got %d", got)
	}
}

func TestProbeHTTPRedirectMovedPermanently(t *testing.T) {
	host := redirectServer(t, http.StatusMovedPermanently, "https://example.org/api/v1/instance")
	if got := check.ProbeHTTPRedirectAt(t.Context(), host); got != http.StatusMovedPermanently {
		t.Fatalf("301 from HTTP: got %d", got)
	}
}

func TestProbeHTTPRedirectTemporaryIgnored(t *testing.T) {
	host := redirectServer(t, http.StatusFound, "https://example.org/api/v1/instance")
	if check.ProbeHTTPRedirectAt(t.Context(), host) != 0 {
		t.Fatal("302 must not count as permanent")
	}
}

// Инвариант (CHK-5): постоянный редирект не на https — не перенаправление на
// HTTPS (заглушка хостера, http того же хоста).
func TestProbeHTTPRedirectToHTTPIgnored(t *testing.T) {
	host := redirectServer(t, http.StatusMovedPermanently, "http://example.org/parking")
	if check.ProbeHTTPRedirectAt(t.Context(), host) != 0 {
		t.Fatal("301 to http must not count")
	}
}

// Инвариант (CHK-4): один разбор public_url — IPv6 без скобок, порт по
// схеме, явный стандартный порт считается стандартным.
func TestParsePublicURL(t *testing.T) {
	for _, tc := range []struct {
		in          string
		host, port  string
		https, dflt bool
	}{
		{"https://home.example.org", "home.example.org", "443", true, true},
		{"https://home.example.org:443/", "home.example.org", "443", true, true},
		{"https://home.example.org:8443", "home.example.org", "8443", true, false},
		{"http://10.0.0.5:7676", "10.0.0.5", "7676", false, false},
		{"https://[2001:db8::1]:8443", "2001:db8::1", "8443", true, false},
		{"https://[2001:db8::1]", "2001:db8::1", "443", true, true},
	} {
		host, port, https, dflt, ok := check.ParsePublicURL(tc.in)
		if !ok || host != tc.host || port != tc.port || https != tc.https || dflt != tc.dflt {
			t.Fatalf("%s: %q %q %v %v %v", tc.in, host, port, https, dflt, ok)
		}
	}
	if _, _, _, _, ok := check.ParsePublicURL("home.example.org"); ok {
		t.Fatal("URL без схемы не разбирается")
	}
}
