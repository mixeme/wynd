package workshop

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

// Only digests are persisted. In particular, push.json must never hold .env values.
type pushAddition struct {
	Env        string `json:"env_sha256"`
	Caddy      string `json:"caddy_sha256"`
	Compose    string `json:"compose_sha256"`
	EnvPresent bool   `json:"env_present"`
}

const pushAddPendingMessage = "Сначала завершите добавление сервера уведомлений"

func (w *Wizard) pushAddPendingLocked() bool {
	return w.state != nil && w.state.Push != nil && w.state.Push.Adding
}

func (p pushAddition) valid() bool {
	env, errEnv := hex.DecodeString(p.Env)
	caddy, errCaddy := hex.DecodeString(p.Caddy)
	compose, errCompose := hex.DecodeString(p.Compose)
	return errEnv == nil && len(env) == 32 && errCaddy == nil && len(caddy) == 32 && errCompose == nil && len(compose) == 32
}

// ApplyPushAdd rechecks the selected installation after confirmation and uses
// the same profile reservation as the other service commands.
func (w *Wizard) ApplyPushAdd(ctx context.Context, domain string) ServiceResult {
	return w.manageService(ctx, func(sess *Session) ([]LogEntry, error) {
		checked, layout, settings, err := w.preparePushAdd(ctx, domain)
		if err != nil {
			return nil, err
		}
		if checked != sess {
			return nil, &StepError{Message: "Подключение изменилось", Advice: "Составьте план снова."}
		}
		r := &Run{sess: sess, layout: layout, steps: []StepState{{ID: "push-add"}}, report: func() {},
			spec: Spec{Probe: w.Probe, TestCert: w.TestCert}}
		err = applyPushAdd(ctx, r, settings)
		return r.log, err
	})
}

type pushAddFiles struct {
	env, caddy installFile
	baseEnv    string
	baseCaddy  string
}

func readPushAddFiles(ctx context.Context, r *Run, p *pushSettings) (pushAddFiles, error) {
	f := pushAddFiles{env: installFile{path: r.layout.Root() + "/.env"}, caddy: installFile{path: r.layout.Caddyfile()}}
	compose, err := readPushFile(ctx, r.sess, r.layout.Compose())
	if err != nil {
		return f, err
	}
	if !strings.Contains(compose, installMark) || (p.Adding != nil && sum(compose) != p.Adding.Compose) {
		return f, pushFilesChanged()
	}
	f.env.content, err = readPushFile(ctx, r.sess, f.env.path)
	if err != nil {
		return f, err
	}
	f.caddy.content, err = readPushFile(ctx, r.sess, f.caddy.path)
	if err != nil {
		return f, err
	}
	f.baseEnv, f.baseCaddy = f.env.content, f.caddy.content
	if p.Adding == nil {
		if _, managed, err := splitPushEnv(f.baseEnv); err != nil || managed ||
			strings.TrimSpace(f.baseCaddy) == "" || strings.Contains(f.baseCaddy, "# BEGIN Wynd Push") ||
			strings.Contains(f.baseCaddy, "# END Wynd Push") {
			return f, pushFilesChanged()
		}
		exists, err := r.ok(ctx, "test -f "+ShellPath(f.env.path))
		if err != nil {
			return f, err
		}
		p.Adding = &pushAddition{Env: sum(f.baseEnv), Caddy: sum(f.baseCaddy), Compose: sum(compose), EnvPresent: exists}
		return f, nil
	}
	if sum(f.baseEnv) != p.Adding.Env {
		other, managed, err := splitPushEnv(f.env.content)
		if err != nil || !managed {
			return f, pushFilesChanged()
		}
		f.baseEnv = other
		if sum(f.baseEnv) != p.Adding.Env {
			f.baseEnv = strings.TrimSuffix(other, "\n")
		}
		merged, err := mergePushEnv(f.baseEnv)
		if err != nil || sum(f.baseEnv) != p.Adding.Env || merged != f.env.content {
			return f, pushFilesChanged()
		}
	}
	if sum(f.baseCaddy) != p.Adding.Caddy {
		base, ok := strings.CutSuffix(f.caddy.content, p.caddySite())
		if !ok || sum(base) != p.Adding.Caddy {
			return f, pushFilesChanged()
		}
		f.baseCaddy = base
	}
	return f, nil
}

