package workshop

import (
	"context"
	"errors"
	"slices"
	"strings"
)

// PushAddPlan reports the checked domain for a separate Push service on an
// existing Wynd installation. Planning runs read-only checks.
type PushAddPlan struct {
	OK      bool   `json:"ok"`
	Domain  string `json:"domain,omitempty"`
	Message string `json:"message,omitempty"`
	Advice  string `json:"advice,omitempty"`
}

// PlanPushAdd checks a separately configured Push domain for this SSH profile.
func (w *Wizard) PlanPushAdd(ctx context.Context, domain string) PushAddPlan {
	_, _, settings, err := w.preparePushAdd(ctx, domain)
	if err != nil {
		var step *StepError
		if errors.As(err, &step) {
			return PushAddPlan{Domain: settings.Domain, Message: step.Message, Advice: step.Advice}
		}
		return PushAddPlan{Message: "Не удалось проверить установку Wynd", Advice: "Проверьте подключение к серверу и попробуйте ещё раз."}
	}
	return PushAddPlan{OK: true, Domain: settings.Domain}
}

func (w *Wizard) preparePushAdd(ctx context.Context, domain string) (*Session, Layout, pushSettings, error) {
	w.mu.Lock()
	if w.progress.Running {
		w.mu.Unlock()
		return nil, Layout{}, pushSettings{}, &StepError{Message: "Сначала дождитесь окончания операции"}
	}
	if w.session == nil || w.report == nil {
		w.mu.Unlock()
		return nil, Layout{}, pushSettings{}, &StepError{Message: "Сначала подключитесь к серверу и дождитесь осмотра"}
	}
	sess, report, layout := w.session, w.report, w.layout()
	w.mu.Unlock()

	settings, err := checkPushAdd(ctx, sess, layout, domain)
	if err != nil {
		return nil, Layout{}, pushSettings{}, err
	}
	dns := w.CheckDomain(ctx, settings.Domain)

	w.mu.Lock()
	sameInstallation := w.session == sess && w.report == report && w.layout() == layout
	w.mu.Unlock()
	if !sameInstallation {
		return nil, Layout{}, pushSettings{}, &StepError{Message: "Состояние сервера изменилось", Advice: "Составьте план снова."}
	}
	if dns.Level != LevelOK {
		return nil, Layout{}, settings, &StepError{Message: dns.Text, Advice: dns.Advice}
	}
	return sess, layout, settings, nil
}

func checkPushAdd(ctx context.Context, sess *Session, l Layout, domain string) (pushSettings, error) {
	st, err := ReadServerStateAt(ctx, sess, l)
	if err != nil {
		return pushSettings{}, err
	}
	if !st.Installed || (!st.Container && !st.pushPresent) || st.Domain == "" {
		return pushSettings{}, &StepError{Message: "Установка Wynd не найдена", Advice: "Сначала установите Wynd через Мастерскую."}
	}
	if st.Move != nil || st.Update != nil {
		return pushSettings{}, &StepError{Message: "На сервере осталась незавершённая операция", Advice: "Завершите её в пульте Wynd."}
	}
	if !st.Running && !st.pushPresent {
		return pushSettings{}, &StepError{Message: "Сначала запустите сайт Wynd"}
	}
	p, err := (Spec{Domain: st.Domain, PushDomain: domain}).pushSettings()
	if err != nil {
		return pushSettings{}, &StepError{Message: err.Error()}
	}
	if p.Domain == "" {
		return pushSettings{}, &StepError{Message: "Укажите домен сервера уведомлений"}
	}
	if err := checkPushRoot(ctx, sess, l); err != nil {
		return pushSettings{}, err
	}
	if st.pushPresent {
		previous, err := loadPushSettings(ctx, sess, l)
		if err != nil {
			return pushSettings{}, err
		}
		if previous.Adding == nil {
			return pushSettings{}, &StepError{Message: "Сервер уведомлений уже настроен", Advice: "Откройте его настройки в пульте."}
		}
		if previous.Domain != p.Domain {
			return pushSettings{}, &StepError{Message: "Добавление сервера уведомлений не завершено", Advice: "Продолжите добавление для домена " + previous.Domain + "."}
		}
		return previous, nil
	}
	// Orphan files also count as an existing configuration, even when empty
	// or a broken symlink. A future apply must preserve them.
	const checkFiles = `for f in push.json push.yaml ntfy.yml; do
  if [ -e "$f" ] || [ -L "$f" ]; then exit 3; fi
done`
	files, err := sess.Exec(ctx, "cd "+l.shellRoot()+" && { "+checkFiles+"\n}", nil, shortTimeout)
	if err != nil {
		return pushSettings{}, err
	}
	if files.Code != 0 {
		return pushSettings{}, pushFilesChanged()
	}
	env, err := readPushFile(ctx, sess, l.Root()+"/.env")
	if err != nil {
		return pushSettings{}, err
	}
	if _, hasPush, err := splitPushEnv(env); err != nil || hasPush {
		return pushSettings{}, pushFilesChanged()
	}
	caddy, err := readPushFile(ctx, sess, l.Caddyfile())
	if err != nil {
		return pushSettings{}, err
	}
	if strings.Contains(caddy, "# BEGIN Wynd Push") || strings.Contains(caddy, "# END Wynd Push") {
		return pushSettings{}, pushFilesChanged()
	}
	const ownCaddy = "Подключение Push требует работающего Caddy из установки Мастерской."
	if strings.TrimSpace(caddy) == "" {
		return pushSettings{}, &StepError{Message: "Caddy Мастерской не найден", Advice: ownCaddy}
	}
	compose := l.ComposeCommand() + "-f compose.yaml "
	services, err := sess.Exec(ctx, compose+"config --services 2>/dev/null", nil, shortTimeout)
	if err != nil {
		return pushSettings{}, err
	}
	if services.Code != 0 {
		return pushSettings{}, &StepError{Message: "Не удалось проверить конфигурацию Docker Compose"}
	}
	names := strings.Fields(services.Output)
	if !slices.Contains(names, "caddy") || !slices.Contains(names, "wynd") {
		return pushSettings{}, &StepError{Message: "Caddy Мастерской не найден", Advice: ownCaddy}
	}
	if slices.Contains(names, "push") {
		return pushSettings{}, pushFilesChanged()
	}
	running, err := sess.Exec(ctx, compose+"ps --quiet caddy 2>/dev/null", nil, shortTimeout)
	if err != nil {
		return pushSettings{}, err
	}
	if running.Code != 0 || strings.TrimSpace(running.Output) == "" {
		return pushSettings{}, &StepError{Message: "Caddy Мастерской не отвечает", Advice: ownCaddy}
	}
	return p, nil
}
