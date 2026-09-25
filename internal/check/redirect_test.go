package check_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/check"
)

func TestProbeHTTPRedirectSkipsNonHTTPS(t *testing.T) {
	if check.ProbeHTTPRedirect("http://example.org") != 0 {
		t.Fatal("http public URL should not probe")
	}
	if check.ProbeHTTPRedirect("") != 0 {
		t.Fatal("empty public URL should not probe")
	}
}

func TestProbeHTTPRedirectPermanent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/instance" {
			t.Errorf("path %s", r.URL.Path)
		}
		w.Header().Set("Location", "https://example.org/api/v1/instance")
		w.WriteHeader(http.StatusPermanentRedirect)
	}))
	defer srv.Close()
	public := "https://" + hostOf(t, srv.URL)
	if got := check.ProbeHTTPRedirect(public); got != http.StatusPermanentRedirect {
		t.Fatalf("308 from HTTP: got %d", got)
	}
}

func TestProbeHTTPRedirectMovedPermanently(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://example.org/api/v1/instance")
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer srv.Close()
	public := "https://" + hostOf(t, srv.URL)
	if got := check.ProbeHTTPRedirect(public); got != http.StatusMovedPermanently {
		t.Fatalf("301 from HTTP: got %d", got)
	}
}

func TestProbeHTTPRedirectTemporaryIgnored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://example.org/api/v1/instance")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()
	public := "https://" + hostOf(t, srv.URL)
	if check.ProbeHTTPRedirect(public) != 0 {
		t.Fatal("302 must not count as permanent")
	}
}

func hostOf(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}
