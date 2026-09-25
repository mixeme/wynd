package check_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/check"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

func TestRunChecksLoopbackDefaults(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	routineAt := now.Add(-2 * time.Hour)
	backupAt := now.Add(-24 * time.Hour)
	smtpSent := now.Add(-10 * time.Minute)

	results := check.RunChecks(context.Background(), check.Input{
		Loopback:        true,
		PublicURL:       "http://127.0.0.1:7676",
		DataDir:         t.TempDir(),
		SMTPTestSentAt:  &smtpSent,
		VAPIDConfigured: true,
		LastRoutineAt:   &routineAt,
		LastBackupAt:    &backupAt,
		Now:             now,
	})
	if len(results) != 17 {
		t.Fatalf("result count: %d", len(results))
	}

	byID := map[string]check.Result{}
	for _, r := range results {
		byID[r.ID] = r
	}

	if byID["clocks"].Status != check.StatusOK {
		t.Fatalf("clocks: %+v", byID["clocks"])
	}
	if byID["disk_space"].Status != check.StatusOK {
		t.Fatalf("disk_space: %+v", byID["disk_space"])
	}
	if byID["mail"].Status != check.StatusOK {
		t.Fatalf("mail: %+v", byID["mail"])
	}
	if byID["mail"].Detail != "ушло 10 мин. назад" {
		t.Fatalf("mail detail: %+v", byID["mail"])
	}
	if byID["vapid_keys"].Status != check.StatusOK {
		t.Fatalf("vapid_keys: %+v", byID["vapid_keys"])
	}
	if byID["daily_routine"].Status != check.StatusOK {
		t.Fatalf("daily_routine: %+v", byID["daily_routine"])
	}
	if byID["backup"].Status != check.StatusOK {
		t.Fatalf("backup: %+v", byID["backup"])
	}
	for _, id := range []string{
		"domain", "https_outside", "http_redirect", "cert_le", "cert_chain", "pwa",
		"proxy_client", "proxy_headers", "proxy_body_limit", "proxy_sse", "proxy_timeout",
	} {
		if byID[id].Status != check.StatusNA {
			t.Fatalf("%s: %+v", id, byID[id])
		}
	}
}

func TestRunChecksPublicWithExternal(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	ext := &check.ExternalReport{
		HTTPSOK:           true,
		HTTPSMS:           180,
		RedirectPermanent: true,
		FromOutside:       true,
		ProxyHTTPS:        true,
		ProxyBodyLimitOK:  true,
		ProxySSEOK:        true,
		ProxyStreamOK:     true,
		ProxyReadTimeout:  300,
		PWAOK:             true,
		XForwardedFor:     "203.0.113.50",
		ClientIP:          "203.0.113.50",
	}
	results := check.RunChecks(context.Background(), check.Input{
		Loopback:        false,
		PublicURL:       "https://home.example.org",
		DataDir:         filepath.Join(t.TempDir(), "data"),
		VAPIDConfigured: false,
		External:        ext,
		TLS: &check.TLSInfo{
			NotAfter:  now.Add(84 * 24 * time.Hour),
			Issuer:    "R3",
			FullChain: true,
		},
		Now: now,
	})
	byID := map[string]check.Result{}
	for _, r := range results {
		byID[r.ID] = r
	}
	if byID["mail"].Status != check.StatusWarn {
		t.Fatalf("mail: %+v", byID["mail"])
	}
	if byID["https_outside"].Status != check.StatusOK {
		t.Fatalf("https_outside: %+v", byID["https_outside"])
	}
	if byID["http_redirect"].Detail != "308, постоянный" {
		t.Fatalf("http_redirect: %+v", byID["http_redirect"])
	}
	if byID["cert_le"].Status != check.StatusOK {
		t.Fatalf("cert_le: %+v", byID["cert_le"])
	}
	if byID["proxy_headers"].Status != check.StatusOK {
		t.Fatalf("proxy_headers: %+v", byID["proxy_headers"])
	}
	if byID["proxy_client"].Status != check.StatusOK {
		t.Fatalf("proxy_client: %+v", byID["proxy_client"])
	}
}

func TestRunChecksStaleRoutineAndBackup(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	old := now.Add(-10 * 24 * time.Hour)
	results := check.RunChecks(context.Background(), check.Input{
		Loopback:      true,
		DataDir:       t.TempDir(),
		LastRoutineAt: &old,
		LastBackupAt:  &old,
		Now:           now,
	})
	byID := map[string]check.Result{}
	for _, r := range results {
		byID[r.ID] = r
	}
	if byID["daily_routine"].Status != check.StatusWarn {
		t.Fatalf("daily_routine: %+v", byID["daily_routine"])
	}
	if byID["backup"].Status != check.StatusWarn {
		t.Fatalf("backup: %+v", byID["backup"])
	}
	if byID["mail"].Status != check.StatusWarn {
		t.Fatalf("mail on loopback should warn when never sent: %+v", byID["mail"])
	}
}

func TestRunChecksMailLastError(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	sent := now.Add(-2 * time.Minute)
	results := check.RunChecks(context.Background(), check.Input{
		Loopback:       true,
		DataDir:        t.TempDir(),
		SMTPTestSentAt: &sent,
		SMTPLastError:  "dial tcp: i/o timeout",
		Now:            now,
	})
	byID := map[string]check.Result{}
	for _, r := range results {
		byID[r.ID] = r
	}
	if byID["mail"].Status != check.StatusFail {
		t.Fatalf("mail: %+v", byID["mail"])
	}
	if !strings.Contains(byID["mail"].Detail, "не ушло") {
		t.Fatalf("mail detail: %+v", byID["mail"])
	}
}

func TestCheckCertLERecognizesCurrentIntermediates(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	results := check.RunChecks(context.Background(), check.Input{
		Loopback:  false,
		PublicURL: "https://home.example.org",
		DataDir:   t.TempDir(),
		TLS: &check.TLSInfo{
			NotAfter:  now.Add(84 * 24 * time.Hour),
			Issuer:    "R10",
			IssuerOrg: "Let's Encrypt",
			FullChain: true,
		},
		Now: now,
	})
	byID := map[string]check.Result{}
	for _, r := range results {
		byID[r.ID] = r
	}
	if byID["cert_le"].Status != check.StatusOK {
		t.Fatalf("R10: %+v", byID["cert_le"])
	}
}

func TestCheckCertLEExpiredUsesRussianDate(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	results := check.RunChecks(context.Background(), check.Input{
		Loopback:  false,
		PublicURL: "https://home.example.org",
		DataDir:   t.TempDir(),
		TLS: &check.TLSInfo{
			NotAfter: time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC),
			Issuer:   "R10",
		},
		Now: now.Add(90 * 24 * time.Hour),
	})
	byID := map[string]check.Result{}
	for _, r := range results {
		byID[r.ID] = r
	}
	if byID["cert_le"].Status != check.StatusFail {
		t.Fatalf("expired status: %+v", byID["cert_le"])
	}
	if byID["cert_le"].Detail != "истёк 12 ноября" {
		t.Fatalf("expired detail: %+v", byID["cert_le"])
	}
}
