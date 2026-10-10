package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const nginxPorts = "LISTEN 0 511 0.0.0.0:80 0.0.0.0:* users:((\"nginx\",pid=1,fd=6))\nLISTEN 0 511 0.0.0.0:443 0.0.0.0:* users:((\"nginx\",pid=1,fd=7))\n"

func behindNginx(t *testing.T) (*fakeHost, Spec) {
	t.Helper()
	fastPolls(t)
	host := newFakeHost()
	host.layout = "layout=sites\n"
	spec := testSpec(t)
	spec.Proxy = ProxyNginx
	return host, spec
}

// За чужим nginx: Wynd слушает только свой порт, сайт лёг отдельным файлом,
// сертификат получил certbot; своего Caddy нет.
func TestInstallBehindNginx(t *testing.T) {
	host, spec := behindNginx(t)
	sess := dialHost(t, host)

	res := Install(context.Background(), sess, spec, nil)
	if !res.Done || res.Failure != nil {
		t.Fatalf("установка не прошла: %+v\n%+v", res.Failure, res.Log)
	}
	var ids []string
	for _, s := range res.Steps {
		ids = append(ids, s.ID)
	}
	if got := strings.Join(ids, " "); got != "docker image files start site cert check" {
		t.Fatalf("шаги: %s", got)
	}
	compose := host.files[installDir+"/compose.yaml"]
	if compose != ComposeFile(spec) || !strings.Contains(compose, `"127.0.0.1:7676:7676"`) || strings.Contains(compose, "caddy") {
		t.Fatalf("compose.yaml: %s", compose)
	}
	if _, ok := host.files[installDir+"/Caddyfile"]; ok {
		t.Fatal("за чужим веб-сервером свой Caddyfile не нужен")
	}
	site := host.files["/etc/nginx/sites-available/wynd-family.example.ru.conf"]
	if site != NginxSite(spec.Domain, true) || !strings.HasPrefix(site, siteMark) {
		t.Fatalf("файл сайта: %q", site)
	}
	issued := 0
	for _, c := range host.changes {
		if strings.HasPrefix(c, "cb certonly") {
			issued++
			if strings.Contains(c, "--test-cert") || !strings.Contains(c, "--webroot -w "+acmeRoot) || !strings.Contains(c, "-d family.example.ru") {
				t.Fatalf("запрос сертификата: %s", c)
			}
		}
	}
	if issued != 1 {
		t.Fatalf("сертификат запрошен %d раз", issued)
	}

	// Повтор на готовом сервере ничего не меняет — ни сайта, ни сертификата.
	host.changes = nil
	if res := Install(context.Background(), sess, spec, nil); !res.Done {
		t.Fatalf("повтор: %+v", res.Failure)
	}
	if len(host.changes) != 1 || !strings.Contains(host.changes[0], "docker compose up -d") {
		t.Fatalf("повтор менял сервер: %q", host.changes)
	}
}

// Сначала сайт только отвечает на проверку Let's Encrypt: на сертификат,
// которого ещё нет, настройка ссылаться не может.
func TestNginxSiteWaitsForCertificate(t *testing.T) {
	host, spec := behindNginx(t)
	host.certbotFails = "Certbot failed to authenticate some domains (authenticator: webroot).\n  Type:   connection\n  Detail: Timeout during connect (likely firewall problem)\nChallenge failed for domain family.example.ru\n"
	sess := dialHost(t, host)
	res := Install(context.Background(), sess, spec, nil)
	if res.Failure == nil || res.Failure.Step != "cert" || res.Failure.Message != "Сертификат не выдали" || !strings.Contains(res.Failure.Advice, "порт 80") {
		t.Fatalf("ждали отказ сертификата: %+v", res.Failure)
	}
	if site := host.files["/etc/nginx/sites-available/wynd-family.example.ru.conf"]; site != NginxSite(spec.Domain, false) || strings.Contains(site, "ssl_certificate") {
		t.Fatalf("до сертификата сайт должен быть без него: %q", site)
	}
	host.certbotFails = "There were too many requests of a given type :: Error creating new order :: too many certificates (5) already issued for this exact set of domains\n"
	res = Install(context.Background(), sess, spec, nil)
	if res.Failure == nil || !strings.Contains(res.Failure.Message, "пока не выдаёт") {
		t.Fatalf("ждали объяснение про предел: %+v", res.Failure)
	}
	host.certbotFails = ""
	if res := Install(context.Background(), sess, spec, nil); !res.Done {
		t.Fatalf("повтор: %+v", res.Failure)
	}
}

