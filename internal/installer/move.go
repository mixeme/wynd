package installer

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

// Переезд на другой сервер (план 45, кадры 9а–9г). Адрес сайта прежний —
// у людей в телефонах сохранён он. Папка установки переносится целиком, с
// сертификатом: новому серверу не нужен новый сертификат. Данные идут
// напрямую со старого сервера на новый: новый сервер на время переноса
// получает ключ, которым старый отдаёт ему только архив папки Wynd, и после
// переноса ключ убирается с обоих. Единственный шаг человека — поменять у
// регистратора запись A адреса сайта.
//
// Wynd на старом стоит не со своим Caddy (за чужим веб-сервером, поставлен
// не Мастерской) — переезжают только данные и адрес, а на новом сервере
// обычная установка Мастерской со своим Caddy и новым сертификатом: чужой
// сертификат получала не Мастерская, и не ей решать его судьбу (владелец,
// 2026-10-25; кадр 9б2).

const (
	moveMark     = installDir + "/.workshop-move"
	moveKey      = "/root/.wynd-move"
	moveKnown    = "/root/.wynd-move-known"
	moveKeyLabel = "wynd-move"
)

// moveTar — что отдаёт старый сервер: папка Wynd без копий и временного.
const moveTar = `tar -C /opt --exclude=wynd/backups --exclude=wynd/src --exclude=wynd/data/backups --exclude='wynd/data/.workshop-*' --exclude='wynd/.workshop-*' -cf - wynd`

// dataTar — то же для переезда одних данных (9б2): папка данных без копий.
const dataTar = `tar -C /opt/wynd --exclude=data/backups --exclude='data/.workshop-*' -cf - data`

// MoveSpec — переезд.
type MoveSpec struct {
	Domain  string
	Version string
	Image   ImageSource
	// OldIP — адрес старого сервера, по которому новый его достанет; NewIP —
	// адрес нового: на него должна указать запись A.
	OldIP, NewIP string
	// RemoveOld — убрать Wynd со старого после переезда (9б); нет — оставить остановленным.
	RemoveOld bool
	// DataOnly — на старом Wynd не со своим Caddy: везём одни данные, на
	// новом — обычная установка со своим Caddy (9б2).
	DataOnly bool
	// Resolve — куда сейчас указывает адрес сайта (адреса A). Пусто — узнать
	// не удалось: у адреса при переезде запись A есть всегда (старый сервер).
	Resolve func(ctx context.Context, domain string) []string
	// CheckNow — «Проверить сейчас» на 9в.
	CheckNow <-chan struct{}
	Probe    func(ctx context.Context, url string) (string, error)
	// Foreign — старый Wynd стоит не на месте Мастерской (кадр 7з): как его
	// остановить и запустить, где его данные и пометка переезда.
	Foreign *ForeignWynd
	// Resume — переезд продолжается (кадр 9в2): пометка со старого сервера.
	// Перенесённое второй раз не переносится — на новом могли уже писать.
	Resume *MoveMark
}

// MoveMark — пометка начатого переезда на старом сервере. Кладёт её шаг
// «Сайт на старом остановлен», убирают «Готово» и «Отменить переезд»; пока
// она лежит, пульт старого сервера показывает незаконченный переезд (9в2).
type MoveMark struct {
	To      string `json:"to"`
	Started string `json:"started"`
	// RemoveOld — выбор на 9б: убрать Wynd со старого после переезда.
	RemoveOld bool `json:"remove_old,omitempty"`
	// Transferred — данные на новом; Launched — Wynd на новом запущен: после
	// этого отменять переезд нельзя — на новом могли уже писать.
	Transferred bool `json:"transferred,omitempty"`
	Launched    bool `json:"launched,omitempty"`
}

// markCmd записывает пометку переезда на старом сервере.
func markCmd(m MoveMark) string { return markCmdAt(moveMark, m) }

