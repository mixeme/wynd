package installer

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"
)

// Сервер с чужим веб-сервером (план 45, срез 3). Wynd встаёт за ним: контейнер
// слушает только 127.0.0.1:7676, а сайт добавляется веб-серверу отдельным
// своим файлом. Чужие файлы не правим; перед тем как просить веб-сервер
// перечитать настройку — его собственная проверка; перечитать, а не
// перезапустить: чужие сайты не прерываются (решения владельца, 2026-10-10).

const (
	ProxyNginx  = "nginx"
	ProxyApache = "apache"
	ProxyCaddy  = "caddy"

	// siteMark — первая строка каждого нашего файла сайта: по ней откат
	// отличает своё от чужого.
	siteMark = "# Файл создал установщик Wynd."
	// acmeRoot — отсюда nginx отдаёт ответ на проверку Let's Encrypt.
	acmeRoot = installDir + "/acme"
	// certOursDir — пометки «сертификат этого адреса получили мы»: чужой
	// сертификат откат не отзывает. Лежит вне /opt/wynd: папку убирают и тогда,
	// когда данные и сертификат остаются.
	certOursDir = "/var/lib/wynd-installer"

	caddyConfig = "/etc/caddy/Caddyfile"

	// exitRejected — веб-сервер не принял настройку с нашим файлом.
	exitRejected    = 5
	markNotReloaded = "WYND_PROXY_NOT_RELOADED"
)

// proxyNames — как веб-сервер зовут в окне.
var proxyNames = map[string]string{ProxyNginx: "nginx", ProxyApache: "Apache", ProxyCaddy: "Caddy"}

// nginxLayout печатает, как nginx подключает сайты: «layout=sites» — папки
// sites-available и sites-enabled (Debian, Ubuntu), «layout=confd» — conf.d.
// Ничего не печатает — устроен иначе, своего файла положить некуда.
const nginxLayout = `if grep -Eqs '^[[:space:]]*include[[:space:]].*sites-enabled' /etc/nginx/nginx.conf && [ -d /etc/nginx/sites-available ]; then echo layout=sites; elif grep -Eqs '^[[:space:]]*include[[:space:]].*conf\.d' /etc/nginx/nginx.conf && [ -d /etc/nginx/conf.d ]; then echo layout=confd; fi`

// apacheMods — модули Apache, без которых сайт Wynd не заработает
// (deploy/proxy/apache.conf). Недостающие включаем сами — решение владельца,
// 2026-10-11; это единственное, что меняется в чужом веб-сервере помимо
// своего файла сайта, и план говорит об этом заранее.
var apacheMods = []string{"proxy", "proxy_http", "ssl", "headers", "rewrite", "setenvif"}

// apacheLayout печатает «layout=sites», если Apache устроен как в Debian и
// Ubuntu (sites-available и sites-enabled), и «missing=<модуль>» на каждый
// модуль из apacheMods, который не включён.
const apacheLayout = `if [ -d /etc/apache2/sites-available ] && [ -d /etc/apache2/sites-enabled ]; then echo layout=sites; fi; for m in proxy proxy_http ssl headers rewrite setenvif; do [ -e /etc/apache2/mods-enabled/$m.load ] || echo missing=$m; done`

// missingMods читает строки «missing=…» из вывода apacheLayout.
func missingMods(lines []string) []string {
	var res []string
	for _, l := range lines {
		if m, ok := strings.CutPrefix(strings.TrimSpace(l), "missing="); ok && m != "" {
			res = append(res, m)
		}
	}
	return res
}

func hasLine(lines []string, want string) bool {
	for _, l := range lines {
		if strings.TrimSpace(l) == want {
			return true
		}
	}
	return false
}

// nginxSitePaths — куда лечь файлу сайта и нужна ли ссылка на него. root —
// папка настройки веб-сервера: у Apache раскладка та же.
func nginxSitePaths(lines []string, domain, root string) (file, link string, ok bool) {
	name := "wynd-" + domain + ".conf"
	switch {
	case hasLine(lines, "layout=sites"):
		return root + "/sites-available/" + name, root + "/sites-enabled/" + name, true
	case hasLine(lines, "layout=confd"):
		return root + "/conf.d/" + name, "", true
	}
	return "", "", false
}