// nginx не принял настройку с нашим файлом — отказ словами; что файл убран, а
// nginx не тронут, делает сценарий на сервере.
func TestNginxRejectsSite(t *testing.T) {
	host, spec := behindNginx(t)
	host.rejectsSite = true
	sess := dialHost(t, host)
	res := Install(context.Background(), sess, spec, nil)
	if res.Failure == nil || res.Failure.Step != "site" || !strings.Contains(res.Failure.Message, "nginx не принял") || !strings.Contains(res.Failure.Advice, "работают как раньше") {
		t.Fatalf("ждали отказ настройки: %+v", res.Failure)
	}
	if len(host.files) != 1 {
		t.Fatalf("на сервере осталось: %v", host.files)
	}
}

// Сценарий записи сайта: сначала проверка самого веб-сервера, перечитать —
// только после неё, и не перезапуск.
func TestApplySiteChecksBeforeReload(t *testing.T) {
	host, spec := behindNginx(t)
	sess := dialHost(t, host)
	r := &Run{sess: sess, spec: spec, steps: StepTitles(spec), report: func() {}}
	if err := applySite(context.Background(), r, "/etc/nginx/sites-available/x.conf", "/etc/nginx/sites-enabled/x.conf", "content"); err != nil {
		t.Fatal(err)
	}
	if host.files["/etc/nginx/sites-available/x.conf"] != "content" {
		t.Fatalf("файл: %v", host.files)
	}
	for _, kind := range []string{ProxyNginx, ProxyCaddy} {
		k := proxyOf(kind)
		if strings.Contains(k.reload, "restart") || !strings.Contains(k.reload, "reload") {
			t.Fatalf("%s: чужой веб-сервер только перечитывает настройку: %s", kind, k.reload)
		}
	}
}

func TestInstallBehindCaddy(t *testing.T) {
	fastPolls(t)
	host := newFakeHost()
	host.layout = "import conf.d/*.caddy\n"
	spec := testSpec(t)
	spec.Proxy = ProxyCaddy
	sess := dialHost(t, host)
	res := Install(context.Background(), sess, spec, nil)
	if !res.Done {
		t.Fatalf("установка не прошла: %+v\n%+v", res.Failure, res.Log)
	}
	if site := host.files["/etc/caddy/conf.d/wynd-family.example.ru.caddy"]; site != CaddySite(spec.Domain) {
		t.Fatalf("файл сайта: %v", host.files)
	}
	for _, c := range host.changes {
		if strings.Contains(c, "certbot") {
			t.Fatalf("за Caddy сертификат получает он сам: %s", c)
		}
	}
}

func TestCaddySitePath(t *testing.T) {
	cases := []struct{ line, want string }{
		{"import conf.d/*", "/etc/caddy/conf.d/wynd-a.example.caddy"},
		{"import /etc/caddy/sites/*.conf", "/etc/caddy/sites/wynd-a.example.conf"},
		{"import  sites-enabled/*.caddy ", "/etc/caddy/sites-enabled/wynd-a.example.caddy"},
		{"import common.caddy", ""},
		{"import */sites/*", ""},
		{"import snippets/* arg", ""},
		{"# import conf.d/*", ""},
	}
	for _, c := range cases {
		got, ok := caddySitePath([]string{c.line}, "a.example")
		if got != c.want || ok != (c.want != "") {
			t.Errorf("%q: %q %v", c.line, got, ok)
		}
	}
}

