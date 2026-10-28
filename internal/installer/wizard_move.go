package installer

import (
	"context"
	"net"
	"strings"
	"time"
)

// Переезд в окне (кадры 9а–9г): второе подключение — к новому серверу.

// MovePlan — 9б: что сделаем; и почему нельзя, если нельзя.
type MovePlan struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Advice  string `json:"advice,omitempty"`
	Domain  string `json:"domain"`
	Version string `json:"version"`
	NewIP   string `json:"new_ip"`
	OldIP   string `json:"old_ip"`
	Size    int64  `json:"size"`
	// DataOnly — на старом Wynd не со своим Caddy: переезжают только данные,
	// на новом — своя установка с новым сертификатом (9б2). OldProxy — чужой
	// веб-сервер старого («nginx», «apache», «caddy»), его не трогаем.
	DataOnly bool   `json:"data_only"`
	OldProxy string `json:"old_proxy,omitempty"`
	// Resume — переезд на этот сервер уже начат (9в2): «Продолжить» идёт
	// с того шага, где он остановился; RemoveOld — выбор, сделанный тогда.
	Resume    bool `json:"resume,omitempty"`
	RemoveOld bool `json:"remove_old,omitempty"`
	// Foreign — старый Wynd поставлен не Мастерской (7з): убрать его после
	// переезда нельзя, выбора на 9б нет.
	Foreign bool `json:"foreign,omitempty"`

	resume *MoveMark
}

// MoveConnect подключается к новому серверу (9а); незнакомый — отпечаток.
func (w *Wizard) MoveConnect(ctx context.Context, in ConnectInput) ConnectResult {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.target != nil {
		_ = w.target.Close()
		w.target = nil
	}
	in.Host, in.User = strings.TrimSpace(in.Host), strings.TrimSpace(in.User)
	w.tAccess = Access{Host: in.Host, Port: in.Port, User: in.User, Password: in.Password, UseKeys: in.UseKeys}
	w.tInput = in
	return w.moveDialLocked(ctx)
}

// MoveConfirmHost — человек сверил отпечаток нового сервера.
func (w *Wizard) MoveConfirmHost(ctx context.Context) ConnectResult {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.tPending == nil {
		return ConnectResult{Status: StatusError, Message: "Сначала подключитесь к новому серверу"}
	}
	if err := w.Dialer.Trust(w.tPending); err != nil {
		return ConnectResult{Status: StatusError, Message: "Не удалось запомнить сервер на этом компьютере"}
	}
	w.tPending = nil
	return w.moveDialLocked(ctx)
}

func (w *Wizard) moveDialLocked(ctx context.Context) ConnectResult {
	sess, err := w.Dialer.Dial(ctx, w.tAccess)
	if err != nil {
		if unknown, ok := err.(*UnknownHostError); ok {
			w.tPending = unknown
			return ConnectResult{Status: StatusUnknownHost, Host: unknown.Host, Fingerprint: unknown.Fingerprint}
		}
		return connectFailure(w.tAccess, err)
	}
	w.target = sess
	res := ConnectResult{Status: StatusConnected, Host: w.tAccess.Host}
	if w.tInput.MakeKey && !w.tInput.UseKeys {
		if line, err := w.Dialer.EnsureWorkshopKey(); err == nil {
			if sess.InstallKey(ctx, line) != nil {
				res.KeyNote = "Положить ключ на новый сервер не получилось — в следующий раз войдите паролем."
			}
		}
	}
	return res
}