// caddySitePath ищет в строках import чужого Caddyfile папку, из которой Caddy
// сам подхватит наш файл: «import conf.d/*», «import /etc/caddy/sites/*.caddy».
// Строка с одним файлом или с местами для подстановки нам не подходит.
func caddySitePath(lines []string, domain string) (string, bool) {
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "import" {
			continue
		}
		dir, pattern := path.Split(fields[1])
		if strings.ContainsAny(dir, "*?[{") {
			continue
		}
		ext := ""
		switch {
		case pattern == "*":
			ext = ".caddy"
		case strings.HasPrefix(pattern, "*.") && !strings.ContainsAny(pattern[2:], "*?[{."):
			ext = pattern[1:]
		default:
			continue
		}
		if !path.IsAbs(dir) {
			dir = path.Join(path.Dir(caddyConfig), dir)
		}
		return path.Join(dir, "wynd-"+domain+ext), true
	}
	return "", false
}

// proxyFinding — помехи чужого веб-сервера: всё, из-за чего свой файл сайта
// положить некуда или нельзя просить веб-сервер его прочитать. lines — секция
// осмотра с именем веб-сервера.
func proxyFinding(proxy string, lines []string) (Finding, bool) {
	broken := func(name, check string) Finding {
		return Finding{
			ID: "proxy", Level: LevelBlock,
			Text:   "В настройке " + name + " сейчас ошибка",
			Advice: "Wynd добавит " + name + " свой сайт и попросит перечитать настройку, а с ошибкой " + name + " этого не сделает. Чужую настройку мы не правим. Исправьте её (проверка — команда «" + check + "» на сервере) и нажмите «Проверить снова».",
		}
	}
	switch proxy {
	case ProxyNginx:
		if _, _, ok := nginxSitePaths(lines, "x", "/etc/nginx"); !ok {
			return Finding{
				ID: "proxy", Level: LevelBlock,
				Text:   "nginx на сервере настроен необычно",
				Advice: "Wynd кладёт свой сайт отдельным файлом в папку sites-enabled или conf.d, а этот nginx ни одну из них не читает. Чужую настройку мы не правим. Возьмите для Wynd отдельный сервер или поставьте Wynd вручную по инструкции.",
			}, true
		}
		if !hasLine(lines, "test=ok") {
			return broken("nginx", "nginx -t"), true
		}
	case ProxyApache:
		if !hasLine(lines, "layout=sites") {
			return Finding{
				ID: "proxy", Level: LevelBlock,
				Text:   "Apache на сервере настроен необычно",
				Advice: "Wynd кладёт свой сайт отдельным файлом в папку sites-enabled, а у этого Apache её нет. Чужую настройку мы не правим. Возьмите для Wynd отдельный сервер или поставьте Wynd вручную по инструкции.",
			}, true
		}
		if !hasLine(lines, "test=ok") {
			return broken("Apache", "apache2ctl configtest"), true
		}
	case ProxyCaddy:
		if _, ok := caddySitePath(lines, "x"); !ok {
			return Finding{
				ID: "proxy", Level: LevelBlock,
				Text:   "Caddy на сервере не читает дополнительные файлы настройки",
				Advice: "Wynd кладёт свой сайт отдельным файлом, а чужой файл " + caddyConfig + " не правит. Допишите в его конец строку «import " + path.Dir(caddyConfig) + "/conf.d/*» и нажмите «Проверить снова».",
			}, true
		}
		if !hasLine(lines, "test=ok") {
			return broken("Caddy", "caddy validate --config "+caddyConfig), true
		}
	}
	return Finding{}, false
}

