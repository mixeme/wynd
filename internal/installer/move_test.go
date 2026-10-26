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

// 9в2: пульт старого сервера видит незаконченный переезд по пометке.
func TestServerStateReadsMoveMark(t *testing.T) {
	out := "@image ghcr.io/mixeme/wynd:0.43.2\n@url https://family.example.ru\n@running \n@container abc\n" +
		`@move {"to":"203.0.113.7","started":"2026-10-26T12:28:00Z","remove_old":true,"transferred":true}` + "\n@free /dev/vda1 1 1 1 1% /\n"
	st := parseServerState(out)
	if st.Move == nil || st.Move.To != "203.0.113.7" || !st.Move.RemoveOld || !st.Move.Transferred || st.Move.Launched {
		t.Fatalf("пометка: %+v", st.Move)
	}
	if parseServerState(strings.Replace(out, "@move {", "@move x{", 1)).Move != nil {
		t.Fatal("испорченная пометка не должна читаться")
	}
}

// 9в2: «Продолжить переезд» не переносит данные второй раз — на новом могли
// уже писать; пометка сохраняет выбор «убрать старый».
func TestMoveResumeSkipsTransfer(t *testing.T) {
	spec := MoveSpec{Version: "0.43.2", Image: RegistryImage{Repo: DefaultRegistry}, NewIP: "203.0.113.7", RemoveOld: true,
		Resume: &MoveMark{To: "203.0.113.7", Started: "2026-10-26T12:28:00Z", Transferred: true}}
	steps := moveSteps(nil, spec)
	if steps[2].id != "transfer" {
		t.Fatalf("третий шаг: %s", steps[2].id)
	}
	r := &Run{steps: []StepState{{ID: "transfer"}}, report: func() {}}
	if err := steps[2].do(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if r.steps[0].Note != "перенесено раньше" || len(r.log) != 0 {
		t.Fatalf("подпись %q, команд %d", r.steps[0].Note, len(r.log))
	}
	cmd := markCmd(MoveMark{To: "203.0.113.7", Started: "2026-10-26T12:28:00Z", RemoveOld: true})
	if !strings.Contains(cmd, `"remove_old":true`) || !strings.HasSuffix(cmd, "> "+moveMark) {
		t.Fatalf("пометка: %s", cmd)
	}
}

// 9в и 9в3: строка шага называет старый сервер, а молчание службы имён —
// «не удалось узнать», а не «указывает на никуда».
func TestDNSStepNotes(t *testing.T) {
	old := dnsWait
	dnsWait = time.Hour
	defer func() { dnsWait = old }()
	for _, c := range []struct {
		answer []string
		want   string
	}{
		{nil, "пока не удалось узнать, куда указывает family.example.ru"},
		{[]string{"198.51.100.4"}, "family.example.ru пока указывает на старый сервер 198.51.100.4"},
		{[]string{"192.0.2.9"}, "family.example.ru пока указывает на 192.0.2.9"},
	} {
		ctx, cancel := context.WithCancel(context.Background())
		spec := MoveSpec{Domain: "family.example.ru", OldIP: "198.51.100.4", NewIP: "203.0.113.7",
			Resolve: func(context.Context, string) []string { cancel(); return c.answer }}
		r := &Run{steps: []StepState{{ID: "dns"}}, report: func() {}}
		_ = dnsStep(spec).do(ctx, r)
		if r.steps[0].Note != c.want {
			t.Fatalf("ответ %v: подпись %q, ждали %q", c.answer, r.steps[0].Note, c.want)
		}
	}
}

// 5е: файл сайта у чужого веб-сервера, который ведёт на порт Wynd без нашей
// первой строки, осмотр называет; ищет и по ссылкам sites-enabled.
func TestInspectionForeignSite(t *testing.T) {
	rep := ParseInspection("## foreignsite\n/etc/apache2/sites-enabled/wynd.conf\n## ownproxy\n", "root", time.Now())
	if len(rep.ForeignSite) != 1 || rep.ForeignSite[0] != "/etc/apache2/sites-enabled/wynd.conf" {
		t.Fatalf("чужой файл: %v", rep.ForeignSite)
	}
	if !strings.Contains(inspectScript, "grep -RlsE") || !strings.Contains(inspectScript, siteMark) {
		t.Fatal("поиск чужого файла должен идти по ссылкам и узнавать нашу первую строку")
	}
}