// Осмотр: за чужим веб-сервером можно встать, только если есть куда положить
// свой файл и его настройка уже сейчас проходит его же проверку.
func TestProxyFindings(t *testing.T) {
	now := time.Unix(1791460003, 0)
	withNginx := replaceSection(cleanUbuntu, "ports", nginxPorts)
	caddyPorts := strings.ReplaceAll(nginxPorts, "nginx", "caddy")
	cases := []struct {
		name, out string
		blocked   bool
		text      string
	}{
		{"nginx в порядке", withNginx + "## nginx\nlayout=sites\ntest=ok\n", false, ""},
		{"nginx с conf.d", withNginx + "## nginx\nlayout=confd\ntest=ok\n", false, ""},
		{"nginx с ошибкой", withNginx + "## nginx\nlayout=sites\ntest=fail\n", true, "В настройке nginx сейчас ошибка"},
		{"nginx устроен иначе", withNginx + "## nginx\ntest=ok\n", true, "nginx на сервере настроен необычно"},
		{"caddy с import", replaceSection(cleanUbuntu, "ports", caddyPorts) + "## caddy\nimport conf.d/*\ntest=ok\n", false, ""},
		{"caddy без import", replaceSection(cleanUbuntu, "ports", caddyPorts) + "## caddy\ntest=ok\n", true, "Caddy на сервере не читает дополнительные файлы настройки"},
		{"caddy с ошибкой", replaceSection(cleanUbuntu, "ports", caddyPorts) + "## caddy\nimport conf.d/*\ntest=fail\n", true, "В настройке Caddy сейчас ошибка"},
	}
	for _, c := range cases {
		rep := ParseInspection(c.out, "root", now)
		f, found := findings(rep)["proxy"]
		if rep.Blocked() != c.blocked || found != c.blocked || f.Text != c.text {
			t.Errorf("%s: blocked=%v %+v", c.name, rep.Blocked(), f)
		}
		if found && (f.Advice == "" || !strings.Contains(f.Advice, "Проверить снова") && c.name != "nginx устроен иначе") {
			t.Errorf("%s: совет: %q", c.name, f.Advice)
		}
	}
	if !strings.Contains(findings(ParseInspection(cases[5].out, "root", now))["proxy"].Advice, "import /etc/caddy/conf.d/*") {
		t.Error("совет должен называть строку, которую дописать")
	}
}

func TestPlanBehindProxy(t *testing.T) {
	now := time.Unix(1791460003, 0)
	rep := ParseInspection(replaceSection(cleanUbuntu, "ports", nginxPorts)+"## nginx\nlayout=sites\ntest=ok\n", "root", now)
	plan := BuildPlan(rep, "family.example.ru", "9.9.9", false)
	joined := strings.Join(plan.Install, "\n")
	if !plan.OK || plan.Proxy != ProxyNginx || !strings.Contains(joined, "в nginx — отдельным файлом") || !strings.Contains(joined, "certbot") || strings.Contains(joined, "Caddy") {
		t.Fatalf("план за nginx: %+v", plan)
	}
	if len(plan.Change) != 0 || len(plan.Keep) != 2 || !strings.Contains(plan.Keep[0], "чужие файлы не правим") {
		t.Fatalf("план за nginx: %+v", plan)
	}
	// certbot уже стоит — в «Поставим» его нет.
	rep = ParseInspection(replaceSection(cleanUbuntu, "ports", nginxPorts)+"## nginx\nlayout=sites\ntest=ok\n## certbot\n/usr/bin/certbot\n", "root", now)
	if plan := BuildPlan(rep, "family.example.ru", "9.9.9", false); strings.Contains(strings.Join(plan.Install, "\n"), "certbot") {
		t.Fatalf("certbot уже есть: %+v", plan.Install)
	}
	rep = ParseInspection(replaceSection(cleanUbuntu, "ports", strings.ReplaceAll(nginxPorts, "nginx", "caddy"))+"## caddy\nimport conf.d/*\ntest=ok\n", "root", now)
	plan = BuildPlan(rep, "family.example.ru", "9.9.9", false)
	joined = strings.Join(plan.Install, "\n")
	if !plan.OK || plan.Proxy != ProxyCaddy || !strings.Contains(joined, "его получит Caddy") || strings.Contains(joined, "Контейнер с веб-сервером") || strings.Contains(joined, "certbot") {
		t.Fatalf("план за Caddy: %+v", plan)
	}
}