// ownProxyCompose — /opt/wynd/compose.yaml за чужим веб-сервером: то же, что
// deploy/docker/compose.own-proxy.yaml, но с готовым образом.
func ownProxyCompose(spec Spec) string {
	return `# Wynd за веб-сервером, который уже работает на этом сервере. Файл создал
# установщик Wynd; устроен так же, как deploy/docker/compose.own-proxy.yaml в
# исходниках, — обслуживается по той же документации.

services:
  wynd:
    image: ` + spec.imageTag() + `
    ports:
      - "127.0.0.1:7676:7676"
    volumes:
      - wynd-data:/data
    environment:
      WYND_PUBLIC_URL: https://` + spec.Domain + `
      # The host proxy reaches the container through docker-proxy, so the
      # source is always the gateway of this network. Trust only it: a wider
      # range would let any container on the host pick its rate-limit bucket.
      WYND_TRUSTED_PROXIES: 172.30.76.1
    networks:
      - wynd
    restart: unless-stopped

networks:
  wynd:
    ipam:
      config:
        - subnet: 172.30.76.0/24
          gateway: 172.30.76.1

volumes:
  wynd-data:
`
}

// nginxLocation — то же, что deploy/proxy/nginx.conf.
const nginxLocation = `    location / {
        proxy_pass http://127.0.0.1:7676;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
        proxy_buffering off;
        proxy_read_timeout 300s;
        client_max_body_size 100m;
    }
`

// NginxSite — файл сайта Wynd для чужого nginx. withCert ложно, пока
// сертификата нет: сайт только отвечает на проверку Let's Encrypt — nginx не
// примет настройку со ссылкой на сертификат, которого ещё нет.
func NginxSite(domain string, withCert bool) string {
	s := siteMark + ` Сайт Wynd за nginx; устроен так же, как
# deploy/proxy/nginx.conf в исходниках. Чтобы убрать — удалите этот файл и
# попросите nginx перечитать настройку.

server {
    listen 80;
    server_name ` + domain + `;

    location /.well-known/acme-challenge/ {
        root ` + acmeRoot + `;
    }
    location / {
        return 301 https://$host$request_uri;
    }
}
`
	if !withCert {
		return s
	}
	return s + `
server {
    listen 443 ssl;
    server_name ` + domain + `;
    ssl_certificate /etc/letsencrypt/live/` + domain + `/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/` + domain + `/privkey.pem;

` + nginxLocation + `}
`
}

// apacheHTTPS — то же, что блок на 443 в deploy/proxy/apache.conf.
func apacheHTTPS(domain string) string {
	return `<VirtualHost *:443>
    ServerName ` + domain + `
    SSLEngine on
    SSLCertificateFile /etc/letsencrypt/live/` + domain + `/fullchain.pem
    SSLCertificateKeyFile /etc/letsencrypt/live/` + domain + `/privkey.pem

    RequestHeader unset X-Forwarded-For
    RequestHeader set X-Real-IP "%{REMOTE_ADDR}s"
    RequestHeader set X-Forwarded-Proto "https"

    ProxyPreserveHost On
    ProxyPass        "/" "http://127.0.0.1:7676/" timeout=300 flushpackets=on
    ProxyPassReverse "/" "http://127.0.0.1:7676/"

    LimitRequestBody 104857600
    # mod_deflate buffers streamed responses: keep SSE and probes uncompressed.
    SetEnvIfNoCase Accept "text/event-stream" no-gzip
    SetEnvIf Request_URI "^/api/v1/probe/" no-gzip

    Header always set Strict-Transport-Security "max-age=31536000; includeSubDomains"
</VirtualHost>
`
}

// ApacheSite — файл сайта Wynd для чужого Apache; withCert — как у NginxSite.
func ApacheSite(domain string, withCert bool) string {
	s := siteMark + ` Сайт Wynd за Apache; устроен так же, как
# deploy/proxy/apache.conf в исходниках. Чтобы убрать — удалите этот файл и
# ссылку на него в sites-enabled и попросите Apache перечитать настройку.

<VirtualHost *:80>
    ServerName ` + domain + `
    Alias /.well-known/acme-challenge/ ` + acmeRoot + `/.well-known/acme-challenge/
    <Directory ` + acmeRoot + `>
        Require all granted
    </Directory>
    RewriteEngine On
    RewriteCond %{REQUEST_URI} !^/\.well-known/acme-challenge/
    RewriteRule ^ https://%{SERVER_NAME}%{REQUEST_URI} [R=301,L]
</VirtualHost>
`
	if !withCert {
		return s
	}
	return s + "\n" + apacheHTTPS(domain)
}

