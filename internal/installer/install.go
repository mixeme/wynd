package installer

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Установка на чистый сервер (план 45, срез 2): Docker, образ Wynd, свой
// Caddy с сертификатом. Не обёртка над deploy/install.sh, а шаги: у каждого
// «уже сделано?», «сделать», «убедиться». Повторный запуск пропускает
// сделанное, оборванное доделывает. Результат — тот же, что у ручной
// установки по deploy/docker: /opt/wynd/compose.yaml и Caddyfile.

const (
	installDir = "/opt/wynd"
	// Так зовётся образ, приехавший файлом (TarImage); версия — тег.
	imageName = "wynd"
	// DefaultRegistry — где лежат образы выпусков (.github/workflows/image.yaml).
	DefaultRegistry = "ghcr.io/mixeme/wynd"
	// DefaultSource — открытый код выпусков: из него сервер собирает образ сам.
	DefaultSource = "https://github.com/mixeme/wynd"
	caddyImage    = "caddy:2-alpine"

	shortTimeout = 2 * time.Minute
	// Пакеты на дешёвом VPS (одно ядро, медленный диск) ставятся десятки
	// минут: Docker на пробном сервере — больше пятнадцати.
	packageTimeout = 40 * time.Minute
	imageTimeout   = 40 * time.Minute
	// Сборка из исходников: на одном ядре с подкачкой — 8 минут (замер
	// 2026-10-09), срок — с запасом на сервер медленнее.
	buildTimeout = time.Hour
	// Сборке клиента (vite) мало памяти маленького сервера: на 708 МБ без
	// подкачки сервер не падает с ошибкой, а перестаёт отвечать. Замер: на
	// пике занято 1,6 ГБ памяти вместе с подкачкой.
	buildMemoryKB = 2 << 20
	// Сборочный кэш и образ — 2 ГБ по замеру, плюс запас.
	minFreeForBuild = 2300 << 20
	healthWait      = 90 * time.Second
	// Образ, Caddy и база должны поместиться и после установки Docker.
	minFreeForImage = 1 << 30
)

// pollInterval — как часто переспрашиваем, пока ждём (тесты его сжимают).
var pollInterval = 5 * time.Second

// certWait — сколько ждём сертификат: Let's Encrypt отвечает за секунды, но
// Caddy после отказа повторяет не сразу.
var certWait = 4 * time.Minute

// Spec — что ставим.
type Spec struct {
	// Domain — адрес сайта, уже проверенный: только латиница, цифры, точки и дефисы.
	Domain string
	// Version — версия Wynd, она же тег образа.
	Version string
	// Image — откуда сервер возьмёт образ Wynd.
	Image ImageSource
	// FixClock — осмотр нашёл, что часы не сверяются.
	FixClock bool
	// Proxy — чужой веб-сервер, за который встаёт Wynd (ProxyNginx,
	// ProxyCaddy); пусто — ставим свой Caddy.
	Proxy string
	// TestCert — сертификат проверочного сервера Let's Encrypt: браузеры ему не
	// верят. Только для проб разработчика — у настоящего центра пять
	// сертификатов на адрес в неделю.
	TestCert bool
	// Probe открывает адрес с этого компьютера и возвращает тело ответа.
	// Пусто — обычный HTTPS-запрос.
	Probe func(ctx context.Context, url string) (string, error)
}

// imageTag — имя образа, как оно записано в compose.yaml. Образ из реестра
// зовётся полным именем: администратор потом обновляет его обычным
// docker compose pull.
func (s Spec) imageTag() string {
	if s.Image != nil {
		return s.Image.Ref(s.Version)
	}
	return imageName + ":" + s.Version
}

// ImageSource — способ доставить образ Wynd на сервер.
type ImageSource interface {
	// Ref — имя образа этой версии на сервере.
	Ref(version string) string
	// Provide делает так, чтобы на сервере появился образ tag.
	Provide(ctx context.Context, r *Run, tag string) error
}

// StepStatus — состояние шага для окна.
type StepStatus string

const (
	StepPending StepStatus = "pending"
	StepRunning StepStatus = "running"
	// StepDone — сделан сейчас или был сделан раньше: человеку разницы нет.
	StepDone   StepStatus = "done"
	StepFailed StepStatus = "failed"
)

// StepState — строка шага в окне установки.
type StepState struct {
	ID     string     `json:"id"`
	Title  string     `json:"title"`
	Status StepStatus `json:"status"`
	// Note — что происходит прямо сейчас: «ждём ответа Let's Encrypt».
	Note string `json:"note,omitempty"`
}

// LogEntry — команда и её вывод, для «показать, что выполняется».
type LogEntry struct {
	Command string `json:"command"`
	Output  string `json:"output"`
}

// Failure — шаг не прошёл: что это значит и что сделать, словами человека.
type Failure struct {
	Step    string `json:"step"`
	Message string `json:"message"`
	Advice  string `json:"advice"`
	// Unpublished — готового образа этой версии нет; см. StepError.
	Unpublished bool `json:"unpublished,omitempty"`
}

// Progress — всё, что окно показывает об установке.
type Progress struct {
	Steps   []StepState `json:"steps"`
	Log     []LogEntry  `json:"log"`
	Running bool        `json:"running"`
	Done    bool        `json:"done"`
	Failure *Failure    `json:"failure,omitempty"`
	// Site — адрес работающего сайта.
	Site string `json:"site,omitempty"`
	// Link — ссылка первого запуска. Пусто при Done — сервер уже настроен.
	Link string `json:"link,omitempty"`
}

// StepError — отказ шага, уже объяснённый человеку.
type StepError struct {
	Message string
	Advice  string
	// Unpublished — готового образа этой версии в реестре нет: окно предложит
	// собрать Wynd на сервере.
	Unpublished bool
}

func (e *StepError) Error() string { return e.Message }

// commandError — команда на сервере кончилась ошибкой.
type commandError struct {
	command string
	output  string
	code    int
}

func (e *commandError) Error() string {
	return fmt.Sprintf("installer: command failed (%d): %s", e.code, firstLine(e.command))
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}

// Run — один прогон установки: подключение, что ставим, и журнал команд.
type Run struct {
	sess *Session
	spec Spec

	mu      sync.Mutex
	steps   []StepState
	log     []LogEntry
	current int
	// link — ссылка первого запуска; в журнал не попадает.
	link   string
	report func()
}

// maxLogOutput — сколько вывода одной команды держим для окна.
const maxLogOutput = 8 << 10

