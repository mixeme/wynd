package check_test

import (
	"context"
	"path/filepath"
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
		MailConfigured:  true,
		SMTPTestSentAt:  &smtpSent,
		VAPIDConfigured: true,
		LastRoutineAt:   &routineAt,
		LastBackupAt:    &backupAt,
		Now:             now,
	})
	if len(results) != 11 {
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
	if byID["smtp"].Status != check.StatusOK {
		t.Fatalf("smtp: %+v", byID["smtp"])
	}
	if byID["dkim"].Status != check.StatusNA {
		t.Fatalf("dkim: %+v", byID["dkim"])
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
	for _, id := range []string{"https_outside", "proxy_headers", "proxy_body_limit", "proxy_sse"} {
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
	}
	results := check.RunChecks(context.Background(), check.Input{
		Loopback:        false,
		PublicURL:       "https://home.example.org",
		DataDir:         filepath.Join(t.TempDir(), "data"),
		MailConfigured:  false,
		VAPIDConfigured: false,
		External:        ext,
		Now:             now,
	})
	byID := map[string]check.Result{}
	for _, r := range results {
		byID[r.ID] = r
	}
	if byID["smtp"].Status != check.StatusWarn {
		t.Fatalf("smtp: %+v", byID["smtp"])
	}
	if byID["https_outside"].Status != check.StatusOK {
		t.Fatalf("https_outside: %+v", byID["https_outside"])
	}
	if byID["proxy_headers"].Status != check.StatusOK {
		t.Fatalf("proxy_headers: %+v", byID["proxy_headers"])
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
	if byID["smtp"].Status != check.StatusWarn {
		t.Fatalf("smtp on loopback should still warn when unset: %+v", byID["smtp"])
	}
}
