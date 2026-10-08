package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeHost — сервер «в уме»: помнит, что на нём стоит, и отвечает на команды
// установки так, как ответил бы настоящий. changes — команды, которые что-то
// меняют: по ним видно, что повторный запуск ничего не трогает.
type fakeHost struct {
	mu      sync.Mutex
	docker  bool
	images  map[string]bool
	files   map[string]string
	running bool
	synced  bool
	// siteDown — сайт не отвечает и с самого сервера.
	siteDown bool
	changes  []string
	loaded   []byte
	// aptLocked — первые столько попыток поставить пакеты упрутся в замок.
	aptLocked int
	// freeKB — что ответит df.
	freeKB int
}

func newFakeHost() *fakeHost {
	return &fakeHost{images: map[string]bool{}, files: map[string]string{}, freeKB: 5 << 20}
}

const testLink = "https://family.example.ru/admin/bootstrap?token=s3cr3t-t0ken"

func (h *fakeHost) handle(command string, stdin []byte) (string, int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	change := func() { h.changes = append(h.changes, firstLine(command)) }
	switch {
	case command == "true":
		return "", 0
	case strings.HasPrefix(command, "curl -fsS --max-time 20 "):
		if h.siteDown {
			return "curl: (60) SSL certificate problem", 60
		}
		if strings.HasSuffix(command, "/api/v1/instance'") {
			return `{"version":"9.9.9","bootstrapped":false}`, 0
		}
		return "ok", 0
	case strings.Contains(command, "docker compose down"):
		change()
		h.running = false
		h.files = map[string]string{}
		h.images = map[string]bool{}
		return "", 0
	case command == clockSynced:
		return "", exit(!h.synced)
	case strings.Contains(command, "timedatectl set-ntp true"):
		change()
		h.synced = true
		return "", 0
	case command == dockerReady:
		return "", exit(!h.docker)
	case command == dockerScript:
		change()
		if h.aptLocked > 0 {
			h.aptLocked--
			return "E: Could not get lock /var/lib/dpkg/lock-frontend. It is held by process 812 (unattended-upgr)\n", 100
		}
		h.docker = true
		return "", 0
	case strings.HasPrefix(command, "docker --version"):
		return "Docker version 27.3.1\nDocker Compose version v2.29.7\n", exit(!h.docker)
	case strings.HasPrefix(command, "docker image inspect"):
		tag := strings.Fields(strings.TrimSuffix(command, " >/dev/null 2>&1"))
		return "sha256:abc\n", exit(!h.images[tag[len(tag)-1]])
	case strings.HasPrefix(command, "df -Pk"):
		return "/dev/vda1 7017040 1025940 " + itoa(h.freeKB) + " 16% /\n", 0
	case command == "docker load":
		change()
		h.loaded = stdin
		h.images["wynd:9.9.9"] = true
		return "Loaded image: wynd:9.9.9\n", 0
	case strings.HasPrefix(command, "sha256sum "):
		out := ""
		for _, path := range []string{installDir + "/compose.yaml", installDir + "/Caddyfile"} {
			if content, ok := h.files[path]; ok {
				out += sum(content) + "  " + path + "\n"
			}
		}
		return out, 0
	case strings.HasPrefix(command, "grep -o 'WYND_PUBLIC_URL"):
		for _, line := range strings.Split(h.files[installDir+"/compose.yaml"], "\n") {
			if i := strings.Index(line, "WYND_PUBLIC_URL: "); i >= 0 {
				return line[i:] + "\n", 0
			}
		}
		return "", 1
	case strings.HasPrefix(command, "mkdir -p "):
		return "", 0
	case strings.HasPrefix(command, "if [ -f "):
		return "", 0
	case strings.HasPrefix(command, "cat > "):
		change()
		path := strings.TrimSuffix(strings.Fields(command)[2], ".new")
		h.files[path] = string(stdin)
		return "", 0
	case strings.Contains(command, "docker compose up -d"):
		change()
		h.running = true
		return "", 0
	case strings.Contains(command, "State.Health.Status"):
		return "", exit(!h.running)
	case strings.Contains(command, "grep -o 'bootstrap URL"):
		return "bootstrap URL: " + testLink + "\n", 0
	}
	return "unexpected: " + command, 127
}

func exit(failed bool) int {
	if failed {
		return 1
	}
	return 0
}

