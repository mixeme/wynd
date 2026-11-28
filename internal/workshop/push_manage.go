package workshop

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

const (
	PushSaveCache      = "cache"
	PushRemove         = "remove"
	composeKeepOrphans = "COMPOSE_REMOVE_ORPHANS=false docker compose "
)

// State saved before a remote mutation so a retry can finish after loss of SSH.
type pushCacheChange struct {
	Cache   string `json:"cache"`
	Running bool   `json:"running"`
}

type pushRemoval struct {
	WyndRunning  bool `json:"wynd_running"`
	CaddyRunning bool `json:"caddy_running"`
}

type PushState struct {
	Domain       string `json:"domain"`
	Cache        string `json:"cache"`
	Version      string `json:"version,omitempty"`
	Running      bool   `json:"running"`
	Healthy      bool   `json:"healthy"`
	PendingCache string `json:"pending_cache,omitempty"`
	Removing     bool   `json:"removing,omitempty"`
	Adding       bool   `json:"adding,omitempty"`
}

func pushFilesChanged() error {
	return &StepError{
		Message: "Настройки Push-сервера отличаются от записанных Мастерской",
		Advice:  "Проверьте push.json, push.yaml, ntfy.yml, .env и Caddyfile в каталоге установки.",
	}
}

func checkPushRoot(ctx context.Context, sess *Session, l Layout) error {
	guard := rootIdentityGuard(l) + `test ! -L "$root" && cd "$root" &&
test ! -L compose.yaml && grep -qF ` + shQuote(installMark) + ` compose.yaml &&
echo WYND_PUSH_ROOT`
	result, err := sess.Exec(ctx, guard, nil, shortTimeout)
	if err != nil {
		return err
	}
	if result.Code != 0 || strings.TrimSpace(result.Output) != "WYND_PUSH_ROOT" {
		return &StepError{Message: "Каталог установки Wynd изменился", Advice: "Подключитесь к серверу заново и проверьте каталог установки."}
	}
	return nil
}

func readPushSettings(ctx context.Context, r *Run) (pushSettings, error) {
	l := r.layout
	if err := checkPushRoot(ctx, r.sess, l); err != nil {
		return pushSettings{}, err
	}
	return loadPushSettings(ctx, r.sess, l)
}

func loadPushSettings(ctx context.Context, sess *Session, l Layout) (pushSettings, error) {
	raw, err := readPushFile(ctx, sess, l.Root()+"/push.json")
	if err != nil {
		return pushSettings{}, err
	}
	var p pushSettings
	if json.Unmarshal([]byte(raw), &p) != nil || !validPushCache(p.Cache) {
		return pushSettings{}, pushFilesChanged()
	}
	checked, err := (Spec{PushDomain: p.Domain, PushCache: p.Cache}).pushSettings()
	if err != nil || checked.Domain == "" || checked.Domain != p.Domain ||
		(p.Change != nil && !validPushCache(p.Change.Cache)) ||
		(p.Adding != nil && (p.Change != nil || p.Removal != nil || !p.Adding.valid())) {
		return pushSettings{}, pushFilesChanged()
	}
	return p, nil
}

func (l Layout) pushCompose() string {
	return "cd " + l.shellRoot() + " && " + composeKeepOrphans + "-f compose.yaml -f push.yaml "
}

// The main site's start button and move rollback used to start every service.
// A Push service has its own stop state.
func (l Layout) startWynd(options string) string {
	up := "docker compose up -d" + options
	return "cd " + l.shellRoot() + " && { if [ -e push.json ] || [ -L push.json ]; then " +
		composeKeepOrphans + "up -d" + options + " wynd caddy; else " + up + "; fi; }"
}

func queryPushContainer(ctx context.Context, r *Run) (running bool, version string, err error) {
	command := "id=$(" + r.layout.pushCompose() + "ps --all --quiet push 2>/dev/null) && " +
		`{ if [ -n "$id" ]; then docker inspect --format '{{.State.Running}} {{.Config.Image}}' "$id"; fi; }`
	result, err := r.sess.Exec(ctx, command, nil, shortTimeout)
	if err != nil {
		return false, "", err
	}
	fields := strings.Fields(result.Output)
	if result.Code != 0 || (len(fields) != 0 && (len(fields) != 2 || (fields[0] != "true" && fields[0] != "false"))) {
		return false, "", &StepError{Message: "Не удалось прочитать состояние контейнера Push"}
	}
	if len(fields) == 0 {
		return false, "", nil
	}
	image, _, _ := strings.Cut(fields[1], "@")
	if i := strings.LastIndexByte(image, ':'); i > strings.LastIndexByte(image, '/') {
		version = strings.TrimPrefix(image[i+1:], "v")
	}
	return fields[0] == "true", version, nil
}