// siteContent — файл сайта для веб-сервера, которому сертификат получает
// certbot (nginx, Apache).
func siteContent(spec Spec, withCert bool) string {
	if spec.Proxy == ProxyApache {
		return ApacheSite(spec.Domain, withCert)
	}
	return NginxSite(spec.Domain, withCert)
}

// CaddySite — файл сайта Wynd для чужого Caddy: deploy/proxy/Caddyfile с
// адресом человека. Сертификат Caddy получает сам.
func CaddySite(domain string) string {
	return siteMark + ` Сайт Wynd за Caddy; устроен так же, как
# deploy/proxy/Caddyfile в исходниках. Чтобы убрать — удалите этот файл и
# попросите Caddy перечитать настройку.

` + domain + ` {
    header Strict-Transport-Security "max-age=31536000; includeSubDomains"
    reverse_proxy 127.0.0.1:7676 {
        flush_interval -1
        header_up X-Real-IP {remote_host}
        header_up X-Forwarded-For {remote_host}
        transport http {
            read_timeout 300s
        }
    }
    request_body {
        max_size 100MB
    }
}
`
}

// proxyKind — как говорить с веб-сервером: его проверка настройки и просьба
// её перечитать.
type proxyKind struct {
	name   string
	test   string
	reload string
	// prepare — что сделать до проверки настройки (включить модули Apache).
	prepare string
	// root — папка настройки веб-сервера.
	root string
	// layout печатает, куда класть файл сайта (строки для nginxSitePaths и
	// caddySitePath).
	layout string
}

func proxyOf(name string) proxyKind {
	switch name {
	case ProxyCaddy:
		return proxyKind{
			name:   "Caddy",
			test:   "caddy validate --config " + caddyConfig + " --adapter caddyfile",
			reload: "systemctl reload caddy",
			layout: "grep -Es '^import[[:space:]]' " + caddyConfig,
		}
	case ProxyApache:
		return proxyKind{
			name:    "Apache",
			test:    "apache2ctl configtest",
			reload:  "systemctl reload apache2",
			prepare: "a2enmod -q " + strings.Join(apacheMods, " "),
			root:    "/etc/apache2",
			layout:  apacheLayout,
		}
	}
	return proxyKind{name: "nginx", test: "nginx -t", reload: "systemctl reload nginx", root: "/etc/nginx", layout: nginxLayout}
}

// sitePaths спрашивает сервер, куда лечь файлу сайта.
func sitePaths(ctx context.Context, r *Run) (file, link string, err error) {
	kind := proxyOf(r.spec.Proxy)
	res, err := r.sess.Exec(ctx, kind.layout, nil, shortTimeout)
	if err != nil {
		return "", "", err
	}
	lines := strings.Split(res.Output, "\n")
	ok := false
	if r.spec.Proxy == ProxyCaddy {
		file, ok = caddySitePath(lines, r.spec.Domain)
	} else {
		file, link, ok = nginxSitePaths(lines, r.spec.Domain, kind.root)
	}
	if !ok {
		return "", "", &StepError{
			Message: "Настройка " + kind.name + " изменилась после осмотра",
			Advice:  "Файл сайта Wynd положить некуда. Вернитесь к осмотру и нажмите «Проверить снова» — он скажет, что не так.",
		}
	}
	return file, link, nil
}