// transferKeyLine экранирует кавычки для authorized_keys. OpenSSH снимает
// только \"; прочие обратные косые черты сохраняются (misc.c, opt_dequote).
func transferKeyLine(command, pub string) string {
	command = strings.ReplaceAll(command, `"`, `\"`)
	return `restrict,command="` + command + `" ` + pub
}

func markCmdAt(path string, m MoveMark) string {
	b, _ := json.Marshal(m)
	return "printf '%s\\n' " + shQuote(string(b)) + " > " + shQuote(path)
}

// oldControl — как на старом сервере остановить и запустить Wynd и где
// пометка переезда: у установки Мастерской — docker compose в /opt/wynd.
func oldControl(f *ForeignWynd) (stop, start, mark string) {
	if f != nil {
		return f.stop, f.start, f.markPath()
	}
	return composeCmd + "stop wynd 2>&1", composeCmd + "up -d 2>&1", moveMark
}

// dnsWait — как часто сами проверяем адрес на 9в.
var dnsWait = time.Minute

// onOld выполняет команду на старом сервере и кладёт её в общий журнал.
func onOld(ctx context.Context, r *Run, old *Session, command string, timeout time.Duration) (string, error) {
	res, err := old.Exec(ctx, command, nil, timeout)
	r.record("старый сервер: "+command, res.Output)
	if err != nil {
		return res.Output, err
	}
	if res.Code != 0 {
		return res.Output, &commandError{command: command, output: res.Output, code: res.Code}
	}
	return res.Output, nil
}

