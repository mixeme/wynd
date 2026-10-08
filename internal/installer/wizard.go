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

	mu      sync.Mutex
	input   ConnectInput
	access  Access
	pending *UnknownHostError
	session *Session
	report  *Report
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
}