// The Compose files, project and live bind mounts must belong to the same
// installation. Refuse an orphan Push service instead of taking ownership.
func checkPushAddProject(ctx context.Context, r *Run, pending bool) error {
	l := r.layout
	badProject := &StepError{Message: "Не удалось подтвердить состав установки Wynd", Advice: "Проверьте контейнеры и файлы Docker Compose на сервере."}
	rendered, err := r.sess.Exec(ctx, l.ComposeCommand()+"-f compose.yaml config --format json 2>/dev/null", nil, shortTimeout)
	if err != nil {
		return err
	}
	var config struct {
		Name     string `json:"name"`
		Services map[string]struct {
			Image   string `json:"image"`
			Volumes []struct {
				Type   string `json:"type"`
				Source string `json:"source"`
				Target string `json:"target"`
			} `json:"volumes"`
		} `json:"services"`
	}
	if rendered.Code != 0 || json.Unmarshal([]byte(rendered.Output), &config) != nil || config.Name == "" {
		return badProject
	}
	for _, bind := range []struct{ service, source, target string }{
		{"wynd", l.Data(), "/data"}, {"caddy", l.Caddyfile(), "/etc/caddy/Caddyfile"},
	} {
		matched := false
		for _, volume := range config.Services[bind.service].Volumes {
			if volume.Target == bind.target {
				matched = volume.Type == "bind" && volume.Source == bind.source
			}
		}
		if !matched || config.Services[bind.service].Image == "" {
			return badProject
		}
	}
	if _, exists := config.Services["push"]; exists {
		return pushFilesChanged()
	}
	mode := "no"
	if pending {
		mode = "yes"
	}
	command := rootIdentityGuard(l) + `test ! -L "$root" && cd "$root" || exit 2
set -e
wynd=$(docker compose -f compose.yaml ps --all --quiet wynd)
caddy=$(docker compose -f compose.yaml ps --all --quiet caddy)
project=` + shQuote(config.Name) + `
pending=` + mode + `
for id in "$wynd" "$caddy"; do
  if [ -z "$id" ]; then test "$pending" = yes || exit 2; continue; fi
  test "$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project"}}' "$id")" = "$project"
  test "$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project.working_dir"}}' "$id")" = "$root"
  config=$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project.config_files"}}' "$id")
  case "$config" in "$root/compose.yaml"|"$root/compose.yaml,$root/push.yaml") ;; *) exit 2 ;; esac
done
if [ -n "$wynd" ]; then
  test "$(docker inspect -f '{{.Config.Image}}' "$wynd")" = ` + shQuote(config.Services["wynd"].Image) + `
  test "$(docker inspect -f '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Source}}{{end}}{{end}}' "$wynd")" = "$root/data"
fi
if [ -n "$caddy" ]; then
  test "$(docker inspect -f '{{.Config.Image}}' "$caddy")" = ` + shQuote(config.Services["caddy"].Image) + `
  test "$(docker inspect -f '{{range .Mounts}}{{if eq .Destination "/etc/caddy/Caddyfile"}}{{.Source}}{{end}}{{end}}' "$caddy")" = "$root/Caddyfile"
fi
push=$(docker ps -aq --filter "label=com.docker.compose.project=$project" --filter label=com.docker.compose.service=push)
if [ -n "$push" ]; then
`
	if pending {
		command += `  test "$(docker inspect -f '{{.Config.Image}}' "$push")" = ` + shQuote(pushImage) + `
  test "$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project.working_dir"}}' "$push")" = "$root"
  test "$(docker inspect -f '{{range .Mounts}}{{if eq .Destination "/etc/ntfy/server.yml"}}{{.Source}}{{end}}{{end}}' "$push")" = "$root/ntfy.yml"
`
	} else {
		command += "  exit 3\n"
	}
	result, err := r.sess.Exec(ctx, command+"fi\necho WYND_PUSH_PROJECT", nil, shortTimeout)
	if err != nil {
		return err
	}
	if result.Code != 0 || strings.TrimSpace(result.Output) != "WYND_PUSH_PROJECT" {
		return badProject
	}
	return nil
}

