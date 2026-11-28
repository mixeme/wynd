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
	Domain  string           `json:"domain"`
	Cache   string           `json:"cache"`
	Change  *pushCacheChange `json:"change,omitempty"`
	Removal *pushRemoval     `json:"removal,omitempty"`
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
	if !validPushCache(cache) {
		return pushSettings{}, errors.New("Выберите один из доступных сроков хранения сообщений.")
	}
	return pushSettings{Domain: domain, Cache: cache}, nil
}

func validPushCache(cache string) bool {
	switch cache {
	case "0", "1h", "6h", "12h", "24h", "72h":
		return true
	}
	return false
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
	return []installFile{p.metadata(l), p.ntfyFile(l), p.composeFile(l)}
}

func (p pushSettings) metadata(l Layout) installFile {
	settings, _ := json.Marshal(p)
	return installFile{l.Root() + "/push.json", string(settings) + "\n"}
}

func (p pushSettings) ntfyFile(l Layout) installFile {
	return installFile{l.Root() + "/ntfy.yml", fmt.Sprintf(`base-url: %q
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
`, p.origin(), p.Cache)}
}

func (p pushSettings) composeFile(l Layout) installFile {
	return installFile{l.Root() + "/push.yaml", fmt.Sprintf(`services:
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
`, p.origin(), pushImage)}
}

var pushComposeVariable = regexp.MustCompile(`^(?:export[ \t]+)?(COMPOSE_FILE|COMPOSE_PATH_SEPARATOR)(?:[ \t]*[=:]|[ \t]*$)`)

func splitPushEnv(content string) (rest string, managed bool, err error) {
	lines := strings.SplitAfter(content, "\n")
	other := strings.Builder{}
	for _, line := range lines {
		value := strings.TrimSpace(line)
		if !pushComposeVariable.MatchString(value) {
			other.WriteString(line)
			continue
		}
		if value != pushComposeLine || managed {
			return "", false, errors.New("В .env задан другой состав Docker Compose. Проверьте этот файл.")
		}
		managed = true
	}
	return other.String(), managed, nil
}

// mergePushEnv preserves the other dotenv values byte-for-byte.
func mergePushEnv(content string) (string, error) {
	_, managed, err := splitPushEnv(content)
	if err != nil {
		return "", err
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
	content, err := readPushFile(ctx, r.sess, envPath)
	if err != nil {
		return nil, "", err
	}
	content, err = mergePushEnv(content)
	if err != nil {
		return nil, "", err
	}
	return append(files, installFile{envPath, content}), dirs, nil
}

// A missing file returns an empty string. Reads go directly through SSH so
// .env contents never enter the Workshop operation log.
func readPushFile(ctx context.Context, sess *Session, path string) (string, error) {
	q := ShellPath(path)
	read := fmt.Sprintf(`if [ -L %[1]s ]; then exit 2; elif [ -e %[1]s ]; then
  test -f %[1]s && test "$(wc -c < %[1]s)" -le %[2]d && cat %[1]s
fi`, q, maxCommandOutput-1)
	result, err := sess.Exec(ctx, read, nil, shortTimeout)
	if err != nil {
		return "", err
	}
	if result.Code != 0 {
		return "", &StepError{Message: "Не удалось безопасно прочитать файл установки", Advice: path}
	}
	return result.Output, nil
}

func pushHealthStep(settings pushSettings) step {
	return step{
		id:    "push-health",
		title: "Сервер уведомлений",
		do: func(ctx context.Context, r *Run) error {
			r.note("ждём сертификат и ответ сервера уведомлений")
			return waitPushHealthy(ctx, r, settings, certWait)
		},
	}
}

func pushHealthy(ctx context.Context, r *Run, p pushSettings) bool {
	var status struct {
		Healthy bool `json:"healthy"`
	}
	body, err := r.probe(ctx, p.origin()+"/v1/health")
	return err == nil && json.Unmarshal([]byte(body), &status) == nil && status.Healthy
}

func waitPushHealthy(ctx context.Context, r *Run, p pushSettings, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		if pushHealthy(waitCtx, r, p) {
			return nil
		}
		if err := sleep(waitCtx, pollInterval); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return &StepError{
				Message: "Сервер уведомлений не ответил по HTTPS",
				Advice:  "Проверьте, что " + p.Domain + " указывает на этот сервер и доступен по портам 80 и 443.",
			}
		}
	}
}
