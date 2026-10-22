package check_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/check"
)

func kinds(ts []check.Trouble) []check.TroubleKind {
	out := make([]check.TroubleKind, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.Kind)
	}
	return out
}

func sameKinds(a, b []check.TroubleKind) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Всё в порядке — писать не о чем.
func TestTroublesNoneWhenHealthy(t *testing.T) {
	now := time.Date(2026, 10, 22, 6, 0, 0, 0, time.UTC)
	routine := now.Add(-3 * time.Hour)
	backup := now.Add(-24 * time.Hour)
	got := check.Troubles(check.TroubleInput{
		UsedBytes: 10 << 30, LimitBytes: 40 << 30,
		DiskFree: 20 << 30, DiskTotal: 50 << 30,
		TLS:           &check.TLSInfo{NotBefore: now.Add(-10 * 24 * time.Hour), NotAfter: now.Add(80 * 24 * time.Hour)},
		LastBackupAt:  &backup,
		LastRoutineAt: &routine,
		Now:           now,
	})
	if len(got) != 0 {
		t.Fatalf("беды у здорового сервера: %+v", got)
	}
}

// Пороги — как в «Проверке»; порядок — срочность, а не порядок проверок.
func TestTroublesThresholdsAndOrder(t *testing.T) {
	now := time.Date(2026, 10, 22, 6, 0, 0, 0, time.UTC)
	routine := now.Add(-40 * time.Hour)
	backup := now.Add(-8 * 24 * time.Hour)
	got := check.Troubles(check.TroubleInput{
		UsedBytes: 37 << 30, LimitBytes: 40 << 30,
		DiskFree: 800 << 20, DiskTotal: 50 << 30,
		// Сертификат на 90 дней, осталось 12: продление должно было пройти.
		TLS:           &check.TLSInfo{NotBefore: now.Add(-78 * 24 * time.Hour), NotAfter: now.Add(12 * 24 * time.Hour)},
		LastBackupAt:  &backup,
		LastRoutineAt: &routine,
		Now:           now,
	})
	want := []check.TroubleKind{check.TroubleDiskLow, check.TroubleCertNotRenewed, check.TroubleServerNearlyFull, check.TroubleBackupOld, check.TroubleRoutineStale}
	if !sameKinds(kinds(got), want) {
		t.Fatalf("беды %v, ждали %v", kinds(got), want)
	}
	if got[0].Used != 800<<20 || got[0].Total != 50<<30 {
		t.Fatalf("диск: %+v", got[0])
	}
}

func TestTroublesWorstCases(t *testing.T) {
	now := time.Date(2026, 10, 22, 6, 0, 0, 0, time.UTC)
	routine := now.Add(-time.Hour)
	got := check.Troubles(check.TroubleInput{
		UsedBytes: 40 << 30, LimitBytes: 40 << 30,
		TLS:           &check.TLSInfo{NotAfter: now.Add(-24 * time.Hour), Verify: check.VerifyExpired},
		LastRoutineAt: &routine,
		Now:           now,
	})
	want := []check.TroubleKind{check.TroubleCertExpired, check.TroubleServerFull, check.TroubleBackupMissing}
	if !sameKinds(kinds(got), want) {
		t.Fatalf("беды %v, ждали %v", kinds(got), want)
	}
}

// Ровно 90% — ещё не беда, как на шкале «Хранилища» (порог — черта).
// Непрочитанный сертификат, диск и предел — тоже не беды для письма.
func TestTroublesBoundaryAndUnknowns(t *testing.T) {
	now := time.Date(2026, 10, 22, 6, 0, 0, 0, time.UTC)
	backup := now.Add(-time.Hour)
	got := check.Troubles(check.TroubleInput{
		UsedBytes: 9, LimitBytes: 10,
		TLS:          &check.TLSInfo{ProbeError: "dial tcp: timeout"},
		LastBackupAt: &backup,
		Now:          now,
	})
	if len(got) != 0 {
		t.Fatalf("беды: %+v", got)
	}
	if got := check.Troubles(check.TroubleInput{UsedBytes: 5, LimitBytes: 0, LastBackupAt: &backup, Now: now}); len(got) != 0 {
		t.Fatalf("без предела: %+v", got)
	}
}

func TestServerLimitRow(t *testing.T) {
	now := time.Date(2026, 10, 22, 6, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		used, limit int64
		want        check.Status
	}{
		{10 << 30, 40 << 30, check.StatusOK},
		{37 << 30, 40 << 30, check.StatusWarn},
		{40 << 30, 40 << 30, check.StatusFail},
	} {
		var row check.Result
		for _, r := range check.RunChecks(t.Context(), check.Input{Loopback: true, UsedBytes: c.used, LimitBytes: c.limit, Now: now}) {
			if r.ID == "server_limit" {
				row = r
			}
		}
		if row.Status != c.want || row.Title != "Предел сервера" {
			t.Fatalf("%d из %d: %+v", c.used, c.limit, row)
		}
	}
	for _, r := range check.RunChecks(t.Context(), check.Input{Loopback: true, UsedBytes: 12 << 10, LimitBytes: 40 << 30, Now: now}) {
		if r.ID == "server_limit" && r.Detail != "занято 12 КБ из 40 ГБ" {
			t.Fatalf("подпись: %q", r.Detail)
		}
	}
}