// Откат за чужим веб-сервером: свои файлы узнаёт по первой строке, веб-сервер
// перечитывает настройку только после своей проверки; сертификат отзывается,
// только когда удаляют данные и только полученный нами.
func TestRollbackBehindProxy(t *testing.T) {
	keep, drop := rollbackProxy(true), rollbackProxy(false)
	for _, want := range []string{siteMark, "/etc/nginx/sites-enabled/wynd-*.conf", "if nginx -t 2>&1; then systemctl reload nginx", markNotReloaded} {
		if !strings.Contains(keep, want) {
			t.Errorf("в откате нет %q", want)
		}
	}
	if strings.Contains(keep, "certbot") || strings.Contains(keep, "restart") || strings.Contains(keep, "cb ") {
		t.Fatalf("данные оставлены — сертификат не трогаем: %s", keep)
	}
	if !strings.Contains(drop, `[ -e `+certOursDir+`/cert-"$dom" ]`) || !strings.Contains(drop, "cb revoke --non-interactive --cert-name") {
		t.Fatalf("данные удаляют — свой сертификат отзываем: %s", drop)
	}
	if !strings.Contains(drop, `-e DOM="$dom"`) || !strings.Contains(drop, `rm -rf "$store"/certificates/*/"$dom"`) {
		t.Fatalf("у чужого Caddy отзываем и убираем только сертификат адреса Wynd: %s", drop)
	}
	if strings.Contains(drop, "apt-get") || strings.Contains(drop, "purge") {
		t.Fatal("certbot остаётся на сервере")
	}
}

// Сайт и compose за чужим веб-сервером — те же, что в deploy/.
func TestProxyFilesMatchDeployTemplates(t *testing.T) {
	spec := Spec{Domain: "example.org", Version: "9.9.9", Proxy: ProxyNginx}
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
		return strings.ReplaceAll(string(raw), "\r\n", "\n")
	}
	compose := strings.Replace(read("docker/compose.own-proxy.yaml"), "    build:\n      context: ../..\n      dockerfile: deploy/docker/Dockerfile\n", "    image: wynd:9.9.9\n", 1)
	if got, want := normalize(ComposeFile(spec)), normalize(compose); got != want {
		t.Fatalf("compose.yaml разошёлся с deploy/docker/compose.own-proxy.yaml:\n--- установщик\n%s\n--- deploy\n%s", got, want)
	}
	if got, want := normalize(nginxLocation), normalize(read("proxy/nginx.conf")); got != want {
		t.Fatalf("сайт nginx разошёлся с deploy/proxy/nginx.conf:\n--- установщик\n%s\n--- deploy\n%s", got, want)
	}
	if !strings.Contains(NginxSite("example.org", true), nginxLocation) {
		t.Fatal("сайт nginx без блока из deploy/proxy/nginx.conf")
	}
	if got, want := normalize(CaddySite("example.org")), normalize(read("proxy/Caddyfile")); got != want {
		t.Fatalf("сайт Caddy разошёлся с deploy/proxy/Caddyfile:\n--- установщик\n%s\n--- deploy\n%s", got, want)
	}
}