func moveSteps(old *Session, spec MoveSpec) []step {
	image := imageStep(Spec{Version: spec.Version, Image: spec.Image})
	docker := dockerStep()
	mark := &MoveMark{To: spec.NewIP, Started: time.Now().UTC().Format(time.RFC3339), RemoveOld: spec.RemoveOld}
	if spec.Resume != nil {
		resumed := *spec.Resume
		resumed.RemoveOld = spec.RemoveOld
		mark = &resumed
	}
	install := "Wynd " + spec.Version + " на новом сервере"
	sendTar, pullDir, moved := moveTar, "/opt", "Папка Wynd перенесена"
	if spec.DataOnly {
		install = "Wynd " + spec.Version + " и Caddy на новом сервере"
		sendTar, pullDir, moved = dataTar, installDir, "Данные Wynd перенесены"
	}
	if spec.Foreign != nil {
		sendTar = spec.Foreign.sendTar()
	}
	stopOld, _, markAt := oldControl(spec.Foreign)
	files := filesStep(Spec{Domain: spec.Domain, Version: spec.Version, Image: spec.Image})
	return []step{
		{
			id: "install", title: install,
			do: func(ctx context.Context, r *Run) error {
				if err := runStep(ctx, r, docker); err != nil {
					return err
				}
				if err := runStep(ctx, r, image); err != nil {
					return err
				}
				if !spec.DataOnly {
					return nil
				}
				// Своя установка, как обычная: compose.yaml и Caddyfile
				// Мастерской, папки данных и Caddy; данные приедут следом.
				return runStep(ctx, r, files)
			},
		},
		{
			id: "stop", title: "Сайт на старом остановлен",
			do: func(ctx context.Context, r *Run) error {
				_, err := onOld(ctx, r, old, markCmdAt(markAt, *mark)+" && "+stopOld, shortTimeout)
				return err
			},
		},
		{
			id: "transfer", title: moved,
			do: func(ctx context.Context, r *Run) error {
				if mark.Transferred {
					r.note("перенесено раньше")
					return nil
				}
				pub, err := r.must(ctx, "test -f "+moveKey+" || ssh-keygen -q -t ed25519 -N '' -C "+moveKeyLabel+" -f "+moveKey+"; cat "+moveKey+".pub", shortTimeout)
				if err != nil {
					return err
				}
				pub = strings.TrimSpace(pub)
				hostKey, err := onOld(ctx, r, old, "cat /etc/ssh/ssh_host_ed25519_key.pub 2>/dev/null || cat /etc/ssh/ssh_host_ecdsa_key.pub", shortTimeout)
				if err != nil {
					return err
				}
				fields := strings.Fields(hostKey)
				if len(fields) < 2 {
					return &StepError{Message: "Старый сервер не назвал свой ключ", Advice: "Нажмите «Повторить»."}
				}
				// Старый отдаёт по этому ключу только архив папки Wynd.
				line := transferKeyLine(sendTar, pub)
				if _, err := onOld(ctx, r, old, "umask 077; mkdir -p /root/.ssh && grep -qF "+shQuote(pub)+" /root/.ssh/authorized_keys 2>/dev/null || printf '%s\\n' "+shQuote(line)+" >> /root/.ssh/authorized_keys", shortTimeout); err != nil {
					return err
				}
				known := spec.OldIP + " " + fields[0] + " " + fields[1]
				pull := "set -e; printf '%s\\n' " + shQuote(known) + " > " + moveKnown +
					"; mkdir -p " + pullDir + " && ssh -i " + moveKey + " -o UserKnownHostsFile=" + moveKnown +
					" -o StrictHostKeyChecking=yes -o BatchMode=yes -o ConnectTimeout=20 root@" + spec.OldIP + " | tar -C " + pullDir + " --numeric-owner -xpf -"
				// Владельцы — числами: у uid 100 и 101 Wynd в контейнере на разных
				// системах разные имена, по именам tar раздал бы файлы чужим.
				_, pullErr := r.must(ctx, pull, imageTimeout)
				// Ключ — с обоих серверов, даже если перенос не вышел.
				_, _ = onOld(ctx, r, old, "sed -i '/ "+moveKeyLabel+"$/d' /root/.ssh/authorized_keys", shortTimeout)
				_, _ = r.sh(ctx, "rm -f "+moveKey+" "+moveKey+".pub "+moveKnown, shortTimeout)
				if pullErr != nil {
					return pullErr
				}
				mark.Transferred = true
				_, err = onOld(ctx, r, old, markCmdAt(markAt, *mark), shortTimeout)
				return err
			},
			explain: func(_ error, output string) *StepError {
				if containsAny(output, "Connection timed out", "Connection refused", "No route to host", "Could not resolve") {
					return &StepError{
						Message: "Новый сервер не достучался до старого",
						Advice:  "Серверы друг друга не видят: старый принимает подключения не от всех. Разрешите на старом вход по SSH с адреса " + spec.NewIP + " и нажмите «Повторить».",
					}
				}
				return &StepError{Message: "Папку перенести не получилось", Advice: "Нажмите «Повторить». Что ответили серверы — в «подробностях»."}
			},
		},
		dnsStep(spec),
		{
			id: "start", title: "Wynd на новом сервере запущен",
			do: func(ctx context.Context, r *Run) error {
				if _, err := r.must(ctx, composeCmd+"up -d --quiet-pull 2>&1", packageTimeout); err != nil {
					return err
				}
				if err := waitHealthy(ctx, r); err != nil {
					return err
				}
				mark.Launched = true
				if _, err := onOld(ctx, r, old, markCmdAt(markAt, *mark), shortTimeout); err != nil {
					return err
				}
				if spec.Probe == nil {
					return nil
				}
				r.note("проверяем сайт")
				deadline := time.Now().Add(certWait)
				for {
					body, err := spec.Probe(ctx, "https://"+spec.Domain+"/api/v1/instance")
					if err == nil && strings.Contains(body, `"version":"`+spec.Version+`"`) {
						return nil
					}
					if time.Now().After(deadline) {
						return &StepError{
							Message: "Сайт " + spec.Domain + " не открывается с нового сервера",
							Advice:  "Wynd на новом сервере запущен. Возможно, адрес ещё не везде сменился — подождите и нажмите «Повторить».",
						}
					}
					if err := sleep(ctx, pollInterval); err != nil {
						return err
					}
				}
			},
		},
		{
			id: "old", title: "Старый сервер",
			do: func(ctx context.Context, r *Run) error {
				if spec.Foreign != nil {
					// Чужую установку не убираем и не помечаем (кадр 7з): только снимаем пометку.
					_, err := onOld(ctx, r, old, "rm -f "+shQuote(markAt), shortTimeout)
					return err
				}
				if spec.RemoveOld {
					// Сертификат не отзываем: он переехал и работает на новом —
					// или он чужой (9б2), и его судьба не наша. Файл сайта
					// чужого веб-сервера тоже не наш: остаётся.
					_, err := onOld(ctx, r, old, composeCmd+"down 2>&1; rm -rf "+installDir, packageTimeout)
					return err
				}
				_, err := onOld(ctx, r, old, "rm -f "+moveMark+" && date -u +%Y-%m-%dT%H:%M:%SZ > "+stoppedMark, shortTimeout)
				return err
			},
		},
	}
}