func readPushState(ctx context.Context, r *Run) (*PushState, error) {
	p, err := readPushSettings(ctx, r)
	if err != nil {
		return nil, err
	}
	st := &PushState{Domain: p.Domain, Cache: p.Cache, Removing: p.Removal != nil, Adding: p.Adding != nil}
	if st.Removing || st.Adding {
		return st, nil
	}
	if p.Change != nil {
		st.PendingCache = p.Change.Cache
	}
	st.Running, st.Version, err = queryPushContainer(ctx, r)
	if err == nil && st.Running {
		probeCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		st.Healthy = pushHealthy(probeCtx, r, p)
	}
	return st, err
}

func pushActionAt(ctx context.Context, sess *Session, l Layout, action, cache string) ([]LogEntry, error) {
	switch action {
	case ServiceStart, ServiceStop, ServiceRestart, PushRemove:
		if cache != "" {
			return nil, ErrInvalid
		}
	case PushSaveCache:
		if !validPushCache(cache) {
			return nil, &StepError{Message: "Выберите один из доступных сроков хранения сообщений."}
		}
	default:
		return nil, ErrInvalid
	}
	r := &Run{sess: sess, layout: l, steps: []StepState{{ID: action}}, report: func() {}}
	err := managePush(ctx, r, action, cache)
	return r.log, err
}

func managePush(ctx context.Context, r *Run, action, cache string) error {
	p, err := readPushSettings(ctx, r)
	if err != nil {
		return err
	}
	if p.Adding != nil {
		return &StepError{Message: "Добавление сервера уведомлений не завершено", Advice: "Продолжите добавление из пульта."}
	}
	if action == PushRemove {
		return removePush(ctx, r, p)
	}
	if p.Removal != nil {
		return &StepError{Message: "Удаление Push-сервера не завершилось", Advice: "Повторите удаление в настройках Push-сервера."}
	}
	if err := checkPushFiles(ctx, r, p); err != nil {
		return err
	}
	if action == PushSaveCache {
		return savePushCache(ctx, r, p, cache)
	}
	if p.Change != nil {
		return &StepError{Message: "Срок хранения сообщений ещё не сохранён", Advice: "Повторите сохранение срока хранения в настройках Push-сервера."}
	}
	command := r.layout.pushCompose()
	switch action {
	case ServiceStart:
		command += "up -d --no-deps push"
	case ServiceStop:
		command += "stop push"
	case ServiceRestart:
		command += "restart --no-deps push"
	}
	if _, err := r.must(ctx, command+" 2>&1", packageTimeout); err != nil {
		return err
	}
	if action == ServiceStop {
		return nil
	}
	return waitPushHealthy(ctx, r, p, healthWait)
}

func checkPushFiles(ctx context.Context, r *Run, p pushSettings) error {
	override := p.composeFile(r.layout)
	got, err := readPushFile(ctx, r.sess, override.path)
	if err != nil {
		return err
	}
	if got != override.content && !(p.Removal != nil && got == "") {
		return pushFilesChanged()
	}
	config := p.ntfyFile(r.layout)
	got, err = readPushFile(ctx, r.sess, config.path)
	if err != nil {
		return err
	}
	if got == config.content || (p.Removal != nil && got == "") {
		return nil
	}
	if p.Change != nil {
		p.Cache = p.Change.Cache
		if got == p.ntfyFile(r.layout).content {
			return nil
		}
	}
	return pushFilesChanged()
}