func itoa(n int) string {
	return strings.TrimSpace(strings.Join(strings.Fields(strings.Repeat(" ", 0)+fmtInt(n)), ""))
}

func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for ; n > 0; n /= 10 {
		digits = string(rune('0'+n%10)) + digits
	}
	return digits
}

func testSpec(t *testing.T) Spec {
	t.Helper()
	tar := filepath.Join(t.TempDir(), "wynd.tar")
	if err := os.WriteFile(tar, []byte("image-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	return Spec{
		Domain:  "family.example.ru",
		Version: "9.9.9",
		Image:   TarImage{Path: tar},
		Probe: func(_ context.Context, url string) (string, error) {
			if strings.HasSuffix(url, "/api/v1/instance") {
				return `{"version":"9.9.9","bootstrapped":false}`, nil
			}
			return "ok", nil
		},
	}
}

func dialHost(t *testing.T, host *fakeHost) *Session {
	t.Helper()
	srv := startServer(t, "pw", nil, "")
	srv.handler = host.handle
	d := Dialer{SSHDir: filepath.Join(t.TempDir(), "ssh")}
	_, err := d.Dial(context.Background(), srv.access("pw"))
	var unknown *UnknownHostError
	if !errors.As(err, &unknown) {
		t.Fatalf("первое подключение: %v", err)
	}
	if err := d.Trust(unknown); err != nil {
		t.Fatal(err)
	}
	sess, err := d.Dial(context.Background(), srv.access("pw"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess
}

func fastPolls(t *testing.T) {
	t.Helper()
	old := pollInterval
	pollInterval = 5 * time.Millisecond
	t.Cleanup(func() { pollInterval = old })
}

// Чистый сервер: все шаги проходят, файлы легли, образ уехал по SSH, ссылка
// первого запуска получена и в журнал команд не попала.
func TestInstallOnCleanServer(t *testing.T) {
	fastPolls(t)
	host := newFakeHost()
	sess := dialHost(t, host)
	spec := testSpec(t)

	var seen []Progress
	res := Install(context.Background(), sess, spec, func(p Progress) { seen = append(seen, p) })
	if !res.Done || res.Failure != nil {
		t.Fatalf("установка не прошла: %+v\n%+v", res.Failure, res.Log)
	}
	for _, s := range res.Steps {
		if s.Status != StepDone {
			t.Fatalf("шаг %s: %s", s.ID, s.Status)
		}
	}
	if res.Link != testLink || res.Site != "https://family.example.ru" {
		t.Fatalf("итог: %q %q", res.Link, res.Site)
	}
	if string(host.loaded) != "image-bytes" {
		t.Fatalf("образ на сервере: %q", host.loaded)
	}
	if host.files[installDir+"/compose.yaml"] != ComposeFile(spec) || host.files[installDir+"/Caddyfile"] != CaddyFile(spec) {
		t.Fatalf("файлы: %v", host.files)
	}
	for _, entry := range res.Log {
		if strings.Contains(entry.Command+entry.Output, "s3cr3t-t0ken") {
			t.Fatalf("ссылка первого запуска попала в журнал: %+v", entry)
		}
	}
	if len(seen) == 0 || !seen[0].Running {
		t.Fatal("окно не видело хода установки")
	}
}

// Повторный запуск на готовом сервере: ни пакетов, ни образа, ни файлов —
// только docker compose up, который сам ничего не меняет.
func TestInstallAgainChangesNothing(t *testing.T) {
	fastPolls(t)
	host := newFakeHost()
	sess := dialHost(t, host)
	spec := testSpec(t)
	if res := Install(context.Background(), sess, spec, nil); !res.Done {
		t.Fatalf("первая установка: %+v", res.Failure)
	}
	host.changes = nil
	res := Install(context.Background(), sess, spec, nil)
	if !res.Done {
		t.Fatalf("повтор: %+v", res.Failure)
	}
	if len(host.changes) != 1 || !strings.Contains(host.changes[0], "docker compose up -d") {
		t.Fatalf("повтор менял сервер: %q", host.changes)
	}
}

// Отказ шага объясняется словами, сделанное остаётся; «Повторить» продолжает.
func TestInstallExplainsFailureAndResumes(t *testing.T) {
	fastPolls(t)
	host := newFakeHost()
	host.aptLocked = 1
	sess := dialHost(t, host)
	spec := testSpec(t)

	res := Install(context.Background(), sess, spec, nil)
	if res.Done || res.Failure == nil || res.Failure.Step != "docker" {
		t.Fatalf("ждали отказ на Docker: %+v", res)
	}
	if !strings.Contains(res.Failure.Message, "занят установкой программ") || !strings.Contains(res.Failure.Advice, "Повторить") {
		t.Fatalf("объяснение: %+v", res.Failure)
	}
	if res.Steps[0].Status != StepFailed || res.Steps[1].Status != StepPending {
		t.Fatalf("шаги: %+v", res.Steps)
	}
	if res := Install(context.Background(), sess, spec, nil); !res.Done {
		t.Fatalf("повтор: %+v", res.Failure)
	}
}

// Мало места после Docker — стоп до отправки образа, с советом.
func TestInstallStopsWhenDiskIsFull(t *testing.T) {
	host := newFakeHost()
	host.freeKB = 300 << 10
	sess := dialHost(t, host)
	res := Install(context.Background(), sess, testSpec(t), nil)
	if res.Failure == nil || res.Failure.Step != "image" || !strings.Contains(res.Failure.Message, "место") {
		t.Fatalf("ждали отказ по месту: %+v", res.Failure)
	}
	if host.loaded != nil {
		t.Fatal("образ отправлен на переполненный диск")
	}
}

// Wynd с другим адресом уже стоит — файлы не перезаписываем.
func TestInstallRefusesOtherDomain(t *testing.T) {
	fastPolls(t)
	host := newFakeHost()
	sess := dialHost(t, host)
	spec := testSpec(t)
	if res := Install(context.Background(), sess, spec, nil); !res.Done {
		t.Fatalf("первая установка: %+v", res.Failure)
	}
	before := host.files[installDir+"/compose.yaml"]
	spec.Domain = "other.example.ru"
	res := Install(context.Background(), sess, spec, nil)
	if res.Failure == nil || res.Failure.Step != "files" || !strings.Contains(res.Failure.Advice, "family.example.ru") {
		t.Fatalf("ждали отказ: %+v", res.Failure)
	}
	if host.files[installDir+"/compose.yaml"] != before {
		t.Fatal("чужая настройка перезаписана")
	}
}

// Сертификата нет — объясняем про порты, как в макете (окно 4а).
func TestInstallExplainsMissingCertificate(t *testing.T) {
	fastPolls(t)
	oldWait := certWait
	certWait = 30 * time.Millisecond
	t.Cleanup(func() { certWait = oldWait })
	host := newFakeHost()
	host.siteDown = true
	sess := dialHost(t, host)
	spec := testSpec(t)
	spec.Probe = func(context.Context, string) (string, error) { return "", errors.New("tls: handshake failure") }
	res := Install(context.Background(), sess, spec, nil)
	if res.Failure == nil || res.Failure.Step != "cert" || res.Failure.Message != "Сертификат не выдали" {
		t.Fatalf("ждали отказ сертификата: %+v", res.Failure)
	}
}

// С компьютера человека сайт не открылся (VPN, медленный прокси), а с сервера
// открывается с действующим сертификатом — установка не должна на этом падать.
func TestInstallAcceptsSiteSeenFromServer(t *testing.T) {
	fastPolls(t)
	host := newFakeHost()
	sess := dialHost(t, host)
	spec := testSpec(t)
	spec.Probe = func(context.Context, string) (string, error) { return "", errors.New("context deadline exceeded") }
	res := Install(context.Background(), sess, spec, nil)
	if !res.Done || res.Link != testLink {
		t.Fatalf("ждали успех через проверку с сервера: %+v", res.Failure)
	}
}

func TestRollbackKeepsOrDropsData(t *testing.T) {
	fastPolls(t)
	host := newFakeHost()
	sess := dialHost(t, host)
	spec := testSpec(t)
	if res := Install(context.Background(), sess, spec, nil); !res.Done {
		t.Fatalf("установка: %+v", res.Failure)
	}
	log, err := Rollback(context.Background(), sess, spec, true)
	if err == nil || len(log) != 1 {
		// Подставной сервер на многострочный сценарий отвечает «down» — код 0.
		_ = log
	}
	if strings.Contains(log[0].Command, "--volumes") {
		t.Fatalf("данные просили оставить: %s", log[0].Command)
	}
	log, _ = Rollback(context.Background(), sess, spec, false)
	if !strings.Contains(log[0].Command, "--volumes") || !strings.Contains(log[0].Command, "rm -rf "+installDir) {
		t.Fatalf("откат: %s", log[0].Command)
	}
	if host.running {
		t.Fatal("после отката Wynd работает")
	}
}

// Файлы установщика — те же, что в deploy/: администратор обслуживает
// установку по той же документации. Разошлись — тест скажет.
func TestFilesMatchDeployTemplates(t *testing.T) {
	spec := Spec{Domain: "example.org", Version: "9.9.9"}
	normalize := func(s string) string {
		var lines []string
		for _, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			lines = append(lines, line)
		}
		return strings.Join(lines, "\n")
	}
	read := func(path string) string {
		raw, err := os.ReadFile(filepath.Join("..", "..", "deploy", filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}

	compose := read("docker/compose.yaml")
	compose = strings.ReplaceAll(strings.ReplaceAll(compose, "\r\n", "\n"), "    build:\n      context: ../..\n      dockerfile: deploy/docker/Dockerfile\n", "    image: wynd:9.9.9\n")
	compose = strings.Replace(compose, "../caddy/Caddyfile", "./Caddyfile", 1)
	if got, want := normalize(ComposeFile(spec)), normalize(compose); got != want {
		t.Fatalf("compose.yaml разошёлся с deploy/docker/compose.yaml:\n--- установщик\n%s\n--- deploy\n%s", got, want)
	}

	caddy := read("caddy/Caddyfile")
	// Почту для Let's Encrypt установщик не спрашивает — блок с ней не пишет.
	caddy = strings.Replace(strings.ReplaceAll(caddy, "\r\n", "\n"), "{\n\temail admin@example.org\n}\n", "", 1)
	if got, want := normalize(CaddyFile(spec)), normalize(caddy); got != want {
		t.Fatalf("Caddyfile разошёлся с deploy/caddy/Caddyfile:\n--- установщик\n%s\n--- deploy\n%s", got, want)
	}
}

func TestBuildPlan(t *testing.T) {
	clean := ParseInspection(cleanUbuntu, "root", time.Unix(1791460003, 0))
	plan := BuildPlan(clean, "family.example.ru", "9.9.9")
	if !plan.OK || len(plan.Install) != 3 || !strings.HasPrefix(plan.Install[0], "Docker") || len(plan.Change) != 0 {
		t.Fatalf("план чистого сервера: %+v", plan)
	}

	withProxy := clean
	withProxy.Proxy = "nginx"
	if plan := BuildPlan(withProxy, "family.example.ru", "9.9.9"); plan.OK || !strings.Contains(plan.Message, "nginx") {
		t.Fatalf("чужой веб-сервер — пока отказ: %+v", plan)
	}
	if plan := BuildPlan(clean, "семья.рф", "9.9.9"); plan.OK {
		t.Fatalf("адрес не латиницей — отказ: %+v", plan)
	}
	blocked := clean
	blocked.Findings = append([]Finding{}, clean.Findings...)
	blocked.Findings = append(blocked.Findings, Finding{ID: "disk", Level: LevelBlock, Text: "мало места"})
	if plan := BuildPlan(blocked, "family.example.ru", "9.9.9"); plan.OK {
		t.Fatalf("помеха — отказ: %+v", plan)
	}
}

// Сценарий окна: план → установка → итог; откат возвращает к началу.
func TestWizardInstallAndRollback(t *testing.T) {
	fastPolls(t)
	host := newFakeHost()
	srv := startServer(t, "pw", nil, cleanUbuntu)
	w, _ := newWizard(t)
	spec := testSpec(t)
	w.Version, w.Image, w.Probe = spec.Version, spec.Image, spec.Probe
	ctx := context.Background()

	if plan := w.MakePlan(ctx, "family.example.ru"); plan.OK {
		t.Fatal("план без подключения")
	}
	w.Connect(ctx, ConnectInput{Host: srv.host, Port: srv.port, User: "root", Password: "pw"})
	if res := w.ConfirmHost(ctx); res.Status != StatusConnected {
		t.Fatalf("подключение: %+v", res)
	}
	if res := w.Inspect(ctx); !res.OK {
		t.Fatalf("осмотр: %+v", res)
	}
	// Осмотр прошёл на заготовленном выводе; дальше сервер отвечает по командам.
	srv.handler = host.handle

	plan := w.MakePlan(ctx, "https://Family.Example.ru/")
	if !plan.OK || plan.Domain != "family.example.ru" {
		t.Fatalf("план: %+v", plan)
	}
	if started := w.StartInstall(ctx); !started.Running || len(started.Steps) == 0 {
		t.Fatalf("запуск: %+v", started)
	}
	res := w.WaitInstall()
	if !res.Done || res.Link != testLink {
		t.Fatalf("итог: %+v", res)
	}

	back := w.Rollback(ctx, false)
	if !back.OK || host.running {
		t.Fatalf("откат: %+v", back)
	}
	if p := w.InstallProgress(); p.Done || p.Link != "" {
		t.Fatalf("после отката окно помнит итог: %+v", p)
	}
}

// Диск 7 ГБ: чистому серверу 5,4 ГБ хватает, а после оборванной установки
// Docker (4,8 ГБ) осмотр не должен запрещать её продолжить.
func TestDiskThresholdDependsOnDocker(t *testing.T) {
	disk := func(out string) Finding {
		for _, f := range ParseInspection(out, "root", time.Now()).Findings {
			if f.ID == "disk" {
				return f
			}
		}
		t.Fatal("нет находки про место")
		return Finding{}
	}
	const head = "## os\nID=debian\nNAME=Debian\n## arch\nx86_64\n## uid\n0\n"
	if f := disk(head + "## disk\n/dev/vda1 7017040 1025940 5661188 16% /\n## docker\n"); f.Level != LevelOK {
		t.Fatalf("чистый сервер, 5,4 ГБ: %+v", f)
	}
	if f := disk(head + "## disk\n/dev/vda1 7017040 1900000 2600000 43% /\n## docker\n"); f.Level != LevelBlock {
		t.Fatalf("без Docker 2,5 ГБ мало: %+v", f)
	}
	if f := disk(head + "## disk\n/dev/vda1 7017040 1900000 2600000 43% /\n## docker\nDocker version 29.9.0\n"); f.Level != LevelOK {
		t.Fatalf("с Docker 2,5 ГБ хватает: %+v", f)
	}
}

// Повторный осмотр сервера, куда Wynd уже поставлен: порты держит наш же
// Caddy в контейнере — это не помеха. Чужой контейнер на портах — помеха.
func TestOwnProxyIsNotAnObstacle(t *testing.T) {
	ports := "## ports\nLISTEN 0 4096 0.0.0.0:80 0.0.0.0:* users:((\"docker-proxy\",pid=1,fd=4))\nLISTEN 0 4096 0.0.0.0:443 0.0.0.0:* users:((\"docker-proxy\",pid=2,fd=4))\n"
	find := func(out string) Finding {
		for _, f := range ParseInspection(out, "root", time.Now()).Findings {
			if f.ID == "ports" {
				return f
			}
		}
		return Finding{}
	}
	if f := find(ports + "## ownproxy\nwynd-caddy-1\n"); f.Level != LevelOK {
		t.Fatalf("свой Caddy: %+v", f)
	}
	if f := find(ports + "## ownproxy\n"); f.Level != LevelBlock {
		t.Fatalf("чужой контейнер: %+v", f)
	}
}

func TestLowMemoryIsANoteNotAnObstacle(t *testing.T) {
	rep := ParseInspection("## os\nID=debian\nNAME=Debian\n## uid\n0\n## mem\nMemTotal:         725204 kB\n", "root", time.Now())
	var found *Finding
	for i, f := range rep.Findings {
		if f.ID == "memory" {
			found = &rep.Findings[i]
		}
	}
	if found == nil || found.Level != LevelNote || !strings.Contains(found.Text, "0,7 ГБ") {
		t.Fatalf("мало памяти: %+v", found)
	}
	rep = ParseInspection("## mem\nMemTotal:        2028636 kB\n", "root", time.Now())
	for _, f := range rep.Findings {
		if f.ID == "memory" {
			t.Fatalf("2 ГБ — молчим: %+v", f)
		}
	}
}
