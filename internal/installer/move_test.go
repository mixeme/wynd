package installer

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestMoveStepOrder(t *testing.T) {
	var ids []string
	for _, s := range MoveTitles(MoveSpec{Version: "0.43.2", Image: RegistryImage{Repo: DefaultRegistry}}) {
		ids = append(ids, s.ID)
	}
	if got := strings.Join(ids, ","); got != "install,stop,transfer,dns,start,old" {
		t.Fatalf("шаги: %s", got)
	}
}

// 9б2: на старом Wynd не со своим Caddy — на новом ставим и Caddy, везём одни
// данные; порядок шагов тот же.
func TestMoveDataOnlyTitles(t *testing.T) {
	var got []string
	for _, s := range MoveTitles(MoveSpec{Version: "0.43.2", Image: RegistryImage{Repo: DefaultRegistry}, DataOnly: true}) {
		got = append(got, s.ID+"="+s.Title)
	}
	joined := strings.Join(got, "; ")
	for _, want := range []string{"install=Wynd 0.43.2 и Caddy на новом сервере", "transfer=Данные Wynd перенесены"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("нет %q в %s", want, joined)
		}
	}
	if !strings.Contains(dataTar, "--exclude=data/backups") || !strings.HasSuffix(dataTar, " data") {
		t.Fatalf("архив данных: %s", dataTar)
	}
}

// 9в: ждём, пока адрес укажет на новый сервер; «Проверить сейчас» — сразу.
func TestDNSStepWaitsForNewAddress(t *testing.T) {
	answers := [][]string{{"198.51.100.4"}, {"198.51.100.4", "203.0.113.7"}, {"203.0.113.7"}}
	calls := 0
	check := make(chan struct{}, 3)
	spec := MoveSpec{Domain: "family.example.ru", NewIP: "203.0.113.7", CheckNow: check,
		Resolve: func(context.Context, string) []string {
			a := answers[min(calls, len(answers)-1)]
			calls++
			return a
		}}
	old := dnsWait
	dnsWait = time.Hour
	defer func() { dnsWait = old }()
	for range 3 {
		check <- struct{}{}
	}
	r := &Run{steps: []StepState{{ID: "dns"}}, report: func() {}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := dnsStep(spec).do(ctx, r); err != nil {
		t.Fatal(err)
	}
	if calls != 3 || !strings.Contains(r.steps[0].Note, "198.51.100.4") {
		t.Fatalf("проверок %d, подпись %q", calls, r.steps[0].Note)
	}
}
