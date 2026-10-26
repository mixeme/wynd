package installer

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Пульт в окне (кадры 7а–7е): страница сервера, кнопки сервиса, обновление.

// ServerPage — страница сервера, где Wynd уже стоит (кадры 7а, 7а2, 7е).
type ServerPage struct {
	OK      bool        `json:"ok"`
	Message string      `json:"message,omitempty"`
	Advice  string      `json:"advice,omitempty"`
	State   ServerState `json:"state"`
	// Host — адрес сервера: у старого и нового при переезде домен один,
	// различает их только он (кадр 9в2, строка под доменом на пульте).
	Host string `json:"host"`
	// Latest — свежая версия в хранилище, если она новее стоящей.
	Latest string `json:"latest,omitempty"`
	// CopyBytes — сколько займёт копия перед обновлением (база).
	CopyBytes uint64 `json:"copy_bytes"`
	// LiveCopy — стоящий Wynd умеет копию без остановки (галочка 7б).
	LiveCopy bool `json:"live_copy"`
	CanBuild bool `json:"can_build"`
	// Built — стоящий Wynd собран на сервере из открытого кода: план
	// обновления сразу предлагает собрать и новый (кадр 7б4), а не молча
	// переходит на хранилище.
	Built bool `json:"built"`
}

// builtOnServer — образ собран на сервере: так его называет SourceImage.
func builtOnServer(image string) bool {
	return strings.HasPrefix(image, imageName+":")
}

// ServiceResult — чем кончилась кнопка «Запустить», «Остановить» или «Перезапустить».
type ServiceResult struct {
	OK      bool       `json:"ok"`
	Message string     `json:"message,omitempty"`
	Advice  string     `json:"advice,omitempty"`
	Log     []LogEntry `json:"log"`
	Page    ServerPage `json:"page"`
}

// UpdateOptions — выбор на плане обновления (7б, 7б3, 7б4).
type UpdateOptions struct {
	StopForCopy bool `json:"stop_for_copy"`
	StartAfter  bool `json:"start_after"`
	Build       bool `json:"build"`
}

// ServerPage читает состояние сервера и свежую версию.
func (w *Wizard) ServerPage(ctx context.Context) ServerPage {
	w.mu.Lock()
	if w.session == nil {
		w.mu.Unlock()
		return ServerPage{Message: "Сначала подключитесь к серверу"}
	}
	// Связь могла оборваться посреди обновления или бэкапа (кадр 7е):
	// подключаемся снова, как «Повторить».
	if !w.aliveLocked(ctx) {
		if res := w.dialLocked(ctx); res.Status != StatusConnected {
			w.mu.Unlock()
			return ServerPage{Message: "Связь с сервером оборвалась", Advice: "Подключитесь ещё раз."}
		}
	}
	sess, rep := w.session, w.report
	w.mu.Unlock()
	st, err := ReadServerState(ctx, sess)
	if err != nil {
		return ServerPage{Message: "Связь с сервером оборвалась", Advice: "Подключитесь ещё раз."}
	}
	page := ServerPage{OK: true, State: st, Host: sess.RemoteIP(), CanBuild: w.Source != nil, LiveCopy: CanCopyLive(st.Version), Built: builtOnServer(st.Image)}
	if reg, ok := w.Image.(RegistryImage); ok && rep != nil {
		if latest, found := reg.Latest(ctx, sess, rep.Arch); found {
			cur, okCur := releaseNumber(st.Version)
			next, okNext := releaseNumber(latest)
			if okNext && (!okCur || versionLess(cur, next)) {
				page.Latest = latest
			}
		}
	}
	if size, err := CopySize(ctx, sess); err == nil {
		page.CopyBytes = size
	}
	w.mu.Lock()
	w.state = &page
	w.mu.Unlock()
	return page
}

// Service — кнопки страницы сервера.
func (w *Wizard) Service(ctx context.Context, action string) ServiceResult {
	w.mu.Lock()
	if w.progress.Running {
		w.mu.Unlock()
		return ServiceResult{Message: "Сначала дождитесь окончания", Advice: "Сейчас идёт обновление."}
	}
	if !w.aliveLocked(ctx) {
		if res := w.dialLocked(ctx); res.Status != StatusConnected {
			w.mu.Unlock()
			return ServiceResult{Message: "Связь с сервером оборвалась", Advice: "Подключитесь заново."}
		}
	}
	sess := w.session
	w.mu.Unlock()
	log, err := ServiceAction(ctx, sess, action)
	res := ServiceResult{OK: err == nil, Log: log}
	if err != nil {
		res.Message = "Не получилось"
		var se *StepError
		if errors.As(err, &se) {
			res.Message, res.Advice = se.Message, se.Advice
		} else {
			res.Advice = "Что ответил сервер — в «подробностях»."
		}
	}
	res.Page = w.ServerPage(ctx)
	return res
}

