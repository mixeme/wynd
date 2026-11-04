package installer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/releasepages"
)

type releasePageTransport func(*http.Request) (*http.Response, error)

func (f releasePageTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSelfUpdateReleasePages(t *testing.T) {
	previous := SigningKey
	SigningKey = "test"
	t.Cleanup(func() { SigningKey = previous })
	workshop := `{"tag_name":"workshop-v0.3.1","published_at":"2026-10-01T00:00:00Z","assets":[{"name":"wynd-workshop.exe","browser_download_url":"https://example.invalid/a.exe"},{"name":"wynd-workshop.exe.sig","browser_download_url":"https://example.invalid/a.sig"}]}`
	servers := "[" + strings.Repeat(`{"tag_name":"v0.48.1"},`, 99) + `{"tag_name":"v0.48.0"}]`
	now := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	for _, mode := range []string{"next page", "next page failure", "limit", "bad JSON"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: releasePageTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				if r.URL.Host != "api.github.com" || r.URL.Query().Get("per_page") != "100" {
					t.Fatalf("unexpected endpoint %s", r.URL)
				}
				body := servers
				h := make(http.Header)
				h.Set("Link", `<http://127.0.0.1/private>; rel="next"`)
				status := 200
				if mode == "limit" {
					body = "[" + workshop + "]"
				} else if page == 2 {
					h.Del("Link")
					body = "[" + workshop + "]"
					if mode == "next page failure" {
						status = 503
					}
					if mode == "bad JSON" {
						body = "bad JSON"
					}
				}
				return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			update := CheckSelfUpdate(t.Context(), client, "0.3.0", now)
			if mode == "next page" {
				if !update.Available || update.Version != "0.3.1" || calls != 2 {
					t.Fatalf("update=%+v calls=%d", update, calls)
				}
			} else {
				if update.Available {
					t.Fatalf("partial update advertised: %+v", update)
				}
				want := 2
				if mode == "limit" {
					want = releasepages.Max
				}
				if calls != want {
					t.Fatalf("calls=%d want=%d", calls, want)
				}
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := selfUpdateReleases(ctx, &http.Client{Transport: releasePageTransport(func(*http.Request) (*http.Response, error) {
		t.Fatal("request after cancellation")
		return nil, fmt.Errorf("cancelled")
	})}); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestSelfUpdateStillRequiresPublishedSignedRelease(t *testing.T) {
	old := time.Now().Add(-7 * 24 * time.Hour)
	for i, r := range []ghRelease{
		{TagName: "workshop-v9.0.0", PublishedAt: old, Prerelease: true},
		{TagName: "workshop-v9.0.0", PublishedAt: old, Draft: true},
		{TagName: "workshop-v9.0.0", PublishedAt: old},
	} {
		if i < 2 {
			r.Assets = []struct {
				Name string `json:"name"`
				URL  string `json:"browser_download_url"`
			}{
				{Name: "wynd-workshop.exe", URL: "https://example.invalid/a.exe"},
				{Name: "wynd-workshop.exe.sig", URL: "https://example.invalid/a.sig"},
			}
		}
		if up := pickRelease([]ghRelease{r}, "0.3.0", time.Now()); up.Available {
			t.Fatalf("unsafe release advertised: %+v", r)
		}
	}
}