func (r *Run) record(command, output string) {
	if len(output) > maxLogOutput {
		output = "…\n" + output[len(output)-maxLogOutput:]
	}
	r.mu.Lock()
	r.log = append(r.log, LogEntry{Command: strings.TrimSpace(command), Output: strings.TrimRight(output, "\n")})
	r.mu.Unlock()
	r.report()
}

// note пишет под текущим шагом, что сейчас происходит.
func (r *Run) note(text string) {
	r.mu.Lock()
	changed := r.steps[r.current].Note != text
	r.steps[r.current].Note = text
	r.mu.Unlock()
	if changed {
		r.report()
	}
}

// sh выполняет команду и кладёт её в журнал. Код выхода — в результате.
func (r *Run) sh(ctx context.Context, command string, timeout time.Duration) (ExecResult, error) {
	res, err := r.sess.Exec(ctx, command, nil, timeout)
	r.record(command, res.Output)
	return res, err
}

// must — как sh, но ненулевой код выхода — ошибка.
func (r *Run) must(ctx context.Context, command string, timeout time.Duration) (string, error) {
	res, err := r.sh(ctx, command, timeout)
	if err != nil {
		return res.Output, err
	}
	if res.Code != 0 {
		return res.Output, &commandError{command: command, output: res.Output, code: res.Code}
	}
	return res.Output, nil
}

// ok — команда-вопрос: код выхода 0. В журнал не идёт: это не изменение.
func (r *Run) ok(ctx context.Context, command string) (bool, error) {
	res, err := r.sess.Exec(ctx, command, nil, shortTimeout)
	if err != nil {
		return false, err
	}
	return res.Code == 0, nil
}

// put кладёт файл на сервер: сначала рядом, потом переименованием — оборванная
// запись не оставит половину файла на месте настоящего.
func (r *Run) put(ctx context.Context, path, content string) error {
	command := fmt.Sprintf("cat > %s.new && mv %s.new %s", path, path, path)
	res, err := r.sess.Exec(ctx, command, strings.NewReader(content), shortTimeout)
	r.record("запись файла "+path, res.Output)
	if err != nil {
		return err
	}
	if res.Code != 0 {
		return &commandError{command: command, output: res.Output, code: res.Code}
	}
	return nil
}

type step struct {
	id    string
	title string
	// done — шаг уже сделан (сейчас или в прошлый запуск): пропустить.
	done func(ctx context.Context, r *Run) (bool, error)
	do   func(ctx context.Context, r *Run) error
	// verify — убедиться, что сделанное работает.
	verify func(ctx context.Context, r *Run) error
	// explain переводит отказ команды в слова человека.
	explain func(err error, output string) *StepError
}

func steps(spec Spec) []step {
	list := []step{dockerStep()}
	if spec.FixClock {
		list = append(list, clockStep())
	}
	list = append(list, imageStep(spec), filesStep(spec), startStep(spec))
	switch spec.Proxy {
	case "":
		list = append(list, certStep(spec))
	case ProxyCaddy:
		list = append(list, siteStep(spec), certStep(spec))
	default:
		list = append(list, siteStep(spec), nginxCertStep(spec))
	}
	return append(list, linkStep(spec))
}

// StepTitles — шаги установки по порядку, для окна до её начала.
func StepTitles(spec Spec) []StepState {
	var res []StepState
	for _, s := range steps(spec) {
		res = append(res, StepState{ID: s.id, Title: s.title, Status: StepPending})
	}
	return res
}

// Install ставит Wynd. report зовётся при каждом изменении: окно показывает
// шаги живьём. Возвращает итог — он же последнее, что видел report.
func Install(ctx context.Context, sess *Session, spec Spec, report func(Progress)) Progress {
	list := steps(spec)
	r := &Run{sess: sess, spec: spec, steps: StepTitles(spec)}
	result := Progress{Running: true}
	snapshot := func() Progress {
		r.mu.Lock()
		defer r.mu.Unlock()
		p := result
		p.Steps = append([]StepState(nil), r.steps...)
		p.Log = append([]LogEntry(nil), r.log...)
		return p
	}
	r.report = func() {
		if report != nil {
			report(snapshot())
		}
	}
	set := func(i int, status StepStatus) {
		r.mu.Lock()
		r.current = i
		r.steps[i].Status = status
		if status != StepRunning {
			r.steps[i].Note = ""
		}
		r.mu.Unlock()
		r.report()
	}
	for i, s := range list {
		set(i, StepRunning)
		err := runStep(ctx, r, s)
		if err != nil {
			set(i, StepFailed)
			r.mu.Lock()
			result.Running = false
			result.Failure = failure(s, err)
			r.mu.Unlock()
			r.report()
			return snapshot()
		}
		set(i, StepDone)
	}
	r.mu.Lock()
	result.Running = false
	result.Done = true
	result.Site = "https://" + spec.Domain
	result.Link = r.link
	r.mu.Unlock()
	r.report()
	return snapshot()
}

