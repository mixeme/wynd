package installer

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
)

// Vault хранит пароль от сервера там, где система хранит пароли (связка
// ключей), и только если человек об этом попросил.
type Vault interface {
	Get(host, user string) (string, bool)
	Set(host, user, password string) error
	Delete(host, user string) error
}

// ConnectInput — то, что человек ввёл на первом окне.
type ConnectInput struct {
	Host string `json:"host"`
	// Port — порт SSH; ноль — обычный 22. Окно его не спрашивает.
	Port     int    `json:"port,omitempty"`
	User     string `json:"user"`
	Password string `json:"password"`
	UseKeys  bool   `json:"use_keys"`
	// Remember — «Запомнить пароль»: после удачного входа положить его в
	// связку ключей. Без галочки прежде сохранённый пароль стирается.
	Remember bool `json:"remember"`
}

// Состояния подключения для окна.
const (
	StatusConnected   = "connected"
	StatusUnknownHost = "unknown_host"
	StatusError       = "error"
)

// ConnectResult — чем кончилась попытка подключиться, словами человека.
type ConnectResult struct {
	Status      string `json:"status"`
	Host        string `json:"host"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Message     string `json:"message,omitempty"`
	Advice      string `json:"advice,omitempty"`
}

// InspectResult — осмотр для окна.
type InspectResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Advice  string `json:"advice,omitempty"`
	Report  Report `json:"report"`
}

// Wizard ведёт одно окно установщика: подключение, отпечаток, осмотр. Пароль
// держит в памяти, пока окно открыто.
type Wizard struct {
	Dialer   Dialer
	Resolver Resolver
	Vault    Vault
	// Version — версия Wynd, которую ставим; Image — откуда сервер её возьмёт.
	Version string
	Image   ImageSource
	// Source — второй способ: сервер собирает Wynd сам из открытого кода.
	// Пусто — выбора в окне нет.
	Source ImageSource
	// Probe — как открыть сайт с этого компьютера; пусто — обычный запрос.
	Probe func(ctx context.Context, url string) (string, error)

	mu      sync.Mutex
	input   ConnectInput
	access  Access
	pending *UnknownHostError
	session *Session
	report  *Report

	plan *Plan
	// build — человек выбрал сборку на сервере (ChooseBuild); unpublished —
	// готового Wynd этой версии в реестре нет (узнаём, составляя план).
	build       bool
	unpublished bool
	progress    Progress
	cancel      context.CancelFunc
	// finished закрывается, когда прогон установки кончился.
	finished chan struct{}
}

// HasSavedPassword — лежит ли в связке пароль для этого сервера: окно тогда
// не требует вводить его заново.
func (w *Wizard) HasSavedPassword(host, user string) bool {
	if w.Vault == nil {
		return false
	}
	_, ok := w.Vault.Get(strings.TrimSpace(host), strings.TrimSpace(user))
	return ok
}

// Connect подключается к серверу. Незнакомый сервер — StatusUnknownHost с
// отпечатком: входа ещё не было, ждём ConfirmHost.
func (w *Wizard) Connect(ctx context.Context, in ConnectInput) ConnectResult {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closeLocked()
	in.Host = strings.TrimSpace(in.Host)
	in.User = strings.TrimSpace(in.User)
	access := Access{Host: in.Host, Port: in.Port, User: in.User, Password: in.Password, UseKeys: in.UseKeys}
	if !in.UseKeys && access.Password == "" && w.Vault != nil {
		if saved, ok := w.Vault.Get(in.Host, in.User); ok {
			access.Password = saved
		}
	}
	w.input, w.access = in, access
	return w.dialLocked(ctx)
}

// ConfirmHost — человек сверил отпечаток: запомнить сервер и войти.
func (w *Wizard) ConfirmHost(ctx context.Context) ConnectResult {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pending == nil {
		return ConnectResult{Status: StatusError, Message: "Сначала подключитесь к серверу"}
	}
	if err := w.Dialer.Trust(w.pending); err != nil {
		return ConnectResult{
			Status: StatusError, Host: w.access.Host,
			Message: "Не удалось запомнить сервер на этом компьютере",
			Advice:  "Отпечаток записывается в файл known_hosts в папке .ssh вашего профиля. Проверьте, что папка доступна для записи.",
		}
	}
	w.pending = nil
	return w.dialLocked(ctx)
}

func (w *Wizard) dialLocked(ctx context.Context) ConnectResult {
	sess, err := w.Dialer.Dial(ctx, w.access)
	if err != nil {
		var unknown *UnknownHostError
		if errors.As(err, &unknown) {
			w.pending = unknown
			return ConnectResult{Status: StatusUnknownHost, Host: unknown.Host, Fingerprint: unknown.Fingerprint}
		}
		return connectFailure(w.access, err)
	}
	w.session = sess
	if w.Vault != nil && !w.input.UseKeys {
		// Связка недоступна — не повод отказывать во входе: пароль просто
		// спросят в следующий раз.
		if w.input.Remember {
			_ = w.Vault.Set(w.access.Host, w.access.User, w.access.Password)
		} else {
			_ = w.Vault.Delete(w.access.Host, w.access.User)
		}
	}
	return ConnectResult{Status: StatusConnected, Host: w.access.Host}
}

func connectFailure(a Access, err error) ConnectResult {
	res := ConnectResult{Status: StatusError, Host: a.Host}
	var changed *ChangedHostError
	switch {
	case errors.As(err, &changed):
		res.Fingerprint = changed.Fingerprint
		res.Message = "Сервер назвал не тот отпечаток, что в прошлый раз"
		res.Advice = "Так бывает, если на сервере переустановили систему. Если вы этого не делали — не подключайтесь: возможно, по этому адресу отвечает чужой сервер. Если переустанавливали — удалите строку с этим адресом из файла known_hosts в папке .ssh вашего профиля и подключитесь снова."
	case errors.Is(err, ErrInvalid):
		res.Message = "Заполните адрес сервера, пользователя и пароль"
		res.Advice = "Адрес — только имя или цифры с точками, как в письме хостинга: без «https://» и без пробелов."
	case errors.Is(err, ErrNoKeys):
		res.Message = "На этом компьютере нет ключа, которым можно войти"
		res.Advice = "Ключи лежат в папке .ssh вашего профиля. Ключ, защищённый своим паролем, пока не подходит — войдите паролем от сервера."
	case errors.Is(err, ErrAuth) && a.UseKeys:
		res.Message = "Сервер не принял ключ с этого компьютера"
		res.Advice = "На сервере этот ключ не записан. Войдите паролем от сервера — он в письме хостинга."
	case errors.Is(err, ErrAuth):
		res.Message = "Сервер не принял пароль"
		res.Advice = "Проверьте пользователя и пароль по письму хостинга. В пароле важны большие и маленькие буквы; лучше скопировать его целиком."
	default:
		res.Message = "Сервер не отвечает по этому адресу"
		res.Advice = "Проверьте адрес по письму хостинга и что сервер включён в панели хостинга. Только что заказанный сервер запускается несколько минут."
	}
	return res
}

// Inspect осматривает подключённый сервер. Ничего на нём не меняет.
func (w *Wizard) Inspect(ctx context.Context) InspectResult {
	w.mu.Lock()
	sess := w.session
	w.mu.Unlock()
	if sess == nil {
		return InspectResult{Message: "Сначала подключитесь к серверу"}
	}
	rep, err := Inspect(ctx, sess)
	if err != nil {
		return InspectResult{
			Message: "Связь с сервером оборвалась",
			Advice:  "Подключитесь ещё раз. На сервере ничего не менялось.",
		}
	}
	w.mu.Lock()
	w.report = &rep
	w.mu.Unlock()
	return InspectResult{OK: true, Report: rep}
}

// CheckDomain проверяет адрес будущего сайта против осмотренного сервера.
func (w *Wizard) CheckDomain(ctx context.Context, domain string) DomainCheck {
	w.mu.Lock()
	sess, rep := w.session, w.report
	w.mu.Unlock()
	var addrs []string
	if sess != nil {
		addrs = append(addrs, sess.RemoteIP())
	}
	if rep != nil {
		addrs = append(addrs, rep.Addresses...)
	}
	r := w.Resolver
	if r == nil {
		r = net.DefaultResolver
	}
	return CheckDomain(ctx, r, domain, addrs)
}

// Close рвёт подключение и забывает пароль из памяти.
func (w *Wizard) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closeLocked()
	w.input, w.access = ConnectInput{}, Access{}
}

func (w *Wizard) closeLocked() {
	if w.session != nil {
		_ = w.session.Close()
	}
	w.session, w.pending, w.report = nil, nil, nil
	if w.cancel != nil {
		w.cancel()
		w.cancel = nil
	}
	w.plan, w.build, w.unpublished = nil, false, false
	w.progress = Progress{}
}

// MakePlan составляет план установки на осмотренный сервер (окно 3). Адрес
// сайта проверяется заново: между осмотром и планом запись могла измениться.
func (w *Wizard) MakePlan(ctx context.Context, domain string) Plan {
	check := w.CheckDomain(ctx, domain)
	w.mu.Lock()
	sess, rep := w.session, w.report
	w.mu.Unlock()
	// Есть ли готовый Wynd, спрашиваем до плана: человек должен увидеть сборку
	// в плане, а не отказом посреди установки.
	unpublished := false
	if reg, ok := w.Image.(RegistryImage); ok && w.Source != nil && sess != nil && rep != nil {
		published, known := reg.Published(ctx, sess, w.Version, rep.Arch)
		unpublished = known && !published
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.session == nil || w.report == nil {
		return Plan{Message: "Сначала подключитесь к серверу и дождитесь осмотра"}
	}
	w.unpublished = unpublished
	if check.Level != LevelOK {
		return Plan{Domain: check.Domain, Message: check.Text, Advice: check.Advice}
	}
	return w.planLocked(*w.report, check.Domain)
}

// planLocked составляет план по осмотру и запоминает его, если по нему можно
// ставить.
func (w *Wizard) planLocked(rep Report, domain string) Plan {
	plan := BuildPlan(rep, domain, w.Version, (w.build || w.unpublished) && w.Source != nil)
	plan.CanBuild = w.Source != nil
	plan.Unpublished = w.unpublished && w.Source != nil
	w.plan = nil
	if plan.OK {
		w.plan = &plan
		w.progress = Progress{Steps: StepTitles(w.specLocked(plan))}
	}
	return plan
}

// ChooseBuild переключает способ: собрать Wynd на сервере из открытого кода
// (on) или скачать готовый образ. План пересоставляется; начатая установка
// продолжится новым способом с шага, на котором встала.
func (w *Wizard) ChooseBuild(on bool) Plan {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.plan == nil || w.report == nil {
		return Plan{Message: "Сначала составьте план установки"}
	}
	if w.progress.Running {
		return Plan{Message: "Установка ещё идёт", Advice: "Дождитесь, пока текущий шаг закончится."}
	}
	w.build = on && w.Source != nil
	return w.planLocked(*w.report, w.plan.Domain)
}

func (w *Wizard) specLocked(plan Plan) Spec {
	image := w.Image
	if plan.Build && w.Source != nil {
		image = w.Source
	}
	return Spec{Domain: plan.Domain, Version: plan.Version, Image: image, FixClock: plan.FixClock, Probe: w.Probe}
}

// StartInstall запускает установку по плану и сразу возвращает её состояние;
// дальше окно спрашивает InstallProgress. Повторный вызов после отказа —
// «Повторить»: сделанное пропускается. Оборванное подключение поднимается заново.
func (w *Wizard) StartInstall(ctx context.Context) Progress {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.plan == nil {
		return Progress{Failure: &Failure{Message: "Сначала составьте план установки"}}
	}
	if w.progress.Running {
		return w.progress
	}
	if !w.aliveLocked(ctx) {
		if res := w.dialLocked(ctx); res.Status != StatusConnected {
			w.progress.Failure = &Failure{Message: res.Message, Advice: res.Advice}
			if res.Status == StatusUnknownHost {
				w.progress.Failure = &Failure{
					Message: "Сервер назвал незнакомый отпечаток",
					Advice:  "Подключитесь к серверу заново с первого окна.",
				}
			}
			return w.progress
		}
	}
	spec, sess := w.specLocked(*w.plan), w.session
	runCtx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	finished := make(chan struct{})
	w.finished = finished
	w.progress = Progress{Steps: StepTitles(spec), Running: true}
	go func() {
		defer close(finished)
		defer cancel()
		last := Install(runCtx, sess, spec, func(p Progress) {
			w.mu.Lock()
			if w.finished == finished {
				w.progress = p
			}
			w.mu.Unlock()
		})
		w.mu.Lock()
		if w.finished == finished {
			w.progress = last
		}
		w.mu.Unlock()
	}()
	return w.progress
}

// aliveLocked — жива ли связь с сервером.
func (w *Wizard) aliveLocked(ctx context.Context) bool {
	if w.session == nil {
		return false
	}
	_, err := w.session.Exec(ctx, "true", nil, dialTimeout)
	if err != nil {
		_ = w.session.Close()
		w.session = nil
	}
	return err == nil
}

// InstallProgress — как идёт установка.
func (w *Wizard) InstallProgress() Progress {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.progress
}

// WaitInstall ждёт конца прогона установки (для тестов и пробы без окна).
func (w *Wizard) WaitInstall() Progress {
	w.mu.Lock()
	finished := w.finished
	w.mu.Unlock()
	if finished != nil {
		<-finished
	}
	return w.InstallProgress()
}

// RollbackResult — чем кончился откат.
type RollbackResult struct {
	OK      bool       `json:"ok"`
	Message string     `json:"message,omitempty"`
	Advice  string     `json:"advice,omitempty"`
	Log     []LogEntry `json:"log"`
	// Report и Plan — сервер после отката: осмотрен заново, план пересоставлен.
	Report *Report `json:"report,omitempty"`
	Plan   *Plan   `json:"plan,omitempty"`
}

// Rollback убирает поставленное этой установкой. keepData — оставить на
// сервере данные Wynd (записи, фотографии): нужны, если ставить заново.
func (w *Wizard) Rollback(ctx context.Context, keepData bool) RollbackResult {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.plan == nil {
		return RollbackResult{Message: "Откатывать нечего: установка не начиналась"}
	}
	if w.progress.Running {
		return RollbackResult{Message: "Установка ещё идёт", Advice: "Дождитесь, пока текущий шаг закончится."}
	}
	if !w.aliveLocked(ctx) {
		if res := w.dialLocked(ctx); res.Status != StatusConnected {
			return RollbackResult{Message: "Связь с сервером оборвалась", Advice: "Подключитесь заново и повторите откат."}
		}
	}
	old := *w.plan
	spec := w.specLocked(old)
	log, err := Rollback(ctx, w.session, spec, keepData)
	if err != nil {
		return RollbackResult{
			Log:     log,
			Message: "Откатить не получилось",
			Advice:  "Нажмите «Да, откатить» ещё раз. Что осталось на сервере — видно в «подробностях».",
		}
	}
	w.progress = Progress{Steps: StepTitles(spec)}
	res := RollbackResult{OK: true, Log: log}
	// Прежний план писался для сервера, где Wynd уже стоял. Осмотр не вышел —
	// остаётся прежний: установка по нему всё равно доделает недостающее.
	if rep, err := Inspect(ctx, w.session); err == nil {
		if plan := w.planLocked(rep, spec.Domain); plan.OK {
			w.report = &rep
			res.Report, res.Plan = &rep, &plan
		} else {
			w.plan = &old
		}
	}
	return res
}