// applySite кладёт файл сайта и просит веб-сервер его прочитать — одним
// сценарием: оборвись связь посередине, сервер сам доведёт дело до конца.
// Проверку настройка не прошла — возвращаем, как было до нас (прежний наш файл
// или ничего), и веб-сервер не трогаем.
func applySite(ctx context.Context, r *Run, file, link, content string) error {
	kind := proxyOf(r.spec.Proxy)
	enable, disable := "", ""
	if link != "" {
		enable = "ln -sfn " + file + " " + link
		disable = " " + link
	}
	// Запасные копии — в /opt/wynd: чужой Caddy читает из своей папки все файлы
	// подряд, лишний там станет частью настройки.
	script := `set -e
keep=` + installDir + `/site.prev
mkdir -p ` + installDir + ` ` + path.Dir(file) + `
rm -f "$keep"
if [ -f ` + file + ` ]; then cp -p ` + file + ` "$keep"; fi
cat > ` + installDir + `/site.new
mv ` + installDir + `/site.new ` + file + `
` + enable + `
` + kind.prepare + `
if ` + kind.test + ` 2>&1; then
  rm -f "$keep"
  ` + kind.reload + ` 2>&1
else
  if [ -f "$keep" ]; then mv "$keep" ` + file + `; else rm -f ` + file + disable + `; fi
  exit ` + fmt.Sprint(exitRejected) + `
fi`
	res, err := r.sess.Exec(ctx, script, strings.NewReader(content), shortTimeout)
	r.record("запись файла "+file+"\n"+kind.test+" && "+kind.reload, res.Output)
	if err != nil {
		return err
	}
	switch res.Code {
	case 0:
		return nil
	case exitRejected:
		return &StepError{
			Message: kind.name + " не принял настройку сайта Wynd",
			Advice:  "Свой файл мы убрали, " + kind.name + " не трогали — остальные сайты на сервере работают как раньше. Что ответил " + kind.name + ", видно в «подробностях».",
		}
	}
	return &commandError{command: script, output: res.Output, code: res.Code}
}

func certPresent(domain string) string {
	return "test -s /etc/letsencrypt/live/" + domain + "/fullchain.pem"
}

// siteStep — сайт Wynd в чужом веб-сервере. У nginx до сертификата — только
// ответ на проверку Let's Encrypt; полный сайт допишет шаг сертификата.
func siteStep(spec Spec) step {
	kind := proxyOf(spec.Proxy)
	return step{
		id:    "site",
		title: "Сайт в " + kind.name,
		done: func(ctx context.Context, r *Run) (bool, error) {
			file, _, err := sitePaths(ctx, r)
			if err != nil {
				return false, err
			}
			res, err := r.sess.Exec(ctx, "sha256sum "+file+" 2>/dev/null", nil, shortTimeout)
			if err != nil {
				return false, err
			}
			if spec.Proxy == ProxyCaddy {
				return strings.Contains(res.Output, sum(CaddySite(spec.Domain))), nil
			}
			return strings.Contains(res.Output, sum(siteContent(spec, true))) || strings.Contains(res.Output, sum(siteContent(spec, false))), nil
		},
		do: func(ctx context.Context, r *Run) error {
			file, link, err := sitePaths(ctx, r)
			if err != nil {
				return err
			}
			if spec.Proxy == ProxyCaddy {
				return applySite(ctx, r, file, link, CaddySite(spec.Domain))
			}
			if _, err := r.must(ctx, "mkdir -p "+acmeRoot, shortTimeout); err != nil {
				return err
			}
			has, err := r.ok(ctx, certPresent(spec.Domain))
			if err != nil {
				return err
			}
			return applySite(ctx, r, file, link, siteContent(spec, has))
		},
	}
}

// certbotScript ставит certbot, если его нет. Стоит и ведёт чужие сертификаты —
// пользуемся им же, чужие не трогаем.
const certbotScript = `command -v certbot >/dev/null 2>&1 || {
  export DEBIAN_FRONTEND=noninteractive
  apt-get -o DPkg::Lock::Timeout=600 install -y -qq certbot
}`

// certbotWait — certbot, который ждёт своей очереди. Запускать его можно по
// одному, а плановое продление держит замок до восьми минут: перед работой оно
// выжидает случайное время (проба 2026-10-10: и выпуск, и отзыв отвечали
// «Another instance of Certbot is already running»). Ждём до десяти минут.
const certbotWait = `cb() {
  n=0
  while :; do
    out=$(certbot "$@" 2>&1); code=$?
    case "$out" in
      *"Another instance of Certbot"*) n=$((n+1)); if [ "$n" -ge 60 ]; then break; fi; sleep 10 ;;
      *) break ;;
    esac
  done
  printf '%s\n' "$out"
  return $code
}
`