func runStep(ctx context.Context, r *Run, s step) error {
	if s.done != nil {
		done, err := s.done(ctx, r)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
	if s.do != nil {
		if err := s.do(ctx, r); err != nil {
			return err
		}
	}
	if s.verify != nil {
		return s.verify(ctx, r)
	}
	return nil
}

func failure(s step, err error) *Failure {
	f := &Failure{Step: s.id}
	var explained *StepError
	var cmd *commandError
	switch {
	case errors.As(err, &explained):
		f.Message, f.Advice, f.Unpublished = explained.Message, explained.Advice, explained.Unpublished
	case errors.Is(err, ErrTimeout):
		f.Message = fmt.Sprintf("Шаг «%s» идёт дольше, чем мы ждали", s.title)
		f.Advice = "Сервер, скорее всего, ещё занят этим шагом — он просто медленный. Подождите несколько минут и нажмите «Повторить»: сделанное не пропадёт."
	case errors.Is(err, ErrUnreachable):
		f.Message = "Связь с сервером оборвалась"
		f.Advice = "Нажмите «Повторить» — подключимся снова и продолжим с этого шага."
	default:
		output := ""
		if errors.As(err, &cmd) {
			output = cmd.output
		}
		if s.explain != nil {
			if e := s.explain(err, output); e != nil {
				f.Message, f.Advice = e.Message, e.Advice
				break
			}
		}
		f.Message = fmt.Sprintf("Шаг «%s» не прошёл", s.title)
		f.Advice = "Нажмите «Повторить»; если не поможет — в «подробностях» видно, на чём остановились."
	}
	return f
}

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// aptBusy — сервер сам ставит обновления и держит замок пакетов.
func aptBusy(output string) *StepError {
	if !containsAny(output, "Could not get lock", "Unable to acquire the dpkg frontend lock", "is another process using it") {
		return nil
	}
	return &StepError{
		Message: "Сервер занят установкой программ",
		Advice:  "Либо он сам ставит обновления (так бывает в первые минуты после запуска), либо ещё не закончил нашу прошлую попытку — на медленном сервере она идёт долго. Подождите несколько минут и нажмите «Повторить».",
	}
}

func noInternet(output, what string) *StepError {
	if !containsAny(output, "Could not resolve", "Failed to fetch", "Temporary failure", "Connection timed out", "Could not connect", "curl: (", "i/o timeout", "TLS handshake timeout", "toomanyrequests", "connection refused") {
		return nil
	}
	return &StepError{
		Message: "Сервер не смог скачать " + what,
		Advice:  "Похоже, у сервера нет доступа к нужному сайту или связь оборвалась. Подождите минуту и нажмите «Повторить». Если повторяется — напишите в поддержку хостинга: серверу нужен выход в интернет.",
	}
}

// Замок пакетов держит сам сервер (обновления после первого запуска) или
// наша же прошлая попытка, от которой оборвалась связь: apt ждёт его до
// десяти минут (DPkg::Lock::Timeout), а не отказывает сразу.
//
// dockerScript ставит Docker из его собственного репозитория — так же, как
// велит документация Docker для Debian и Ubuntu: в поставке самих систем нет
// docker compose.
const dockerScript = `set -e
export DEBIAN_FRONTEND=noninteractive
WAIT="-o DPkg::Lock::Timeout=600"
apt-get $WAIT update -qq
apt-get $WAIT install -y -qq ca-certificates curl
install -m 0755 -d /etc/apt/keyrings
. /etc/os-release
curl -fsSL "https://download.docker.com/linux/$ID/gpg" -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/$ID $VERSION_CODENAME stable" > /etc/apt/sources.list.d/docker.list
apt-get $WAIT update -qq
apt-get $WAIT install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin
systemctl enable --now docker`

// Готов — значит, и программа на месте, и служба отвечает: после оборванной
// установки пакетов docker уже есть, а службы ещё нет (проба 2026-10-09).
const dockerReady = `docker compose version >/dev/null 2>&1 && docker info >/dev/null 2>&1`

func dockerStep() step {
	return step{
		id:    "docker",
		title: "Docker",
		done:  func(ctx context.Context, r *Run) (bool, error) { return r.ok(ctx, dockerReady) },
		do: func(ctx context.Context, r *Run) error {
			r.note("скачиваем и ставим — от пары минут до получаса на медленном сервере")
			_, err := r.must(ctx, dockerScript, packageTimeout)
			return err
		},
		verify: func(ctx context.Context, r *Run) error {
			_, err := r.must(ctx, "docker --version && docker compose version && docker info --format '{{.ServerVersion}}'", shortTimeout)
			return err
		},
		explain: func(_ error, output string) *StepError {
			if e := aptBusy(output); e != nil {
				return e
			}
			return noInternet(output, "Docker")
		},
	}
}

const clockSynced = `test "$(timedatectl show -p NTPSynchronized --value 2>/dev/null)" = yes`

func clockStep() step {
	return step{
		id:    "clock",
		title: "Синхронизация часов",
		done:  func(ctx context.Context, r *Run) (bool, error) { return r.ok(ctx, clockSynced) },
		do: func(ctx context.Context, r *Run) error {
			_, err := r.must(ctx, `set -e
export DEBIAN_FRONTEND=noninteractive
WAIT="-o DPkg::Lock::Timeout=600"
apt-get $WAIT install -y -qq systemd-timesyncd
timedatectl set-ntp true`, packageTimeout)
			return err
		},
		verify: func(ctx context.Context, r *Run) error {
			r.note("ждём, пока сервер сверит время")
			deadline := time.Now().Add(time.Minute)
			for {
				synced, err := r.ok(ctx, clockSynced)
				if err != nil {
					return err
				}
				if synced {
					return nil
				}
				if time.Now().After(deadline) {
					return &StepError{
						Message: "Часы сервера не сверились",
						Advice:  "Синхронизацию времени мы включили, но сервер пока не получил точное время. Подождите минуту и нажмите «Повторить». Без верных часов не работают коды входа и сертификат.",
					}
				}
				if err := sleep(ctx, pollInterval); err != nil {
					return err
				}
			}
		},
		explain: func(_ error, output string) *StepError { return aptBusy(output) },
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("%w: %v", ErrUnreachable, ctx.Err())
	case <-time.After(d):
		return nil
	}
}

func imageStep(spec Spec) step {
	tag := spec.imageTag()
	present := "docker image inspect " + tag + " >/dev/null 2>&1"
	return step{
		id:    "image",
		title: "Wynd " + spec.Version,
		done:  func(ctx context.Context, r *Run) (bool, error) { return r.ok(ctx, present) },
		do: func(ctx context.Context, r *Run) error {
			if spec.Image == nil {
				return &StepError{
					Message: "Установщик не знает, откуда взять Wynd",
					Advice:  "Это ошибка сборки установщика. Скачайте свежую версию установщика.",
				}
			}
			out, err := r.must(ctx, "df -Pk / | tail -n 1", shortTimeout)
			if err != nil {
				return err
			}
			if free, ok := freeBytes(out); ok && free < minFreeForImage {
				return &StepError{
					Message: "На сервере кончилось место",
					Advice:  "После установки Docker свободно " + formatGB(free) + ", а для Wynd нужен хотя бы " + formatGB(minFreeForImage) + ". Увеличьте диск в панели хостинга и нажмите «Повторить».",
				}
			}
			return spec.Image.Provide(ctx, r, tag)
		},
		verify: func(ctx context.Context, r *Run) error {
			_, err := r.must(ctx, "docker image inspect -f '{{.Id}}' "+tag, shortTimeout)
			return err
		},
		explain: func(_ error, output string) *StepError {
			if containsAny(output, "no space left on device") {
				return &StepError{
					Message: "На сервере кончилось место",
					Advice:  "Wynd не поместился на диск. Увеличьте диск в панели хостинга и нажмите «Повторить».",
				}
			}
			return noInternet(output, "Wynd")
		},
	}
}

func freeBytes(dfLine string) (uint64, bool) {
	fields := strings.Fields(dfLine)
	if len(fields) < 4 {
		return 0, false
	}
	var kb uint64
	if _, err := fmt.Sscan(fields[3], &kb); err != nil {
		return 0, false
	}
	return kb * 1024, true
}

// TarImage — образ из файла на этом компьютере (docker save, можно сжатый):
// уезжает на сервер по тому же SSH. Реестра образов у Wynd пока нет.
type TarImage struct {
	Path string
}

type countingReader struct {
	r     io.Reader
	total int64
	read  int64
	last  time.Time
	note  func(string)
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.read += int64(n)
	if err == io.EOF {
		// Файл уехал; дальше сервер сам раскладывает образ по диску — на
		// медленном это минуты.
		c.note("сервер распаковывает Wynd")
		return n, err
	}
	if time.Since(c.last) > time.Second {
		c.last = time.Now()
		c.note(fmt.Sprintf("отправляем на сервер: %d из %d МБ", c.read>>20, c.total>>20))
	}
	return n, err
}

// Ref — образ из файла зовётся коротко: реестра за ним нет.
func (t TarImage) Ref(version string) string { return imageName + ":" + version }

// RegistryImage — образ из реестра: сервер скачивает его сам.
type RegistryImage struct {
	// Repo — имя без тега, например ghcr.io/mixeme/wynd.
	Repo string
}

// Ref — полное имя образа в реестре.
func (g RegistryImage) Ref(version string) string { return g.Repo + ":" + version }

// publishedScript спрашивает реестр, есть ли сборка этой версии под
// архитектуру сервера, и печатает код ответа. Спрашивает сам сервер: качать
// будет он. Метка «<версия>-<архитектура>» — сама сборка, а не оглавление:
// оглавление отвечает 200 и тогда, когда сборок под ним уже нет.
func (g RegistryImage) publishedScript(version, arch string) string {
	host, path, _ := strings.Cut(g.Repo, "/")
	return `t=$(curl -fsS --max-time 15 ` + shQuote("https://"+host+"/token?scope=repository:"+path+":pull") + ` 2>/dev/null | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
[ -n "$t" ] || exit 0
curl -s --max-time 15 -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $t" -H 'Accept: application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json' ` + shQuote("https://"+host+"/v2/"+path+"/manifests/"+version+"-"+arch)
}

// Published — выложена ли версия в реестр. known ложно, когда узнать не
// вышло (нет связи с реестром, незнакомая архитектура): тогда план составляем
// как обычно, а отказ скажет своё на шаге.
func (g RegistryImage) Published(ctx context.Context, sess *Session, version, arch string) (published, known bool) {
	if arch == "" {
		return false, false
	}
	res, err := sess.Exec(ctx, g.publishedScript(version, arch), nil, shortTimeout)
	if err != nil {
		return false, false
	}
	switch strings.TrimSpace(res.Output) {
	case "200":
		return true, true
	case "404":
		return false, true
	}
	return false, false
}

// Provide скачивает образ на сервер.
func (g RegistryImage) Provide(ctx context.Context, r *Run, tag string) error {
	r.note("сервер скачивает Wynd")
	out, err := r.must(ctx, "docker pull -q "+tag+" 2>&1", imageTimeout)
	if err != nil && containsAny(out, "manifest unknown", "not found", "denied", "unauthorized") {
		return &StepError{
			Message:     "Этой версии Wynd в хранилище ещё нет",
			Advice:      "Установщик ищет " + tag + ", а его там нет: версия ещё не выложена или хранилище закрыто. Скачайте свежий установщик или попробуйте позже.",
			Unpublished: true,
		}
	}
	return err
}

// Provide отправляет файл образа в docker load.
func (t TarImage) Provide(ctx context.Context, r *Run, tag string) error {
	f, err := os.Open(t.Path)
	if err != nil {
		return &StepError{
			Message: "Не нашли файл с Wynd на этом компьютере",
			Advice:  "Установщик ждал образ в файле " + t.Path + ". Скачайте установщик заново.",
		}
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	reader := &countingReader{r: f, total: info.Size(), note: r.note}
	res, err := r.sess.Exec(ctx, "docker load", reader, imageTimeout)
	r.record("docker load  (образ с этого компьютера)", res.Output)
	if err != nil {
		return err
	}
	if res.Code != 0 {
		return &commandError{command: "docker load", output: res.Output, code: res.Code}
	}
	// Файл мог быть собран под другим именем — даём своё.
	for _, line := range strings.Split(res.Output, "\n") {
		loaded, ok := strings.CutPrefix(strings.TrimSpace(line), "Loaded image: ")
		if ok && loaded != tag {
			_, err := r.must(ctx, "docker tag "+shQuote(loaded)+" "+tag, shortTimeout)
			return err
		}
	}
	return nil
}

// SourceImage — образ, собранный на самом сервере из открытого кода выпуска:
// для тех, кому нельзя ставить чужой готовый образ, и на случай, когда в
// реестре версии нет. Сервер клонирует метку v<версия> и собирает по
// deploy/docker/Dockerfile — так же, как это делают руками.
type SourceImage struct {
	// Repo — адрес открытого репозитория, например https://github.com/mixeme/wynd.
	Repo string
}

// Ref — собранный образ зовётся коротко: реестра за ним нет.
func (s SourceImage) Ref(version string) string { return imageName + ":" + version }

// sourceDir — куда клонируем на время сборки.
const sourceDir = "/var/tmp/wynd-src"

// gitScript ставит git, если его нет: клонировать больше нечем.
const gitScript = `command -v git >/dev/null 2>&1 || {
  export DEBIAN_FRONTEND=noninteractive
  apt-get -o DPkg::Lock::Timeout=600 install -y -qq git
}`

// buildSwap — временный файл подкачки на время сборки; buildLock — замок,
// чтобы две сборки не пошли разом; buildCacheOurs — пометка «сборочный кэш
// Docker до нашей сборки был пуст».
const (
	buildSwap      = "/var/tmp/wynd-build.swap"
	buildLock      = "/var/tmp/wynd-build.lock"
	buildCacheOurs = "/var/tmp/wynd-build.cache-ours"
)

// Сборочный кэш у Docker один на всех, своё от чужого в нём не отличить.
// Поэтому перед сборкой смотрим, пуст ли он: пуст — после сборки в нём только
// наше (базовые образы node и golang, слои зависимостей — около 2 ГБ), и его
// можно убрать целиком. Не пуст — на сервере собирает кто-то ещё, не трогаем.
// Пометка переживает оборванную сборку: повтор начнёт уже с непустым кэшем,
// но это всё ещё наш кэш.
const (
	markCacheOurs  = `if [ "$(docker system df --format '{{.Type}}={{.TotalCount}}' 2>/dev/null | sed -n 's/^Build Cache=//p')" = 0 ]; then : > ` + buildCacheOurs + `; fi`
	pruneCacheOurs = `if [ -e ` + buildCacheOurs + ` ]; then docker builder prune -af >/dev/null 2>&1 || true; rm -f ` + buildCacheOurs + `; fi`
)

const swapOff = "swapoff " + buildSwap + " 2>/dev/null || true; rm -f " + buildSwap

// Коды выхода сценария сборки, которые установщик объясняет сам.
const (
	exitNoRoom = 3
	exitNoTag  = 4
)

// buildScript — вся сборка одной командой: клон метки выпуска, подкачка,
// docker build, уборка (и сборочного кэша, если он весь наш). Одной — потому что связь может оборваться, а сервер
// должен довести дело и убрать за собой сам (проба 2026-10-09: соединение
// пропало на первой минуте, сборка шла дальше, подкачка и исходники остались
// бы навсегда). Замок: «Повторить» после обрыва ждёт идущую сборку и берёт её
// образ, а не запускает вторую — двум памяти не хватит.
//
// Памяти мало — на время сборки заводим подкачку: без неё маленький сервер не
// отказывает, а перестаёт отвечать. Место под неё и под сборочный кэш
// проверяем заранее.
func buildScript(repo, version, tag string) string {
	return `set -e
exec 9>` + buildLock + `
flock 9
if docker image inspect ` + tag + ` >/dev/null 2>&1; then exit 0; fi
kb=$(awk '/^(MemTotal|SwapTotal):/{s+=$2} END{print s}' /proc/meminfo)
swap=0
if [ "$kb" -lt ` + fmt.Sprint(buildMemoryKB) + ` ]; then swap=$(( (` + fmt.Sprint(buildMemoryKB) + ` - kb) / 1024 + 1 )); fi
free=$(df -Pk / | awk 'END{print $4}')
need=$(( ` + fmt.Sprint(minFreeForBuild>>10) + ` + swap * 1024 ))
if [ "$free" -lt "$need" ]; then echo "WYND_NO_ROOM $free $need"; exit ` + fmt.Sprint(exitNoRoom) + `; fi
cleanup() { ` + swapOff + `; rm -rf ` + sourceDir + `; }
trap cleanup EXIT
cleanup
if [ "$swap" -gt 0 ]; then
  fallocate -l "${swap}M" ` + buildSwap + ` 2>/dev/null || dd if=/dev/zero of=` + buildSwap + ` bs=1M count="$swap" status=none
  chmod 600 ` + buildSwap + `
  mkswap -q ` + buildSwap + `
  swapon ` + buildSwap + `
fi
git -c advice.detachedHead=false clone -q --depth 1 --branch ` + shQuote("v"+version) + ` ` + shQuote(repo) + ` ` + sourceDir + ` 2>&1 || exit ` + fmt.Sprint(exitNoTag) + `
cd ` + sourceDir + `
` + markCacheOurs + `
docker build -q -f deploy/docker/Dockerfile --build-arg VERSION=` + shQuote(version) + ` -t ` + tag + ` . 2>&1
` + pruneCacheOurs
}

// Provide собирает образ на сервере. Сборочный кэш после удачной сборки
// убирается, если до неё был пуст (markCacheOurs); после неудачной остаётся —
// повтор не качает всё заново, — и тогда его уберёт удачный повтор или откат.
func (s SourceImage) Provide(ctx context.Context, r *Run, tag string) error {
	_, version, _ := strings.Cut(tag, ":")
	if _, err := r.must(ctx, gitScript, packageTimeout); err != nil {
		return err
	}
	r.note("сервер собирает Wynd из исходного кода — на маленьком сервере это минут десять")
	res, err := r.sh(ctx, buildScript(s.Repo, version, tag), buildTimeout)
	if err != nil {
		return err
	}
	out := res.Output
	switch {
	case res.Code == 0:
		return nil
	case res.Code == exitNoRoom:
		var free, need uint64
		if i := strings.Index(out, "WYND_NO_ROOM "); i >= 0 {
			_, _ = fmt.Sscan(out[i+len("WYND_NO_ROOM "):], &free, &need)
		}
		return &StepError{
			Message: "На сервере мало места, чтобы собрать Wynd",
			Advice:  "Для сборки из исходного кода нужно " + formatGB(need<<10) + " свободного места, а есть " + formatGB(free<<10) + ". Поставьте готовый Wynd из хранилища или увеличьте диск в панели хостинга.",
		}
	case res.Code == exitNoTag && containsAny(out, "not found in upstream", "Remote branch", "Repository not found", "returned error: 404"):
		return &StepError{
			Message: "Этой версии Wynd в открытом коде ещё нет",
			Advice:  "Установщик ищет метку v" + version + " в " + s.Repo + ", а её там нет. Скачайте свежий установщик или попробуйте позже.",
		}
	case containsAny(out, "heap out of memory", "exit code: 137", "signal: killed", "Killed", "cannot allocate memory", "out of memory"):
		return &StepError{
			Message: "Серверу не хватило памяти, чтобы собрать Wynd",
			Advice:  "Сборка из исходного кода требует больше памяти, чем работа Wynd. Поставьте готовый Wynd из хранилища или возьмите сервер с большей памятью.",
		}
	}
	return &commandError{command: "сборка Wynd из исходного кода", output: out, code: res.Code}
}

// ComposeFile — /opt/wynd/compose.yaml: то же, что deploy/docker/compose.yaml,
// но с готовым образом вместо сборки из исходников.
func ComposeFile(spec Spec) string {
	if spec.Proxy != "" {
		return ownProxyCompose(spec)
	}
	return `# Wynd со своим Caddy. Файл создал установщик Wynd; устроен так же, как
# deploy/docker/compose.yaml в исходниках, — обслуживается по той же документации.

services:
  wynd:
    image: ` + spec.imageTag() + `
    volumes:
      - wynd-data:/data
    environment:
      WYND_PUBLIC_URL: https://` + spec.Domain + `
      # Only the caddy service can reach wynd (no published port), so its
      # X-Forwarded-For may be trusted for rate limiting.
      WYND_TRUSTED_PROXIES: 10.0.0.0/8,172.16.0.0/12,192.168.0.0/16
    restart: unless-stopped

  caddy:
    image: ` + caddyImage + `
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy-data:/data
      - caddy-config:/config
    depends_on:
      - wynd
    restart: unless-stopped

volumes:
  wynd-data:
  caddy-data:
  caddy-config:
`
}

// CaddyFile — /opt/wynd/Caddyfile: сайт из deploy/caddy/Caddyfile с адресом
// человека. Почту для Let's Encrypt не спрашиваем: Caddy работает и без неё.
func CaddyFile(spec Spec) string {
	return `# Wynd production reverse proxy for docker compose.
# Файл создал установщик Wynd.

` + spec.Domain + ` {
	header Strict-Transport-Security "max-age=31536000; includeSubDomains"
	reverse_proxy wynd:7676 {
		flush_interval -1
		header_up X-Real-IP {remote_host}
		header_up X-Forwarded-For {remote_host}
		transport http {
			read_timeout 300s
		}
	}

	request_body {
		max_size 100MB
	}
}
`
}

func sum(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

func filesStep(spec Spec) step {
	type file struct{ path, content string }
	files := []file{{installDir + "/compose.yaml", ComposeFile(spec)}}
	if spec.Proxy == "" {
		files = append(files, file{installDir + "/Caddyfile", CaddyFile(spec)})
	}
	return step{
		id:    "files",
		title: "Настройка",
		done: func(ctx context.Context, r *Run) (bool, error) {
			command := "sha256sum"
			for _, f := range files {
				command += " " + f.path
			}
			res, err := r.sess.Exec(ctx, command+" 2>/dev/null", nil, shortTimeout)
			if err != nil {
				return false, err
			}
			for _, f := range files {
				if !strings.Contains(res.Output, sum(f.content)) {
					return false, nil
				}
			}
			return true, nil
		},
		do: func(ctx context.Context, r *Run) error {
			// Wynd с другим адресом уже стоит: смена адреса — не установка.
			res, err := r.sess.Exec(ctx, "grep -o 'WYND_PUBLIC_URL: .*' "+files[0].path+" 2>/dev/null", nil, shortTimeout)
			if err != nil {
				return err
			}
			if old, ok := strings.CutPrefix(strings.TrimSpace(res.Output), "WYND_PUBLIC_URL: "); ok && old != "https://"+spec.Domain {
				return &StepError{
					Message: "На сервере уже стоит Wynd с другим адресом",
					Advice:  "Он настроен на " + old + ". Смену адреса установщик пока не умеет. Чтобы поставить заново с адресом " + spec.Domain + ", сначала нажмите «Откатить установку».",
				}
			}
			if _, err := r.must(ctx, "mkdir -p "+installDir, shortTimeout); err != nil {
				return err
			}
			for _, f := range files {
				// Файл уже есть и он другой — сначала копия рядом.
				backup := fmt.Sprintf(`if [ -f %[1]s ] && ! echo "%[2]s  %[1]s" | sha256sum -c --status; then cp -p %[1]s %[1]s.bak-$(date +%%Y%%m%%d-%%H%%M%%S); fi`, f.path, sum(f.content))
				if _, err := r.must(ctx, backup, shortTimeout); err != nil {
					return err
				}
				if err := r.put(ctx, f.path, f.content); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func startStep(spec Spec) step {
	compose := "cd " + installDir + " && docker compose "
	return step{
		id:    "start",
		title: "Запуск",
		do: func(ctx context.Context, r *Run) error {
			if spec.Proxy == "" {
				r.note("скачиваем веб-сервер и запускаем")
			}
			_, err := r.must(ctx, compose+"up -d --quiet-pull 2>&1", packageTimeout)
			return err
		},
		verify: func(ctx context.Context, r *Run) error {
			r.note("ждём, пока Wynd ответит")
			healthy := compose + `ps -q wynd | xargs -r docker inspect -f '{{.State.Health.Status}}' | grep -qx healthy`
			deadline := time.Now().Add(healthWait)
			for {
				up, err := r.ok(ctx, healthy)
				if err != nil {
					return err
				}
				if up {
					return nil
				}
				if time.Now().After(deadline) {
					_, _ = r.sh(ctx, compose+"ps; "+compose+"logs --no-color --tail 20 wynd 2>&1 | grep -v 'bootstrap URL'", shortTimeout)
					return &StepError{
						Message: "Wynd запустился, но не отвечает",
						Advice:  "Нажмите «Повторить». Если не поможет — в «подробностях» видно, что пишет сам Wynd.",
					}
				}
				if err := sleep(ctx, pollInterval); err != nil {
					return err
				}
			}
		},
		explain: func(_ error, output string) *StepError {
			busy := containsAny(output, "address already in use", "port is already allocated")
			if busy && spec.Proxy != "" {
				return &StepError{
					Message: "Порт 7676 на сервере кто-то занял",
					Advice:  "Через него веб-сервер передаёт запросы Wynd. Освободите порт и нажмите «Повторить».",
				}
			}
			if busy {
				return &StepError{
					Message: "Порты 80 или 443 на сервере кто-то занял",
					Advice:  "При осмотре они были свободны. Через них сайт открывается в браузере. Освободите их и нажмите «Повторить».",
				}
			}
			return noInternet(output, "веб-сервер Caddy")
		},
	}
}

// httpProbe открывает адрес с этого компьютера. Сертификат проверяется как
// в браузере: непроверенный сайт — отказ.
func httpProbe(ctx context.Context, url string) (string, error) {
	return httpGet(ctx, http.DefaultClient, url)
}

// untrustedProbe — то же, но сертификату не верим: для проб с проверочным
// сервером Let's Encrypt (Spec.TestCert).
func untrustedProbe(ctx context.Context, url string) (string, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // только Spec.TestCert
	defer transport.CloseIdleConnections()
	return httpGet(ctx, &http.Client{Transport: transport}, url)
}

func httpGet(ctx context.Context, client *http.Client, url string) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", res.StatusCode)
	}
	return string(body), nil
}

// probe открывает адрес сайта: с этого компьютера, а если не вышло — с
// сервера. Проверка с сервера слабее (закрытые снаружи порты она не увидит),
// но сертификат Let's Encrypt выдаёт, только зайдя на сервер снаружи: раз он
// есть и сайт отвечает, порты открыты. А с компьютера человека сайт может не
// открыться по причинам, которые к серверу не относятся: мёртвый DNS за VPN,
// медленный прокси (проба 2026-10-09: ответ через 17 секунд и через раз).
func (r *Run) probe(ctx context.Context, url string) (string, error) {
	probe, curl := r.spec.Probe, "curl -fsS --max-time 20 "
	if probe == nil {
		probe = httpProbe
		if r.spec.TestCert {
			probe = untrustedProbe
		}
	}
	if r.spec.TestCert {
		curl = "curl -kfsS --max-time 20 "
	}
	body, err := probe(ctx, url)
	if err == nil {
		return body, nil
	}
	res, execErr := r.sess.Exec(ctx, curl+shQuote(url), nil, shortTimeout)
	if execErr != nil {
		return "", execErr
	}
	if res.Code != 0 {
		return "", fmt.Errorf("с этого компьютера: %v; с сервера: %s", err, strings.TrimSpace(res.Output))
	}
	return res.Output, nil
}

// certStep ждёт сертификат, который Caddy получает сам, — свой в контейнере
// или чужой, которому мы добавили сайт.
func certStep(spec Spec) step {
	logs := "cd " + installDir + " && docker compose logs --no-color --tail 30 caddy 2>&1"
	if spec.Proxy != "" {
		logs = "journalctl -u caddy --no-pager -n 30 2>&1"
	}
	return step{
		id:    "cert",
		title: "Сертификат для " + spec.Domain,
		do: func(ctx context.Context, r *Run) error {
			r.note("ждём ответа Let's Encrypt")
			err := waitSite(ctx, r, spec, certWait)
			if !errors.Is(err, errSiteDown) {
				return err
			}
			_, _ = r.sh(ctx, logs, shortTimeout)
			return &StepError{
				Message: "Сертификат не выдали",
				Advice:  "Let's Encrypt не смог зайти на " + spec.Domain + " по порту 80, или сайт не открывается снаружи. Чаще всего порты 80 и 443 закрыты в панели хостинга (брандмауэр). Откройте входящие порты 80 и 443 и нажмите «Повторить».",
			}
		},
	}
}

func linkStep(spec Spec) step {
	return step{
		id:    "check",
		title: "Проверка",
		do: func(ctx context.Context, r *Run) error {
			site := "https://" + spec.Domain
			body, err := r.probe(ctx, site+"/api/v1/instance")
			if err != nil {
				if errors.Is(err, ErrUnreachable) {
					return err
				}
				r.record("проверка "+site+"/api/v1/instance", err.Error())
				return &StepError{
					Message: "Сайт открылся, но Wynd не ответил",
					Advice:  "Нажмите «Повторить» — возможно, Wynd ещё запускается.",
				}
			}
			var instance struct {
				Version      string `json:"version"`
				Bootstrapped bool   `json:"bootstrapped"`
			}
			if err := json.Unmarshal([]byte(body), &instance); err != nil {
				return &StepError{
					Message: "По адресу " + spec.Domain + " отвечает не Wynd",
					Advice:  "Проверьте, что адрес указывает на этот сервер, и нажмите «Повторить».",
				}
			}
			r.record("проверка "+site+"/api/v1/instance", "Wynd "+instance.Version)
			if instance.Bootstrapped {
				// Сервер уже настроен: ссылки первого запуска у него нет.
				return nil
			}
			// Ссылку в журнал не кладём: по ней становятся администратором.
			res, err := r.sess.Exec(ctx, "cd "+installDir+" && docker compose logs --no-color wynd 2>&1 | grep -o 'bootstrap URL: [^ ]*' | tail -n 1", nil, shortTimeout)
			if err != nil {
				return err
			}
			url, ok := strings.CutPrefix(strings.TrimSpace(res.Output), "bootstrap URL: ")
			if !ok || !strings.HasPrefix(url, site+"/") {
				return &StepError{
					Message: "Wynd работает, но не назвал ссылку первого запуска",
					Advice:  "Нажмите «Повторить».",
				}
			}
			r.record("ссылка первого запуска", "получена — она на следующем экране")
			r.mu.Lock()
			r.link = url
			r.mu.Unlock()
			return nil
		},
	}
}

// certbotImage — им отзываем сертификат: у Caddy своей команды отзыва нет.
const certbotImage = "certbot/certbot"

// Метки в выводе отката: что вышло с отзывом сертификата.
const (
	markRevoked      = "WYND_REVOKED"
	markRevokeFailed = "WYND_REVOKE_FAILED"
)

// revokeScript отзывает сертификаты из тома нашего Caddy, пока том ещё есть.
// Отзыв подписывается ключом самого сертификата — учётная запись у
// удостоверяющего центра не нужна. Центр узнаём по имени папки, в которую
// Caddy кладёт сертификат; незнакомый — отзыв не вышел. Неудача откат не
// останавливает: сертификат всё равно уйдёт вместе с ключом. Отозванный раньше
// — тоже отозван. Запрос подписан ключом сертификата, поэтому Let's Encrypt
// сам ставит причину «ключ раскрыт» вместо названной и больше не выдаёт
// сертификаты на этот ключ (проба 2026-10-10) — ключ всё равно удаляется.
const revokeScript = `  proj=$(docker compose config 2>/dev/null | sed -n 's/^name: //p' | head -n 1)
  vol=$(docker volume ls -q --filter label=com.docker.compose.project="${proj:-wynd}" --filter label=com.docker.compose.volume=caddy-data 2>/dev/null | head -n 1)
  if [ -n "$vol" ]; then
    docker run --rm -v "$vol":/data:ro --entrypoint sh ` + certbotImage + ` -c '` + revokeLoop + `' 2>&1 || echo "` + markRevokeFailed + ` certbot did not run"
    docker image rm ` + certbotImage + ` >/dev/null 2>&1 || true
  fi`

// revokeLoop идёт внутри контейнера certbot: отзывает сертификаты из хранилища
// Caddy, подключённого в /data/caddy. DOM задан — только этого адреса (в
// хранилище чужого Caddy лежат и чужие сертификаты).
const revokeLoop = `
      for crt in /data/caddy/certificates/*/*/*.crt; do
        [ -f "$crt" ] || continue
        if [ -n "$DOM" ] && [ "$(basename "$(dirname "$crt")")" != "$DOM" ]; then continue; fi
        ca=$(basename "$(dirname "$(dirname "$crt")")")
        case "$ca" in
          acme-v02.api.letsencrypt.org-directory) srv=https://acme-v02.api.letsencrypt.org/directory ;;
          acme-staging-v02.api.letsencrypt.org-directory) srv=https://acme-staging-v02.api.letsencrypt.org/directory ;;
          acme.zerossl.com-v2-dv90) srv=https://acme.zerossl.com/v2/DV90 ;;
          *) echo "` + markRevokeFailed + ` $crt: unknown CA $ca"; continue ;;
        esac
        out=$(certbot revoke --non-interactive --no-delete-after-revoke --reason cessationofoperation --server "$srv" --cert-path "$crt" --key-path "${crt%.crt}.key" --config-dir /tmp/c --work-dir /tmp/w --logs-dir /tmp/l 2>&1) && ok=1 || ok=
        echo "$out"
        case "$out" in *"already revoked"*) ok=1 ;; esac
        if [ -n "$ok" ]; then
          echo "` + markRevoked + ` $crt"
        else
          echo "` + markRevokeFailed + ` $crt"
        fi
      done`

// Rollback убирает поставленное: контейнеры, папку /opt/wynd, образ Wynd и
// образ Caddy (тот — только если им не пользуется чужой контейнер: docker
// занятый образ не удалит). Образы берутся и из самих контейнеров: Wynd мог
// быть поставлен другим способом или другой версии, чем ставит это окно.
// Данные (тома) — только если keepData ложно. Docker остаётся: он мог
// понадобиться чему-то ещё, и сам по себе ничего не меняет.
//
// Данные удаляют — сертификат сначала отзываем (решение владельца,
// 2026-10-10): ключ уходит вместе с томом, а отозванный сертификат перестаёт
// действовать сразу, не дожидаясь срока.
func Rollback(ctx context.Context, sess *Session, spec Spec, keepData bool) ([]LogEntry, error) {
	down := "docker compose down --remove-orphans"
	if !keepData {
		down += " --volumes"
	}
	revoke := ""
	if !keepData {
		revoke = revokeScript
		// Тома берём по метке, а не только из нынешнего compose.yaml: Wynd могли
		// раньше ставить иначе (свой Caddy, потом чужой веб-сервер), и том с
		// отозванным сертификатом остался бы на сервере.
		down += ` 2>&1
  docker volume ls -q --filter label=com.docker.compose.project="${proj:-wynd}" 2>/dev/null | xargs -r docker volume rm`
	}
	script := rollbackProxy(keepData) + `imgs=
if [ -f ` + installDir + `/compose.yaml ] && command -v docker >/dev/null 2>&1; then
  cd ` + installDir + `
  imgs=$(docker compose images -q 2>/dev/null || true)
` + revoke + `
  ` + down + ` 2>&1
fi
cd /
rm -rf ` + installDir + `
if command -v docker >/dev/null 2>&1; then
  for i in $imgs; do docker image rm "$i" >/dev/null 2>&1 || true; done
  docker image rm ` + spec.imageTag() + ` >/dev/null 2>&1 || true
  docker image rm ` + caddyImage + ` >/dev/null 2>&1 || true
  rm -rf ` + sourceDir + `
  ` + swapOff + `
  rm -f ` + buildLock + `
  ` + pruneCacheOurs + `
fi
test ! -e ` + installDir
	res, err := sess.Exec(ctx, script, nil, packageTimeout)
	log := []LogEntry{{Command: script, Output: strings.TrimRight(res.Output, "\n")}}
	if err != nil {
		return log, err
	}
	if res.Code != 0 {
		return log, &commandError{command: script, output: res.Output, code: res.Code}
	}
	return log, nil
}

// Plan — что установщик сделает, до первого изменения (окно 3).
type Plan struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Advice  string `json:"advice,omitempty"`

	Domain  string `json:"domain"`
	Version string `json:"version"`
	// Install — что появится; Change — что изменим с резервной копией;
	// Keep — что найдено и не будет тронуто.
	Install  []string `json:"install"`
	Change   []string `json:"change"`
	Keep     []string `json:"keep"`
	Duration string   `json:"duration"`
	// Build — Wynd собирается на сервере из открытого кода, а не скачивается
	// готовым; CanBuild — такой способ у установщика есть.
	Build    bool `json:"build"`
	CanBuild bool `json:"canBuild"`
	// Unpublished — готового Wynd этой версии нет, выбирать не из чего:
	// сервер соберёт сам.
	Unpublished bool `json:"unpublished"`
	// FixClock и Proxy уходят в Spec.
	FixClock bool   `json:"-"`
	Proxy    string `json:"-"`
}

func isASCIIDomain(d string) bool {
	for _, r := range d {
		if r > 127 {
			return false
		}
	}
	return d != ""
}

// BuildPlan составляет план по осмотру. build — собрать Wynd на сервере из
// открытого кода вместо готового образа.
func BuildPlan(rep Report, domain, version string, build bool) Plan {
	p := Plan{Domain: domain, Version: version, Build: build, Duration: "обычно 5–10 минут, на медленном сервере — до получаса"}
	if build {
		p.Duration = "около 15 минут"
	}
	level := map[string]Level{}
	for _, f := range rep.Findings {
		level[f.ID] = f.Level
	}
	switch {
	case rep.Blocked():
		p.Message = "Сначала нужно убрать помеху, которую нашёл осмотр"
		return p
	case !isASCIIDomain(domain):
		p.Message = "Адрес с нелатинскими буквами установщик пока не умеет"
		p.Advice = "Возьмите адрес латиницей, например family.example.ru."
		return p
	}
	p.OK = true
	p.Proxy = rep.Proxy
	if level["docker"] != LevelOK {
		p.Install = append(p.Install, "Docker — он запускает контейнеры")
	}
	if build && !rep.Git {
		p.Install = append(p.Install, "git — им сервер скачает исходный код Wynd")
	}
	switch _, has := level["wynd"]; {
	case build:
		p.Install = append(p.Install, "Контейнер Wynd "+version+" — сервер соберёт из исходного кода")
	case has:
		p.Install = append(p.Install, "Контейнер Wynd "+version+" — доделаем то, чего на сервере не хватает")
	default:
		p.Install = append(p.Install, "Контейнер Wynd "+version)
	}
	switch rep.Proxy {
	case "":
		p.Install = append(p.Install, "Контейнер с веб-сервером Caddy")
		p.Install = append(p.Install, "Сертификат для "+domain)
	case ProxyCaddy:
		p.Install = append(p.Install, "Сайт "+domain+" в Caddy — отдельным файлом")
		p.Install = append(p.Install, "Сертификат для "+domain+" — его получит Caddy")
	default:
		p.Install = append(p.Install, "Сайт "+domain+" в "+proxyNames[rep.Proxy]+" — отдельным файлом")
		if len(rep.ApacheMods) > 0 {
			p.Change = append(p.Change, "В Apache включим модули: "+strings.Join(rep.ApacheMods, ", ")+" — без них сайт Wynd не заработает")
		}
		if !rep.Certbot {
			p.Install = append(p.Install, "certbot — он получает сертификаты")
		}
		p.Install = append(p.Install, "Сертификат для "+domain)
	}
	if level["clock"] == LevelNote {
		p.FixClock = true
		p.Install = append(p.Install, "Синхронизацию часов")
	}
	if rep.Proxy != "" {
		name := proxyOf(rep.Proxy).name
		p.Keep = append(p.Keep, name+" и его сайты: чужие файлы не правим, "+name+" перечитает настройку без остановки")
	}
	p.Keep = append(p.Keep, "Всё, что на сервере уже есть: Wynd займёт свою папку /opt/wynd")
	return p
}