// MovePlan осматривает новый сервер и составляет план переезда (9б).
func (w *Wizard) MovePlan(ctx context.Context) MovePlan {
	w.mu.Lock()
	old, target, page, oldRep := w.session, w.target, w.state, w.report
	w.mu.Unlock()
	if old == nil || target == nil || page == nil {
		return MovePlan{Message: "Сначала подключитесь к обоим серверам"}
	}
	if old.User != "root" || target.User != "root" {
		return MovePlan{Message: "Переезд пока умеет только вход root", Advice: "Подключитесь к обоим серверам пользователем root."}
	}
	st := page.State
	plan := MovePlan{Domain: st.Domain, Version: st.Version, OldIP: old.RemoteIP(), NewIP: target.RemoteIP()}
	if st.Move != nil && st.Move.To != plan.NewIP {
		plan.Message = "Переезд начат на сервер " + st.Move.To
		plan.Advice = "Подключитесь к нему, чтобы продолжить или отменить переезд."
		return plan
	}
	rep, err := Inspect(ctx, target)
	if err != nil {
		plan.Message = "Связь с новым сервером оборвалась"
		return plan
	}
	if page.Foreign != nil && page.Foreign.stop == "" {
		plan.Message = "Этот Wynd Мастерская остановить не может"
		plan.Advice = "Он запущен не службой и не контейнером. Остановите его на старом сервере сами и подключитесь снова."
		return plan
	}
	if st.Move != nil {
		// Новый сервер уже приняли на 9б, и Wynd на нём — наш, перенесённый.
		plan.Resume, plan.RemoveOld, plan.resume = true, st.Move.RemoveOld, st.Move
		rep.Findings, rep.Proxy = nil, ""
	}
	for _, f := range rep.Findings {
		switch {
		case f.ID == "wynd":
			plan.Message, plan.Advice = "На новом сервере уже стоит Wynd", "Переезжать можно на сервер без Wynd. Уберите его там или выберите другой сервер."
			return plan
		case f.Level == LevelBlock:
			plan.Message, plan.Advice = f.Text, f.Advice
			return plan
		}
	}
	if rep.Proxy != "" {
		plan.Message, plan.Advice = "На новом сервере уже есть веб-сервер", "Переезд пока умеет только сервер без своего веб-сервера: порты 80 и 443 должны быть свободны."
		return plan
	}
	// Свой Caddy старого — его Caddyfile в папке установки и нет чужого
	// веб-сервера на портах; иначе сертификат получала не Мастерская.
	if oldRep != nil {
		plan.OldProxy = oldRep.Proxy
	}
	own, err := old.Exec(ctx, "test -f "+installDir+"/Caddyfile && echo own", nil, shortTimeout)
	if err != nil {
		plan.Message = "Связь со старым сервером оборвалась"
		return plan
	}
	plan.DataOnly = plan.OldProxy != "" || strings.TrimSpace(own.Output) != "own" || page.Foreign != nil
	what := installDir
	if plan.DataOnly {
		what = dataDir
	}
	if page.Foreign != nil {
		// Кадр 7з: везём одни данные; убрать старый Мастерская не берётся.
		what, plan.Foreign = shQuote(page.Foreign.DataDir), true
	}
	if res, err := old.Exec(ctx, "du -sb --exclude=backups --exclude=src --exclude='.workshop-*' "+what+" | cut -f1", nil, shortTimeout); err == nil {
		plan.Size = parseInt64(strings.TrimSpace(res.Output))
	}
	plan.OK = true
	w.mu.Lock()
	w.movePlan = &plan
	w.mu.Unlock()
	return plan
}

func parseInt64(s string) int64 {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int64(c-'0')
	}
	return n
}