// certbotIssue получает сертификат через папку, которую отдаёт наш сайт в
// nginx: в настройку nginx certbot не заходит. После каждого продления nginx
// перечитывает настройку — иначе продолжит отдавать прежний сертификат.
func certbotIssue(spec Spec) string {
	cmd := certbotWait + "cb certonly --non-interactive --agree-tos --register-unsafely-without-email --webroot -w " + acmeRoot +
		" -d " + spec.Domain + " --deploy-hook '" + proxyOf(spec.Proxy).reload + "'"
	if spec.TestCert {
		cmd += " --test-cert"
	}
	return cmd + " && mkdir -p " + certOursDir + " && : > " + certOursDir + "/cert-" + spec.Domain
}

// waitSite ждёт, пока сайт откроется по HTTPS с действующим сертификатом.
func waitSite(ctx context.Context, r *Run, spec Spec, wait time.Duration) error {
	url := "https://" + spec.Domain + "/health"
	deadline := time.Now().Add(wait)
	for {
		_, err := r.probe(ctx, url)
		if err == nil {
			r.record("проверка "+url, "сайт открывается, сертификат действует")
			return nil
		}
		if errors.Is(err, ErrUnreachable) {
			return err
		}
		if time.Now().After(deadline) {
			r.record("проверка "+url, err.Error())
			return errSiteDown
		}
		if err := sleep(ctx, pollInterval); err != nil {
			return err
		}
	}
}

var errSiteDown = errors.New("installer: site does not answer")

// nginxCertStep — сертификат от Let's Encrypt через certbot и сайт на 443.
func nginxCertStep(spec Spec) step {
	return step{
		id:    "cert",
		title: "Сертификат для " + spec.Domain,
		do: func(ctx context.Context, r *Run) error {
			has, err := r.ok(ctx, certPresent(spec.Domain))
			if err != nil {
				return err
			}
			if !has {
				r.note("ставим certbot — он получает сертификаты")
				if _, err := r.must(ctx, certbotScript, packageTimeout); err != nil {
					return err
				}
				r.note("ждём ответа Let's Encrypt")
				out, err := r.must(ctx, certbotIssue(spec), 20*time.Minute)
				var failed *commandError
				switch {
				case errors.As(err, &failed) && containsAny(out, "too many certificates", "rateLimited", "too many failed authorizations"):
					return &StepError{
						Message: "Let's Encrypt пока не выдаёт сертификат для этого адреса",
						Advice:  "Для " + spec.Domain + " за последние дни запросили слишком много сертификатов — так бывает после нескольких установок подряд. Когда можно будет снова, видно в «подробностях». Сделанное осталось на месте: позже нажмите «Повторить».",
					}
				case errors.As(err, &failed) && containsAny(out, "Challenge failed", "unauthorized", "Timeout during connect", "DNS problem", "Connection refused"):
					return &StepError{
						Message: "Сертификат не выдали",
						Advice:  "Let's Encrypt не смог зайти на " + spec.Domain + " по порту 80. Проверьте, что входящий порт 80 открыт в панели хостинга (брандмауэр) и что адрес указывает на этот сервер, и нажмите «Повторить».",
					}
				case err != nil:
					return err
				}
			}
			file, link, err := sitePaths(ctx, r)
			if err != nil {
				return err
			}
			full := siteContent(spec, true)
			res, err := r.sess.Exec(ctx, "sha256sum "+file+" 2>/dev/null", nil, shortTimeout)
			if err != nil {
				return err
			}
			if !strings.Contains(res.Output, sum(full)) {
				if err := applySite(ctx, r, file, link, full); err != nil {
					return err
				}
			}
			if err := waitSite(ctx, r, spec, time.Minute); err != nil {
				if !errors.Is(err, errSiteDown) {
					return err
				}
				return &StepError{
					Message: "Сертификат получен, но сайт не открывается",
					Advice:  "Чаще всего входящий порт 443 закрыт в панели хостинга (брандмауэр). Откройте его и нажмите «Повторить».",
				}
			}
			return nil
		},
		explain: func(_ error, output string) *StepError {
			if e := aptBusy(output); e != nil {
				return e
			}
			return noInternet(output, "certbot")
		},
	}
}