// StartUpdate обновляет Wynd до свежей версии со страницы сервера. Дальше
// окно спрашивает InstallProgress — тот же ход, что у установки.
func (w *Wizard) StartUpdate(ctx context.Context, opt UpdateOptions) Progress {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.state == nil || !w.state.OK || w.state.Latest == "" && w.state.State.Update == nil {
		return Progress{Failure: &Failure{Message: "Обновлять нечего: стоит самая свежая версия"}}
	}
	if w.progress.Running {
		return w.progress
	}
	if !w.aliveLocked(ctx) {
		if res := w.dialLocked(ctx); res.Status != StatusConnected {
			return Progress{Failure: &Failure{Message: res.Message, Advice: res.Advice}}
		}
	}
	st := w.state.State
	to := w.state.Latest
	if st.Update != nil {
		// Продолжить начатое (7е): та же версия, сайт уже остановлен.
		to = st.Update.To
		opt.StartAfter = st.Update.StartAfter
	}
	image := w.Image
	if opt.Build && w.Source != nil {
		image = w.Source
	}
	if !opt.StopForCopy && !w.state.LiveCopy {
		opt.StopForCopy = true
	}
	running := st.Running
	if !running {
		opt.StopForCopy = false
	} else {
		opt.StartAfter = true
	}
	spec := UpdateSpec{From: st.Version, To: to, Domain: st.Domain, Image: image,
		Running: running, StopForCopy: opt.StopForCopy, StartAfter: opt.StartAfter, Probe: w.Probe}
	if st.Update != nil {
		spec.From = st.Update.From
	}
	if spec.Probe == nil {
		spec.Probe = httpProbe
	}
	sess := w.session
	runCtx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	finished := make(chan struct{})
	w.finished = finished
	w.progress = Progress{Steps: UpdateTitles(spec), Running: true}
	go func() {
		defer close(finished)
		defer cancel()
		last := Update(runCtx, sess, spec, func(p Progress) {
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

// BackupOptions — выбор на 8а и 8а2.
type BackupOptions struct {
	Folder   string `json:"folder"`
	Site     string `json:"site"`
	Password string `json:"password"`
}

// PlanBackup — строка «Скачаем …» на 8а для папки folder.
func (w *Wizard) PlanBackup(ctx context.Context, folder string) LocalBackupPlan {
	w.mu.Lock()
	sess := w.session
	w.mu.Unlock()
	if sess == nil {
		return LocalBackupPlan{Message: "Сначала подключитесь к серверу"}
	}
	return PlanLocalBackup(ctx, sess, folder)
}

// StartBackup — бэкап на компьютер; ход — InstallProgress (8б).
func (w *Wizard) StartBackup(ctx context.Context, opt BackupOptions) Progress {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.state == nil || !w.state.OK {
		return Progress{Failure: &Failure{Message: "Сначала откройте страницу сервера"}}
	}
	if w.progress.Running {
		return w.progress
	}
	if !w.aliveLocked(ctx) {
		if res := w.dialLocked(ctx); res.Status != StatusConnected {
			return Progress{Failure: &Failure{Message: res.Message, Advice: res.Advice}}
		}
	}
	st := w.state.State
	spec := LocalBackupSpec{Folder: opt.Folder, Site: opt.Site, Password: opt.Password,
		Running: st.Running, LiveCopy: w.state.LiveCopy, Server: st.Domain, Version: st.Version}
	sess := w.session
	runCtx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	finished := make(chan struct{})
	w.finished = finished
	w.progress = Progress{Running: true, Steps: []StepState{
		{ID: "snapshot", Title: "Копия базы на сервере", Status: StepPending},
		{ID: "fetch", Title: "Скачиваем", Status: StepPending},
		{ID: "check", Title: "Проверка скачанного", Status: StepPending},
	}}
	go func() {
		defer close(finished)
		defer cancel()
		last := LocalBackup(runCtx, sess, spec, func(p Progress) {
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

// ReturnPrevious — «Вернуть 0.42.2» (кадры 7д, 7е).
func (w *Wizard) ReturnPrevious(ctx context.Context) ServiceResult {
	w.mu.Lock()
	if w.progress.Running {
		w.mu.Unlock()
		return ServiceResult{Message: "Сначала дождитесь окончания"}
	}
	if !w.aliveLocked(ctx) {
		if res := w.dialLocked(ctx); res.Status != StatusConnected {
			w.mu.Unlock()
			return ServiceResult{Message: "Связь с сервером оборвалась", Advice: "Подключитесь заново."}
		}
	}
	sess := w.session
	var from string
	start := true
	if w.state != nil {
		from = w.state.State.Version
		if m := w.state.State.Update; m != nil {
			from, start = m.From, m.Stopped || w.state.State.Running
		}
	}
	w.mu.Unlock()
	if from == "" {
		return ServiceResult{Message: "Неизвестно, какую версию возвращать"}
	}
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	log, err := ReturnPrevious(runCtx, sess, from, start)
	res := ServiceResult{OK: err == nil, Log: log}
	if err != nil {
		res.Message = "Вернуть не получилось"
		res.Advice = "Что ответил сервер — в «подробностях». Копия данных на месте: /opt/wynd/backups/" + from + "."
	}
	w.mu.Lock()
	w.progress = Progress{}
	w.mu.Unlock()
	res.Page = w.ServerPage(ctx)
	return res
}
