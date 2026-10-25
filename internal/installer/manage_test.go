package installer

import (
	"strings"
	"testing"
)

func TestParseServerState(t *testing.T) {
	out := strings.Join([]string{
		"@image ghcr.io/mixeme/wynd:0.42.2",
		"@url https://family.example.ru",
		"@running 3f2a1b",
		"@stopped ",
		`@update {"from":"0.42.2","to":"0.43.0","ref":"ghcr.io/mixeme/wynd:0.43.0","stopped":true,"start_after":true}`,
		"@free /dev/vda1 47185920 38797312 8388608 83% /",
	}, "\n")
	st := parseServerState(out)
	if !st.Installed || st.Version != "0.42.2" || st.Domain != "family.example.ru" || !st.Running ||
		st.StoppedAt != "" || st.FreeBytes != 8388608<<10 || st.Update == nil || st.Update.To != "0.43.0" {
		t.Fatalf("состояние: %+v", st)
	}
	if st := parseServerState("@none\n"); st.Installed {
		t.Fatalf("Wynd нет, а состояние: %+v", st)
	}
	st = parseServerState("@image wynd:0.43.1\n@url https://a.example\n@running \n@stopped 2026-10-24T10:00:00Z\n@update \n")
	if st.Running || st.StoppedAt == "" || st.Update != nil || st.Version != "0.43.1" {
		t.Fatalf("остановлен нами: %+v", st)
	}
}

// «Вернуть» берёт базу из копии, только если новая версия уже вписана в
// compose.yaml: до этого она не запускалась, а в папке копии может лежать
// прошлая, старше живой базы (обрыв после остановки, до копии — кадр 7е).
func TestNewPlaced(t *testing.T) {
	if newPlaced(ServerState{Version: "0.42.2"}, "0.42.2") {
		t.Fatal("в compose.yaml прежняя версия — новая не вписана")
	}
	if !newPlaced(ServerState{Version: "0.43.0"}, "0.42.2") {
		t.Fatal("в compose.yaml новая версия — вписана")
	}
	if newPlaced(ServerState{}, "0.42.2") {
		t.Fatal("строки версии нет — не знаем, не трогаем")
	}
}

func TestFetchedOf(t *testing.T) {
	if got := fetchedOf(56<<20, 140<<20); got != "56 из 140 МБ" {
		t.Fatal(got)
	}
	if got := fetchedOf(1<<30, 3<<30); got != "1,0 ГБ из 3,0 ГБ" {
		t.Fatal(got)
	}
}

// Мастерская меняет в compose.yaml только строку образа Wynd (кадр 7ж).
func TestSetImage(t *testing.T) {
	compose := "services:\n  wynd:\n    image: ghcr.io/mixeme/wynd:0.42.2\n    mem_limit: 512m\n  caddy:\n    image: caddy:2-alpine\n"
	next, ok := SetImage(compose, "ghcr.io/mixeme/wynd:0.43.0")
	if !ok || !strings.Contains(next, "    image: ghcr.io/mixeme/wynd:0.43.0\n") ||
		!strings.Contains(next, "mem_limit: 512m") || !strings.Contains(next, "image: caddy:2-alpine") {
		t.Fatalf("правка:\n%s", next)
	}
	if _, ok := SetImage("services:\n  wynd:\n    image: ${WYND_IMAGE}\n", "x"); ok {
		t.Fatal("строки с версией нет, а файл тронут")
	}
	if _, ok := SetImage(compose+"  other:\n    image: wynd:0.1.0\n", "x"); ok {
		t.Fatal("две строки Wynd — какую менять, неизвестно")
	}
}

func TestCanCopyLive(t *testing.T) {
	for v, want := range map[string]bool{"0.43.1": false, "0.43.2": true, "0.44.0": true, "1.0.0": true, "0.9.9": false, "x": false} {
		if got := CanCopyLive(v); got != want {
			t.Fatalf("%s: %v", v, got)
		}
	}
}

// Порядок шагов: с остановкой — копия после остановки; без неё — копия на
// работающем сайте, остановка после; остановленный сайт не останавливают.
func TestUpdateStepOrder(t *testing.T) {
	ids := func(spec UpdateSpec) string {
		var out []string
		for _, s := range UpdateTitles(spec) {
			out = append(out, s.ID)
		}
		return strings.Join(out, ",")
	}
	base := UpdateSpec{From: "0.42.2", To: "0.43.0", Image: RegistryImage{Repo: DefaultRegistry}, StartAfter: true}
	stop, live, stopped := base, base, base
	stop.Running, stop.StopForCopy = true, true
	live.Running = true
	if got := ids(stop); got != "image,stop,copy,start,check" {
		t.Fatalf("с остановкой: %s", got)
	}
	if got := ids(live); got != "image,copy,stop,start,check" {
		t.Fatalf("без остановки: %s", got)
	}
	if got := ids(stopped); got != "image,copy,start,check" {
		t.Fatalf("сайт стоял: %s", got)
	}
}