// StartMove — «Переехать» (9б); ход — InstallProgress (9в).
func (w *Wizard) StartMove(ctx context.Context, removeOld bool) Progress {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.movePlan == nil || !w.movePlan.OK {
		return Progress{Failure: &Failure{Message: "Сначала составьте план переезда"}}
	}
	if w.progress.Running {
		return w.progress
	}
	plan := *w.movePlan
	var foreign *ForeignWynd
	if w.state != nil {
		foreign = w.state.Foreign
	}
	if foreign != nil {
		removeOld = false
	}
	check := make(chan struct{}, 1)
	w.moveCheck = check
	spec := MoveSpec{Domain: plan.Domain, Version: plan.Version, Image: w.Image, OldIP: plan.OldIP, NewIP: plan.NewIP,
		RemoveOld: removeOld, DataOnly: plan.DataOnly, CheckNow: check, Probe: w.Probe, Resume: plan.resume, Foreign: foreign}
	target := w.target
	spec.Resolve = func(ctx context.Context, domain string) []string {
		// Спрашиваем новый сервер: он видит адрес, как люди в интернете, и VPN
		// этого компьютера ни при чём (кадр 9в3). Не ответил — спрашиваем здесь.
		if w.Resolver == nil && target != nil {
			if addrs := resolveOn(ctx, target, domain); len(addrs) > 0 {
				return addrs
			}
		}
		r := w.Resolver
		if r == nil {
			r = net.DefaultResolver
		}
		addrs, _ := r.LookupHost(ctx, domain)
		return ipv4(addrs)
	}
	if spec.Probe == nil {
		spec.Probe = httpProbe
	}
	old := w.session
	runCtx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	finished := make(chan struct{})
	w.finished = finished
	w.progress = Progress{Steps: MoveTitles(spec), Running: true}
	go func() {
		defer close(finished)
		defer cancel()
		last := Move(runCtx, old, target, spec, func(p Progress) {
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

// resolveOn — адреса A имени глазами сервера sess; пусто — не узнали.
func resolveOn(ctx context.Context, sess *Session, domain string) []string {
	if NormalizeDomain(domain) != domain {
		return nil
	}
	res, err := sess.Exec(ctx, "getent ahostsv4 "+shQuote(domain)+" | cut -d' ' -f1 | sort -u", nil, shortTimeout)
	if err != nil || res.Code != 0 {
		return nil
	}
	return ipv4(strings.Fields(res.Output))
}

func ipv4(addrs []string) []string {
	var v4 []string
	for _, a := range addrs {
		if ip := net.ParseIP(a); ip != nil && ip.To4() != nil {
			v4 = append(v4, ip.String())
		}
	}
	return v4
}

// MoveCheckNow — «Проверить сейчас» на 9в.
func (w *Wizard) MoveCheckNow() {
	w.mu.Lock()
	ch := w.moveCheck
	w.mu.Unlock()
	if ch != nil {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// MoveAdopt — после переезда «К серверу» (9г): дальше Мастерская работает с
// новым сервером, старое подключение закрывается.
func (w *Wizard) MoveAdopt(ctx context.Context) ServerPage {
	w.mu.Lock()
	if w.target != nil {
		if w.session != nil {
			_ = w.session.Close()
		}
		w.session, w.target, w.access, w.input = w.target, nil, w.tAccess, w.tInput
		w.movePlan, w.moveCheck = nil, nil
		w.progress = Progress{}
	}
	sess := w.session
	w.mu.Unlock()
	if sess != nil {
		if rep, err := Inspect(ctx, sess); err == nil {
			w.mu.Lock()
			w.report = &rep
			w.mu.Unlock()
		}
	}
	return w.ServerPage(ctx)
}

// CancelMove — «Отменить переезд» (9в).
func (w *Wizard) CancelMove(ctx context.Context) ServiceResult {
	w.mu.Lock()
	if w.cancel != nil {
		w.cancel()
	}
	finished := w.finished
	old, target := w.session, w.target
	w.mu.Unlock()
	if finished != nil {
		select {
		case <-finished:
		case <-time.After(30 * time.Second):
		}
	}
	// Wynd на новом уже запущен — туда могли писать: отменить значило бы
	// потерять написанное.
	var foreign *ForeignWynd
	if old != nil {
		page := w.ServerPage(ctx)
		foreign = page.Foreign
		if page.State.Move != nil && page.State.Move.Launched {
			res := ServiceResult{Message: "Отменить переезд уже нельзя", Advice: "Wynd на новом сервере запущен, там могли писать. Продолжите переезд."}
			res.Page = page
			return res
		}
	}
	log, err := CancelMove(ctx, old, target, foreign)
	res := ServiceResult{OK: err == nil, Log: log}
	if err != nil {
		res.Message = "Отменить не получилось"
		res.Advice = "Что ответили серверы — в «подробностях»."
	}
	w.mu.Lock()
	w.progress = Progress{}
	w.mu.Unlock()
	res.Page = w.ServerPage(ctx)
	return res
}