// rollbackProxy — часть отката для чужого веб-сервера. Сама находит, что
// убирать: откат зовут и без плана, для сервера, где Wynd уже стоял. Свои
// файлы сайта узнаёт по первой строке, адрес сайта читает из compose.yaml.
// Веб-сервер перечитывает настройку только после своей проверки; не прошла
// (чужая ошибка) — наш файл уже убран, а перечитает он её при следующем случае.
func rollbackProxy(keepData bool) string {
	script := `dom=$(sed -n 's|.*WYND_PUBLIC_URL: https://||p' ` + installDir + `/compose.yaml 2>/dev/null | head -n 1)
mark=` + shQuote(siteMark) + `
ngx=; cdy=; apa=
for f in /etc/apache2/sites-enabled/wynd-*.conf /etc/apache2/sites-available/wynd-*.conf; do
  if grep -qsF "$mark" "$f"; then rm -f "$f"; apa=1; fi
done
for f in /etc/nginx/sites-enabled/wynd-*.conf /etc/nginx/sites-available/wynd-*.conf /etc/nginx/conf.d/wynd-*.conf; do
  if grep -qsF "$mark" "$f"; then rm -f "$f"; ngx=1; fi
done
for f in $(grep -rlsF "$mark" ` + path.Dir(caddyConfig) + ` 2>/dev/null); do
  case "$(basename "$f")" in wynd-*) rm -f "$f"; cdy=1 ;; esac
done
if [ -n "$ngx" ]; then
  if nginx -t 2>&1; then systemctl reload nginx 2>&1 || echo ` + markNotReloaded + `; else echo ` + markNotReloaded + `; fi
fi
if [ -n "$apa" ]; then
  if apache2ctl configtest 2>&1; then systemctl reload apache2 2>&1 || echo ` + markNotReloaded + `; else echo ` + markNotReloaded + `; fi
fi
if [ -n "$cdy" ]; then
  if ` + proxyOf(ProxyCaddy).test + ` 2>&1; then systemctl reload caddy 2>&1 || echo ` + markNotReloaded + `; else echo ` + markNotReloaded + `; fi
fi
`
	if keepData {
		return script
	}
	// Данные удаляют — сертификат, который получили мы, отзываем; certbot после
	// отзыва сам убирает его из продления. Сам certbot остаётся, как Docker.
	// У чужого Caddy сертификат адреса Wynd лежит в его хранилище: наш сайт из
	// настройки уже убран, Caddy им больше не занят — отзываем так же, как из
	// тома своего Caddy, и убираем из хранилища вместе с ключом.
	return script + certbotWait + `if [ -n "$dom" ] && [ -e ` + certOursDir + `/cert-"$dom" ]; then
  if cb revoke --non-interactive --cert-name "$dom" --delete-after-revoke --reason cessationofoperation; then
    echo "` + markRevoked + ` $dom"
  else
    echo "` + markRevokeFailed + ` $dom"
    cb delete --non-interactive --cert-name "$dom" || true
  fi
fi
rm -rf ` + certOursDir + `
if [ -n "$cdy" ] && [ -n "$dom" ] && command -v docker >/dev/null 2>&1; then
  store=$(getent passwd caddy | cut -d: -f6)/.local/share/caddy
  if ls "$store"/certificates/*/"$dom"/*.crt >/dev/null 2>&1; then
    docker run --rm -e DOM="$dom" -v "$store":/data/caddy:ro --entrypoint sh ` + certbotImage + ` -c '` + revokeLoop + `' 2>&1 || echo "` + markRevokeFailed + ` certbot did not run"
    docker image rm ` + certbotImage + ` >/dev/null 2>&1 || true
    rm -rf "$store"/certificates/*/"$dom"
  fi
fi
`
}