// dnsStep ждёт, пока адрес сайта укажет на новый сервер (9в).
func dnsStep(spec MoveSpec) step {
	return step{
		id: "dns", title: "Ждём новый адрес",
		do: func(ctx context.Context, r *Run) error {
			for {
				addrs := spec.Resolve(ctx, spec.Domain)
				if len(addrs) > 0 && allEqual(addrs, spec.NewIP) {
					return nil
				}
				switch {
				case len(addrs) == 0:
					// Кадр 9в3: служба имён не ответила — не «никуда».
					r.note("пока не удалось узнать, куда указывает " + spec.Domain)
				case allEqual(addrs, spec.OldIP):
					r.note(spec.Domain + " пока указывает на старый сервер " + spec.OldIP)
				default:
					r.note(spec.Domain + " пока указывает на " + strings.Join(addrs, ", "))
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-spec.CheckNow:
				case <-time.After(dnsWait):
				}
			}
		},
	}
}

func allEqual(addrs []string, want string) bool {
	for _, a := range addrs {
		if a != want {
			return false
		}
	}
	return true
}

// Move переносит Wynd со старого сервера (old) на новый (sess).
func Move(ctx context.Context, old, sess *Session, spec MoveSpec, report func(Progress)) Progress {
	return runSteps(ctx, sess, Spec{Domain: spec.Domain, Version: spec.Version, Image: spec.Image}, moveSteps(old, spec), report,
		func(_ *Run, p *Progress) { p.Site = "https://" + spec.Domain })
}

// MoveTitles — шаги переезда для окна до начала.
func MoveTitles(spec MoveSpec) []StepState { return titles(moveSteps(nil, spec)) }

// CancelMove — «Отменить переезд» (9в): адрес ещё не менялся — сайт снова
// запускается на старом, с нового Wynd убирается.
func CancelMove(ctx context.Context, old, sess *Session, foreign *ForeignWynd) ([]LogEntry, error) {
	r := &Run{sess: sess, steps: []StepState{{ID: "cancel"}}, report: func() {}}
	_, startOld, markAt := oldControl(foreign)
	cmd := startOld + " && rm -f " + shQuote(markAt) + " " + stoppedMark
	if foreign != nil {
		cmd = startOld + " && rm -f " + shQuote(markAt)
	}
	if _, err := onOld(ctx, r, old, cmd, packageTimeout); err != nil {
		return r.log, err
	}
	_, _ = onOld(ctx, r, old, "sed -i '/ "+moveKeyLabel+"$/d' /root/.ssh/authorized_keys", shortTimeout)
	if sess != nil {
		_, _ = r.sh(ctx, "cd "+installDir+" 2>/dev/null && docker compose down 2>&1; rm -rf "+installDir+" "+moveKey+" "+moveKey+".pub "+moveKnown, packageTimeout)
	}
	return r.log, nil
}