func savePushCache(ctx context.Context, r *Run, p pushSettings, cache string) error {
	if p.Change == nil {
		if p.Cache == cache {
			return nil
		}
		running, _, err := queryPushContainer(ctx, r)
		if err != nil {
			return err
		}
		p.Change = &pushCacheChange{Cache: cache, Running: running}
		mark := p.metadata(r.layout)
		if err := r.put(ctx, mark.path, mark.content); err != nil {
			return err
		}
	} else if cache != p.Change.Cache {
		return &StepError{
			Message: "Смена срока хранения сообщений не завершилась",
			Advice:  "Сначала повторите сохранение ранее выбранного значения.",
		}
	}

	p.Cache = p.Change.Cache
	config := p.ntfyFile(r.layout)
	if err := r.put(ctx, config.path, config.content); err != nil {
		return err
	}
	if p.Change.Running {
		// The old container still has the previous file inode bind-mounted.
		if _, err := r.must(ctx, r.layout.pushCompose()+"up -d --no-deps --force-recreate push 2>&1", packageTimeout); err != nil {
			return err
		}
		if err := waitPushHealthy(ctx, r, p, healthWait); err != nil {
			return err
		}
	}
	p.Change = nil
	mark := p.metadata(r.layout)
	return r.put(ctx, mark.path, mark.content)
}

func removePush(ctx context.Context, r *Run, p pushSettings) error {
	if err := checkPushFiles(ctx, r, p); err != nil {
		return err
	}
	l := r.layout
	envPath := l.Root() + "/.env"
	env, err := readPushFile(ctx, r.sess, envPath)
	if err != nil {
		return err
	}
	otherEnv, hasLine, err := splitPushEnv(env)
	if err != nil || (!hasLine && p.Removal == nil) {
		return pushFilesChanged()
	}
	caddy, err := readPushFile(ctx, r.sess, l.Caddyfile())
	if err != nil {
		return err
	}
	otherCaddy := strings.Replace(caddy, p.caddySite(), "", 1)
	if strings.TrimSpace(otherCaddy) == "" || strings.Contains(otherCaddy, "# BEGIN Wynd Push") ||
		strings.Contains(otherCaddy, "# END Wynd Push") || (caddy == otherCaddy && p.Removal == nil) {
		return pushFilesChanged()
	}

	base := "cd " + l.shellRoot() + " && " + composeKeepOrphans + "-f compose.yaml "
	if p.Removal == nil {
		// Validate the ordinary project before taking away the override.
		if _, err := r.must(ctx, base+"config --quiet 2>&1", shortTimeout); err != nil {
			return err
		}
		wynd, err := r.must(ctx, base+"ps --quiet wynd 2>/dev/null", shortTimeout)
		if err != nil {
			return err
		}
		caddy, err := r.must(ctx, base+"ps --quiet caddy 2>/dev/null", shortTimeout)
		if err != nil {
			return err
		}
		p.Removal = &pushRemoval{
			WyndRunning:  strings.TrimSpace(wynd) != "",
			CaddyRunning: strings.TrimSpace(caddy) != "",
		}
		mark := p.metadata(l)
		if err := r.put(ctx, mark.path, mark.content); err != nil {
			return err
		}
	}
	if _, err := r.must(ctx, "cd "+l.shellRoot()+` && { if [ -f push.yaml ]; then `+
		composeKeepOrphans+"-f compose.yaml -f push.yaml rm --stop --force push; fi; } 2>&1", packageTimeout); err != nil {
		return err
	}
	if hasLine {
		if err := r.put(ctx, envPath, otherEnv); err != nil {
			return err
		}
	}
	if caddy != otherCaddy {
		if err := r.put(ctx, l.Caddyfile(), otherCaddy); err != nil {
			return err
		}
	}
	for _, service := range []struct {
		name    string
		running bool
	}{
		{"wynd", p.Removal.WyndRunning},
		{"caddy", p.Removal.CaddyRunning},
	} {
		command := base + "up -d --no-deps"
		if service.name == "caddy" {
			command += " --force-recreate"
		}
		if !service.running {
			command += " --no-start"
		}
		if _, err := r.must(ctx, command+" "+service.name+" 2>&1", packageTimeout); err != nil {
			return err
		}
	}
	if p.Removal.WyndRunning {
		if err := waitHealthy(ctx, r); err != nil {
			return err
		}
	}
	_, err = r.must(ctx, "cd "+l.shellRoot()+" && rm -f -- push.yaml ntfy.yml && rm -f -- push.json", shortTimeout)
	return err
}
