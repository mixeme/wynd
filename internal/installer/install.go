package installer

import (
	"context"
	"crypto/sha256"
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
	return append(list, imageStep(spec), filesStep(spec), startStep(), certStep(spec), linkStep(spec))
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
		f.Message, f.Advice = explained.Message, explained.Advice
	case errors.Is(err, ErrTimeout):
		f.Message = fmt.Sprintf("Шаг «%s» идёт дольше, чем мы ждали", s.title)
		f.Advice = "Сервер, скорее всего, ещё занят этим шагом — он просто медленный. Подождите несколько минут и нажмите «Повторить»: сделанное не пропадёт."
	case errors.Is(err, ErrUnreachable):
		f.Message = "Связь с сервером оборвалась"
		f.Advice = "Сделанное раньше осталось на месте. Нажмите «Повторить» — подключимся снова и продолжим с этого шага."
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
		f.Advice = "Сделанное раньше осталось на месте. Нажмите «Повторить»; если не поможет — в «подробностях» видно, на чём остановились."
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

// Provide скачивает образ на сервер.
func (g RegistryImage) Provide(ctx context.Context, r *Run, tag string) error {
	r.note("сервер скачивает Wynd")
	out, err := r.must(ctx, "docker pull -q "+tag+" 2>&1", imageTimeout)
	if err != nil && containsAny(out, "manifest unknown", "not found", "denied", "unauthorized") {
		return &StepError{
			Message: "Этой версии Wynd в хранилище ещё нет",
			Advice:  "Установщик ищет " + tag + ", а его там нет: версия ещё не выложена или хранилище закрыто. Скачайте свежий установщик или попробуйте позже.",
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

// buildSwap — временный файл подкачки на время сборки.
const buildSwap = "/var/tmp/wynd-build.swap"

// memoryKB — память сервера вместе с подкачкой, в килобайтах.
const memoryKB = `awk '/^(MemTotal|SwapTotal):/{s+=$2} END{print s}' /proc/meminfo`

const swapOff = "swapoff " + buildSwap + " 2>/dev/null; rm -f " + buildSwap

// swapOn заводит файл подкачки на столько мегабайт. fallocate умеет не
// всякая файловая система — тогда пишем нули.
func swapOn(mb uint64) string {
	size := fmt.Sprint(mb)
	return swapOff + `
set -e
fallocate -l ` + size + `M ` + buildSwap + ` 2>/dev/null || dd if=/dev/zero of=` + buildSwap + ` bs=1M count=` + size + ` status=none
chmod 600 ` + buildSwap + `
mkswap -q ` + buildSwap + `
swapon ` + buildSwap
}

// Provide клонирует выпуск и собирает образ. Исходники после сборки убираем;
// сборочный кэш Docker остаётся — с ним следующая сборка не качает заново
// базовые образы и зависимости. Откат его тоже не трогает: кэш у Docker
// общий, чужое от своего в нём не отличить (docker builder prune — руками).
func (s SourceImage) Provide(ctx context.Context, r *Run, tag string) error {
	_, version, _ := strings.Cut(tag, ":")
	ref := "v" + version
	if _, err := r.must(ctx, gitScript, packageTimeout); err != nil {
		return err
	}
	r.note("сервер скачивает исходный код Wynd")
	clone := "rm -rf " + sourceDir + " && git clone -q --depth 1 --branch " + shQuote(ref) + " " + shQuote(s.Repo) + " " + sourceDir + " 2>&1"
	if out, err := r.must(ctx, clone, shortTimeout); err != nil {
		if containsAny(out, "not found in upstream", "Remote branch", "Repository not found", "returned error: 404") {
			return &StepError{
				Message: "Этой версии Wynd в открытом коде ещё нет",
				Advice:  "Установщик ищет метку " + ref + " в " + s.Repo + ", а её там нет. Скачайте свежий установщик или попробуйте позже.",
			}
		}
		return err
	}
	// Памяти мало — на время сборки добавляем подкачку; место под неё и под
	// сборочный кэш проверяем заранее: отказ лучше зависшего сервера.
	var swapMB uint64
	if out, err := r.must(ctx, memoryKB, shortTimeout); err == nil {
		var kb uint64
		if _, err := fmt.Sscan(strings.TrimSpace(out), &kb); err == nil && kb < buildMemoryKB {
			swapMB = (buildMemoryKB-kb)>>10 + 1
		}
	}
	if out, err := r.must(ctx, "df -Pk / | tail -n 1", shortTimeout); err == nil {
		need := uint64(minFreeForBuild) + swapMB<<20
		if free, ok := freeBytes(out); ok && free < need {
			_, _ = r.sh(ctx, "rm -rf "+sourceDir, shortTimeout)
			return &StepError{
				Message: "На сервере мало места, чтобы собрать Wynd",
				Advice:  "Для сборки из исходного кода нужно " + formatGB(need) + " свободного места, а есть " + formatGB(free) + ". Поставьте готовый Wynd из хранилища или увеличьте диск в панели хостинга.",
			}
		}
	}
	if swapMB > 0 {
		r.note("серверу мало памяти для сборки — добавляем временную подкачку")
		if _, err := r.must(ctx, swapOn(swapMB), shortTimeout); err != nil {
			_, _ = r.sh(ctx, swapOff+"; rm -rf "+sourceDir, shortTimeout)
			return err
		}
	}
	r.note("сервер собирает Wynd из исходного кода — это долго")
	build := "cd " + sourceDir + " && docker build -q -f deploy/docker/Dockerfile --build-arg VERSION=" + shQuote(version) + " -t " + tag + " . 2>&1"
	out, err := r.must(ctx, build, buildTimeout)
	// Исходники больше не нужны, чем бы сборка ни кончилась.
	_, _ = r.sh(ctx, "rm -rf "+sourceDir, shortTimeout)
	if swapMB > 0 {
		_, _ = r.sh(ctx, swapOff, shortTimeout)
	}
	if err != nil && containsAny(out, "heap out of memory", "exit code: 137", "signal: killed", "Killed", "cannot allocate memory", "out of memory") {
		return &StepError{
			Message: "Серверу не хватило памяти, чтобы собрать Wynd",
			Advice:  "Сборка из исходного кода требует больше памяти, чем работа Wynd. Поставьте готовый Wynd из хранилища или возьмите сервер с большей памятью.",
		}
	}
	return err
}

// ComposeFile — /opt/wynd/compose.yaml: то же, что deploy/docker/compose.yaml,
// но с готовым образом вместо сборки из исходников.
func ComposeFile(spec Spec) string {
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
	files := []struct{ path, content string }{
		{installDir + "/compose.yaml", ComposeFile(spec)},
		{installDir + "/Caddyfile", CaddyFile(spec)},
	}
	return step{
		id:    "files",
		title: "Настройка",
		done: func(ctx context.Context, r *Run) (bool, error) {
			res, err := r.sess.Exec(ctx, "sha256sum "+files[0].path+" "+files[1].path+" 2>/dev/null", nil, shortTimeout)
			if err != nil {
				return false, err
			}
			return strings.Contains(res.Output, sum(files[0].content)) && strings.Contains(res.Output, sum(files[1].content)), nil
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

func startStep() step {
	compose := "cd " + installDir + " && docker compose "
	return step{
		id:    "start",
		title: "Запуск",
		do: func(ctx context.Context, r *Run) error {
			r.note("скачиваем веб-сервер и запускаем")
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
			if containsAny(output, "address already in use", "port is already allocated") {
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
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	res, err := http.DefaultClient.Do(req)
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
	probe := r.spec.Probe
	if probe == nil {
		probe = httpProbe
	}
	body, err := probe(ctx, url)
	if err == nil {
		return body, nil
	}
	res, execErr := r.sess.Exec(ctx, "curl -fsS --max-time 20 "+shQuote(url), nil, shortTimeout)
	if execErr != nil {
		return "", execErr
	}
	if res.Code != 0 {
		return "", fmt.Errorf("с этого компьютера: %v; с сервера: %s", err, strings.TrimSpace(res.Output))
	}
	return res.Output, nil
}

func certStep(spec Spec) step {
	return step{
		id:    "cert",
		title: "Сертификат для " + spec.Domain,
		do: func(ctx context.Context, r *Run) error {
			r.note("ждём ответа Let's Encrypt")
			url := "https://" + spec.Domain + "/health"
			deadline := time.Now().Add(certWait)
			var last error
			for {
				_, err := r.probe(ctx, url)
				if err == nil {
					r.record("проверка "+url, "сайт открывается, сертификат действует")
					return nil
				}
				if errors.Is(err, ErrUnreachable) {
					return err
				}
				last = err
				if time.Now().After(deadline) {
					break
				}
				if err := sleep(ctx, pollInterval); err != nil {
					return err
				}
			}
			r.record("проверка "+url, last.Error())
			_, _ = r.sh(ctx, "cd "+installDir+" && docker compose logs --no-color --tail 30 caddy 2>&1", shortTimeout)
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

// Rollback убирает поставленное: контейнеры, папку /opt/wynd, образ Wynd и
// образ Caddy (тот — только если им не пользуется чужой контейнер: docker
// занятый образ не удалит).
// Данные (тома) — только если keepData ложно. Docker остаётся: он мог
// понадобиться чему-то ещё, и сам по себе ничего не меняет.
func Rollback(ctx context.Context, sess *Session, spec Spec, keepData bool) ([]LogEntry, error) {
	down := "docker compose down --remove-orphans"
	if !keepData {
		down += " --volumes"
	}
	script := `if [ -f ` + installDir + `/compose.yaml ] && command -v docker >/dev/null 2>&1; then
  cd ` + installDir + ` && ` + down + ` 2>&1
fi
cd /
rm -rf ` + installDir + `
if command -v docker >/dev/null 2>&1; then
  docker image rm ` + spec.imageTag() + ` >/dev/null 2>&1 || true
  docker image rm ` + caddyImage + ` >/dev/null 2>&1 || true
  rm -rf ` + sourceDir + `
  ` + swapOff + `
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
	// FixClock уходит в Spec.
	FixClock bool `json:"-"`
}

func isASCIIDomain(d string) bool {
	for _, r := range d {
		if r > 127 {
			return false
		}
	}
	return d != ""
}

// BuildPlan составляет план по осмотру. Сервер с чужим веб-сервером — пока
// отказ с причиной: это следующий срез.
func BuildPlan(rep Report, domain, version string) Plan {
	p := Plan{Domain: domain, Version: version, Duration: "обычно 5–10 минут, на медленном сервере — до получаса"}
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
	case rep.Proxy != "":
		p.Message = "На сервере уже работает " + rep.Proxy
		p.Advice = "Установка рядом с чужим веб-сервером появится в следующей версии установщика. Пока Wynd ставится на сервер, где порты 80 и 443 свободны."
		return p
	}
	p.OK = true
	if level["docker"] != LevelOK {
		p.Install = append(p.Install, "Docker — в нём работает Wynd")
	}
	if _, has := level["wynd"]; has {
		p.Install = append(p.Install, "Wynd "+version+" — доделаем то, чего на сервере не хватает")
	} else {
		p.Install = append(p.Install, "Wynd "+version)
	}
	p.Install = append(p.Install, "Веб-сервер Caddy и сертификат для "+domain)
	if level["clock"] == LevelNote {
		p.FixClock = true
		p.Install = append(p.Install, "Синхронизацию часов")
	}
	p.Keep = []string{"Всё, что на сервере уже есть: Wynd займёт свою папку /opt/wynd"}
	return p
}