// Check the current bytes immediately before an atomic write. New files are
// published exclusively so a foreign file or dangling symlink stays intact.
func editPushAddFile(ctx context.Context, r *Run, f installFile, previous *string, remove bool) error {
	guard := rootIdentityGuard(r.layout) + `test ! -L "$root" && cd "$root" || exit 2
test ! -L compose.yaml && grep -qF ` + shQuote(installMark) + ` compose.yaml || exit 2
file=` + ShellPath(f.path) + `
check() {
  test ! -L "$file" || return 1
`
	if previous == nil {
		guard += `  test ! -e "$file"` + "\n"
	} else {
		guard += `  test -f "$file" || return 1
  digest=$(sha256sum -- "$file") && test "${digest%% *}" = ` + shQuote(sum(*previous)) + "\n"
	}
	guard += "}\n"
	if remove {
		guard += `check && rm -- "$file" || exit 2`
	} else {
		guard += `umask 077
tmp=$(mktemp -- "$file.XXXXXX") || exit 2
trap 'rm -f -- "$tmp"' EXIT
cat > "$tmp" && check || exit 2
`
		if previous == nil {
			if !strings.HasSuffix(f.path, "/.env") {
				guard += "chmod 644 -- \"$tmp\" || exit 2\n"
			}
			guard += `ln -T -- "$tmp" "$file" || exit 2`
		} else {
			guard += `chmod --reference="$file" -- "$tmp" && mv -T -- "$tmp" "$file" || exit 2`
		}
	}
	res, err := r.sess.Exec(ctx, guard+"\necho WYND_PUSH_FILE", strings.NewReader(f.content), shortTimeout)
	r.record("настройка файла "+f.path, "")
	if err != nil {
		return err
	}
	if res.Code != 0 || strings.TrimSpace(res.Output) != "WYND_PUSH_FILE" {
		return pushFilesChanged()
	}
	return nil
}

func applyPushAdd(ctx context.Context, r *Run, p pushSettings) error {
	pending := p.Adding != nil
	if err := checkPushAddProject(ctx, r, pending); err != nil {
		return err
	}
	files, err := readPushAddFiles(ctx, r, &p)
	if err != nil {
		return err
	}
	marker := p.metadata(r.layout)
	if !pending {
		if err := editPushAddFile(ctx, r, marker, nil, false); err != nil {
			return err
		}
	}
	if err := activatePushAdd(ctx, r, p, files); err != nil {
		var step *StepError
		message := "Не удалось добавить сервер уведомлений"
		advice := "Прежние настройки Wynd восстановлены."
		if errors.As(err, &step) {
			message = step.Message
			if step.Advice != "" {
				advice += " " + step.Advice
			}
		}
		if restorePushAdd(ctx, r, p) == nil {
			return &StepError{Message: message, Advice: advice}
		}
		return &StepError{Message: message, Advice: "Добавление не завершено. Проверьте подключение и повторите его для того же домена."}
	}
	p.Adding = nil
	return editPushAddFile(ctx, r, p.metadata(r.layout), &marker.content, false)
}

