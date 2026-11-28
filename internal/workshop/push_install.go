package workshop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/config"
)

const (
	pushImage        = "binwiederhier/ntfy:v2.38.3"
	defaultPushCache = "12h"
	pushComposeLine  = "COMPOSE_FILE=compose.yaml:push.yaml"
)

type pushSettings struct {
	Domain string `json:"domain"`
	Cache  string `json:"cache"`
}

func (s Spec) pushSettings() (pushSettings, error) {
	if s.PushDomain == "" && s.PushCache == "" {
		return pushSettings{}, nil
	}
	if s.Proxy != "" {
		return pushSettings{}, errors.New("Подключение Push-сервера требует Caddy, установленного Мастерской.")
	}
	domain := NormalizeDomain(s.PushDomain)
	if _, ok := config.NormalizePushServer("https://" + domain); !ok || domain == NormalizeDomain(s.Domain) {
		return pushSettings{}, errors.New("Укажите отдельное публичное имя, например push.family.example.ru.")
	}
	cache := s.PushCache
	if cache == "" {
		cache = defaultPushCache
	}
	switch cache {
	case "0", "1h", "6h", "12h", "24h", "72h":
		return pushSettings{Domain: domain, Cache: cache}, nil
	default:
		return pushSettings{}, errors.New("Выберите один из доступных сроков хранения сообщений.")
	}
}

func (p pushSettings) origin() string { return "https://" + p.Domain }

func (p pushSettings) caddySite() string {
	return "\n# BEGIN Wynd Push\n" + p.Domain + ` {
	header Strict-Transport-Security "max-age=31536000"
	reverse_proxy push:80
}
# END Wynd Push
`
}

func (p pushSettings) installFiles(l Layout) []installFile {
	// The struct contains only strings, which cannot fail to marshal.
	settings, _ := json.Marshal(p)
	return []installFile{
		{l.Root() + "/push.json", string(settings) + "\n"},
		{l.Root() + "/ntfy.yml", fmt.Sprintf(`base-url: %q
listen-http: ":80"
behind-proxy: true
cache-duration: %q
auth-file: "/tmp/wynd-ntfy-users.db"
auth-default-access: deny-all
auth-access: ["*:up*:rw"]
enable-signup: false
enable-login: false
enable-reservations: false
web-root: disable
attachment-cache-dir: ""
enable-metrics: false
visitor-subscriber-rate-limiting: true
`, p.origin(), p.Cache)},
		{l.Root() + "/push.yaml", fmt.Sprintf(`services:
  wynd:
    environment:
      WYND_PUSH_SERVER: %q
  push:
    image: %s
    command: ["serve"]
    volumes:
      - ./ntfy.yml:/etc/ntfy/server.yml:ro
    logging:
      driver: none
    restart: unless-stopped
`, p.origin(), pushImage)},
	}
}

var pushComposeVariable = regexp.MustCompile(`^(?:export[ \t]+)?(COMPOSE_FILE|COMPOSE_PATH_SEPARATOR)(?:[ \t]*[=:]|[ \t]*$)`)

// mergePushEnv preserves the other dotenv values byte-for-byte. A Compose
// override already chosen by an operator belongs to that operator.
func mergePushEnv(content string) (string, error) {
	managed := false
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		match := pushComposeVariable.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if line != pushComposeLine || managed {
			return "", errors.New("В .env уже задан состав Docker Compose. Проверьте его перед установкой Push-сервера.")
		}
		managed = true
	}
	if managed {
		return content, nil
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + pushComposeLine + "\n", nil
}

func installFilesForRun(ctx context.Context, r *Run, spec Spec) ([]installFile, string, error) {
	files, dirs := installFilesAt(r.layout, spec)
	if spec.PushDomain == "" {
		return files, dirs, nil
	}
	envPath := r.layout.Root() + "/.env"
	q := ShellPath(envPath)
	read := fmt.Sprintf(`if [ -L %[1]s ]; then exit 2; elif [ -e %[1]s ]; then
  test -f %[1]s && test "$(wc -c < %[1]s)" -le %[2]d && cat %[1]s
fi`, q, maxCommandOutput-1)
	result, err := r.sess.Exec(ctx, read, nil, shortTimeout)
	if err != nil {
		return nil, "", err
	}
	if result.Code != 0 {
		return nil, "", errors.New("Не удалось безопасно прочитать .env установки.")
	}
	content, err := mergePushEnv(result.Output)
	if err != nil {
		return nil, "", err
	}
	return append(files, installFile{envPath, content}), dirs, nil
}

func pushHealthStep(settings pushSettings) step {
	return step{
		id:    "push-health",
		title: "Сервер уведомлений",
		do: func(ctx context.Context, r *Run) error {
			r.note("ждём сертификат и ответ сервера уведомлений")
			deadline := time.Now().Add(certWait)
			for {
				var status struct {
					Healthy bool `json:"healthy"`
				}
				body, err := r.probe(ctx, settings.origin()+"/v1/health")
				if err == nil && json.Unmarshal([]byte(body), &status) == nil && status.Healthy {
					return nil
				}
				if time.Now().After(deadline) {
					return &StepError{
						Message: "Сервер уведомлений не ответил по HTTPS",
						Advice:  "Проверьте, что " + settings.Domain + " указывает на этот сервер и доступен по портам 80 и 443, затем нажмите «Повторить».",
					}
				}
				if err := sleep(ctx, pollInterval); err != nil {
					return err
				}
			}
		},
	}
}
