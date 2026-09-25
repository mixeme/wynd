package check_test

import (
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/check"
)

func domainResult(t *testing.T, in check.Input) check.Result {
	t.Helper()
	for _, r := range check.RunChecks(t.Context(), in) {
		if r.ID == "domain" {
			return r
		}
	}
	t.Fatal("нет строки domain")
	return check.Result{}
}

// Инвариант (план 42, CHK-1): сервер за NAT видит свой приватный адрес, и
// публичная A-запись с ним не совпадёт никогда — это warn «сверьте вручную»,
// а не fail «домен ведёт не туда». Публичный несовпавший адрес — по-прежнему
// fail. localhost разрешается из hosts, без сети.
func TestCheckDomainBehindNATIsWarnNotFail(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		egress string
		want   check.Status
	}{
		{"192.168.1.10", check.StatusWarn},
		{"10.0.0.5", check.StatusWarn},
		{"100.72.1.1", check.StatusWarn},
		{"203.0.113.7", check.StatusFail},
	} {
		r := domainResult(t, check.Input{
			PublicURL:      "https://localhost",
			ServerEgressIP: tc.egress,
			Now:            now,
		})
		if r.Status != tc.want {
			t.Fatalf("egress %s: %s %q, want %s", tc.egress, r.Status, r.Detail, tc.want)
		}
		if tc.want == check.StatusWarn && !strings.Contains(r.Detail, "NAT") {
			t.Fatalf("egress %s: в подсказке нет NAT: %q", tc.egress, r.Detail)
		}
	}
}