func activatePushAdd(ctx context.Context, r *Run, p pushSettings, files pushAddFiles) error {
	l := r.layout
	for _, file := range []installFile{p.ntfyFile(l), p.composeFile(l)} {
		got, err := readPushFile(ctx, r.sess, file.path)
		if err != nil {
			return err
		}
		if got == file.content {
			continue
		}
		if got != "" {
			return pushFilesChanged()
		}
		if err := editPushAddFile(ctx, r, file, nil, false); err != nil {
			return err
		}
	}
	compose := l.pushCompose()
	if _, err := r.must(ctx, compose+"config --quiet 2>&1", shortTimeout); err != nil {
		return err
	}
	if _, err := r.must(ctx, compose+"up -d --no-deps push 2>&1", packageTimeout); err != nil {
		return err
	}
	if files.caddy.content == files.baseCaddy {
		caddy := installFile{files.caddy.path, files.baseCaddy + p.caddySite()}
		check := compose + "run --rm -T --no-deps --pull never --entrypoint caddy caddy validate --config /dev/stdin --adapter caddyfile 2>&1"
		result, err := r.sess.Exec(ctx, check, strings.NewReader(caddy.content), packageTimeout)
		r.record("проверка настройки Caddy", "")
		if err != nil {
			return err
		}
		if result.Code != 0 {
			return &StepError{Message: "Caddy не принял настройку сервера уведомлений"}
		}
		if err := editPushAddFile(ctx, r, caddy, &files.baseCaddy, false); err != nil {
			return err
		}
	}
	if _, err := r.must(ctx, compose+"up -d --no-deps --pull never --force-recreate caddy 2>&1", packageTimeout); err != nil {
		return err
	}
	if err := waitPushHealthy(ctx, r, p, certWait); err != nil {
		return err
	}
	if files.env.content == files.baseEnv {
		env, err := mergePushEnv(files.baseEnv)
		if err != nil {
			return err
		}
		var previous *string
		if p.Adding.EnvPresent {
			previous = &files.baseEnv
		}
		if err := editPushAddFile(ctx, r, installFile{files.env.path, env}, previous, false); err != nil {
			return err
		}
	}
	if _, err := r.must(ctx, compose+"up -d --no-deps --pull never wynd 2>&1", packageTimeout); err != nil {
		return err
	}
	if err := waitHealthy(ctx, r); err != nil {
		return err
	}
	state, err := ReadServerStateAt(ctx, r.sess, l)
	if err != nil {
		return err
	}
	if state.Domain == "" {
		return pushFilesChanged()
	}
	body, err := r.probe(ctx, "https://"+state.Domain+"/api/v1/instance")
	if err != nil {
		return &StepError{Message: "Не удалось проверить адрес уведомлений на сайте Wynd"}
	}
	var instance struct {
		PushServer string `json:"push_server"`
	}
	if json.Unmarshal([]byte(body), &instance) != nil || instance.PushServer != p.origin() {
		return &StepError{Message: "Wynd не сообщил адрес сервера уведомлений", Advice: "Проверьте версию Wynd."}
	}
	return nil
}

func restorePushAdd(ctx context.Context, r *Run, p pushSettings) error {
	if err := checkPushAddProject(ctx, r, true); err != nil {
		return err
	}
	files, err := readPushAddFiles(ctx, r, &p)
	if err != nil {
		return err
	}
	l := r.layout
	generated := []installFile{p.composeFile(l), p.ntfyFile(l)}
	var created []installFile
	for _, file := range generated {
		got, err := readPushFile(ctx, r.sess, file.path)
		if err != nil {
			return err
		}
		if got == file.content {
			created = append(created, file)
		} else if got != "" {
			return pushFilesChanged()
		}
	}
	base := "cd " + l.shellRoot() + " && " + composeKeepOrphans + "-f compose.yaml "
	if files.caddy.content != files.baseCaddy {
		if err := editPushAddFile(ctx, r, installFile{files.caddy.path, files.baseCaddy}, &files.caddy.content, false); err != nil {
			return err
		}
		if _, err := r.must(ctx, base+"up -d --no-deps --pull never --force-recreate caddy 2>&1", packageTimeout); err != nil {
			return err
		}
	}
	if files.env.content != files.baseEnv {
		if err := editPushAddFile(ctx, r, installFile{files.env.path, files.baseEnv}, &files.env.content, !p.Adding.EnvPresent); err != nil {
			return err
		}
		if _, err := r.must(ctx, base+"up -d --no-deps --pull never wynd 2>&1", packageTimeout); err != nil {
			return err
		}
		if err := waitHealthy(ctx, r); err != nil {
			return err
		}
	}
	if len(created) > 0 && created[0].path == generated[0].path {
		if _, err := r.must(ctx, l.pushCompose()+"rm --stop --force push 2>&1", packageTimeout); err != nil {
			return err
		}
	}
	for _, file := range append(created, p.metadata(l)) {
		if err := editPushAddFile(ctx, r, installFile{path: file.path}, &file.content, true); err != nil {
			return err
		}
	}
	return nil
}
