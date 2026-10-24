package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Пульт сервера (план 45, срез 4; кадры 7а–7ж2): что стоит на сервере, где
// Wynd уже работает; запуск, остановка, перезапуск; обновление с копией
// данных на сервере и возвратом прежней версии, если новая не запустилась.
//
// Мастерская кладёт рядом с compose.yaml две пометки: .workshop-stopped —
// сайт остановлен ею (кнопкой или бэкапом «остановить и не запускать»), и
// .workshop-update — обновление начато и не закончено (кадр 7е).

const (
	stoppedMark = installDir + "/.workshop-stopped"
	updateMark  = installDir + "/.workshop-update"
	backupsDir  = installDir + "/backups"
	composePath = installDir + "/compose.yaml"
)

// ServerState — что Мастерская видит на сервере с Wynd (кадры 7а, 7а2, 7е).
type ServerState struct {
	Installed bool `json:"installed"`
	// Version — версия Wynd из строки образа в compose.yaml; Image — сама строка.
	Version string `json:"version,omitempty"`
	Image   string `json:"image,omitempty"`
	Domain  string `json:"domain,omitempty"`
	Running bool   `json:"running"`
	// Container — контейнер Wynd есть (запущен или остановлен). Нет — Wynd
	// убран с оставленными данными: это не пульт, а установка заново.
	Container bool `json:"container"`
	// StoppedAt — когда сайт остановила Мастерская; пусто — не она.
	StoppedAt string `json:"stopped_at,omitempty"`
	FreeBytes uint64 `json:"free_bytes"`
	// Update — обновление начато и не закончено (кадр 7е).
	Update *UpdateMark `json:"update,omitempty"`
}

// UpdateMark — пометка начатого обновления.
type UpdateMark struct {
	From string `json:"from"`
	To   string `json:"to"`
	// Ref — образ новой версии, как его впишет compose.yaml.
	Ref string `json:"ref"`
	// Stopped — Мастерская остановила сайт (кадр 7е: «не работает» из-за неё).
	Stopped bool `json:"stopped"`
	// StartAfter — запустить новую версию; нет — только поставить (кадр 7б3).
	StartAfter bool   `json:"start_after"`
	Started    string `json:"started"`
}

const stateScript = `cd ` + installDir + ` 2>/dev/null || { echo '@none'; exit 0; }
test -f compose.yaml || { echo '@none'; exit 0; }
echo "@image $(sed -n 's/^[[:space:]]*image:[[:space:]]*\([^[:space:]]*wynd:[^[:space:]]*\)[[:space:]]*$/\1/p' compose.yaml | head -n 1)"
echo "@url $(sed -n 's/^[[:space:]]*WYND_PUBLIC_URL:[[:space:]]*//p' compose.yaml | head -n 1)"
echo "@running $(` + wyndRunning + ` | head -n 1)"
echo "@container $(docker ps -aq --filter label=com.docker.compose.project.working_dir=` + installDir + ` --filter label=com.docker.compose.service=wynd 2>/dev/null | head -n 1)"
echo "@stopped $(cat ` + stoppedMark + ` 2>/dev/null)"
echo "@update $(cat ` + updateMark + ` 2>/dev/null)"
echo "@free $(df -Pk ` + installDir + ` | tail -n 1)"`

// ReadServerState смотрит, что стоит на сервере. Ничего не меняет.
func ReadServerState(ctx context.Context, sess *Session) (ServerState, error) {
	res, err := sess.Exec(ctx, stateScript, nil, shortTimeout)
	if err != nil {
		return ServerState{}, err
	}
	return parseServerState(res.Output), nil
}

func parseServerState(out string) ServerState {
	var st ServerState
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		key, val, _ := strings.Cut(line, " ")
		val = strings.TrimSpace(val)
		switch key {
		case "@none":
			return ServerState{}
		case "@image":
			st.Installed = true
			st.Image = val
			if i := strings.LastIndex(val, ":"); i >= 0 {
				st.Version = val[i+1:]
			}
		case "@url":
			st.Installed = true
			st.Domain = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(val, "https://"), "http://"), "/")
		case "@running":
			st.Running = val != ""
		case "@container":
			st.Container = val != ""
		case "@stopped":
			st.StoppedAt = val
		case "@update":
			if val != "" {
				var m UpdateMark
				if json.Unmarshal([]byte(val), &m) == nil && m.To != "" {
					st.Update = &m
				}
			}
		case "@free":
			st.FreeBytes, _ = freeBytes(val)
		}
	}
	if !st.Installed {
		return ServerState{}
	}
	return st
}

// Действия кнопок страницы сервера (кадры 7а, 7а2, 7а3).
const (
	ServiceStart   = "start"
	ServiceStop    = "stop"
	ServiceRestart = "restart"
)

const composeCmd = "cd " + installDir + " && docker compose "

// ServiceAction запускает, останавливает или перезапускает Wynd. Caddy не
// трогается: остановленный сайт отвечает «сервер недоступен», а не пропадает
// с адреса (кадр 7а3).
func ServiceAction(ctx context.Context, sess *Session, action string) ([]LogEntry, error) {
	r := &Run{sess: sess, steps: []StepState{{ID: action}}, report: func() {}}
	var cmd string
	switch action {
	case ServiceStart:
		cmd = composeCmd + "up -d --quiet-pull 2>&1 && rm -f " + stoppedMark
	case ServiceStop:
		cmd = composeCmd + "stop wynd 2>&1 && date -u +%Y-%m-%dT%H:%M:%SZ > " + stoppedMark
	case ServiceRestart:
		cmd = composeCmd + "restart wynd 2>&1"
	default:
		return nil, ErrInvalid
	}
	if _, err := r.must(ctx, cmd, packageTimeout); err != nil {
		return r.log, err
	}
	if action != ServiceStop {
		if err := waitHealthy(ctx, r); err != nil {
			return r.log, err
		}
	}
	return r.log, nil
}

// waitHealthy ждёт, пока Wynd ответит.
func waitHealthy(ctx context.Context, r *Run) error {
	healthy := composeCmd + `ps -q wynd | xargs -r docker inspect -f '{{.State.Health.Status}}' | grep -qx healthy`
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
			_, _ = r.sh(ctx, composeCmd+"ps; "+composeCmd+"logs --no-color --tail 20 wynd 2>&1 | grep -v 'bootstrap URL'", shortTimeout)
			return &StepError{
				Message: "Wynd запустился, но не отвечает",
				Advice:  "В «подробностях» видно, что пишет сам Wynd.",
			}
		}
		if err := sleep(ctx, pollInterval); err != nil {
			return err
		}
	}
}

// imageLineRe — строка образа Wynd в compose.yaml. Мастерская меняет в файле
// только её (решено: остальное в файле — правки человека, кадр 7ж).
var imageLineRe = regexp.MustCompile(`(?m)^([ \t]*image:[ \t]*)(\S*wynd:\S*)[ \t]*$`)

// SetImage меняет в compose.yaml образ Wynd на ref. ok=false — строки нет
// или она не одна: файл правили вручную, трогать его нельзя.
func SetImage(compose, ref string) (string, bool) {
	if len(imageLineRe.FindAllStringIndex(compose, -1)) != 1 {
		return compose, false
	}
	return imageLineRe.ReplaceAllString(compose, "${1}"+ref), true
}

// UpdateSpec — обновление Wynd на сервере (кадры 7б, 7б3, 7б4).
type UpdateSpec struct {
	From, To string
	Domain   string
	Image    ImageSource
	// Running — сайт работал до обновления.
	Running bool
	// StopForCopy — остановить сайт на время копии (галочка 7б); без неё копия
	// снимается с работающего сайта (`wynd backup -no-blobs`).
	StopForCopy bool
	// StartAfter — запустить новую версию (7б3: «Оставить сайт остановленным» — нет).
	StartAfter bool
	Probe      func(ctx context.Context, url string) (string, error)
}

// LiveCopyFrom — с какой версии Wynd умеет копию без файлов на работающем
// сайте (`wynd backup -no-blobs`).
var LiveCopyFrom = [3]int{0, 43, 2}

// CanCopyLive — умеет ли Wynd этой версии копию без остановки.
func CanCopyLive(version string) bool {
	n, ok := releaseNumber(version)
	return ok && !versionLess(n, LiveCopyFrom)
}

func backupFor(version string) string { return backupsDir + "/" + version }

// updateSteps — шаги обновления; пометка начатого обновления кладётся до
// первого изменения сайта и убирается последним шагом.
func updateSteps(spec UpdateSpec) []step {
	ref := spec.Image.Ref(spec.To)
	mark := UpdateMark{From: spec.From, To: spec.To, Ref: ref, StartAfter: spec.StartAfter,
		Stopped: spec.Running && spec.StartAfter, Started: time.Now().UTC().Format(time.RFC3339)}
	markJSON, _ := json.Marshal(mark)
	img := imageStep(Spec{Version: spec.To, Image: spec.Image})
	img.title = "Wynd " + spec.To
	list := []step{img}
	stopNow := spec.Running && spec.StopForCopy
	if stopNow {
		list = append(list, stopStep(markJSON))
	}
	list = append(list, copyStep(spec, markJSON, stopNow || !spec.Running, !stopNow))
	if spec.Running && !spec.StopForCopy {
		list = append(list, stopStep(nil))
	}
	return append(list, startNewStep(spec, ref), checkStep(spec))
}

func stopStep(mark []byte) step {
	return step{
		id:    "stop",
		title: "Остановка Wynd",
		do: func(ctx context.Context, r *Run) error {
			if mark != nil {
				if err := r.put(ctx, updateMark, string(mark)+"\n"); err != nil {
					return err
				}
			}
			_, err := r.must(ctx, composeCmd+"stop wynd 2>&1", shortTimeout)
			return err
		},
	}
}

// copyStep — копия данных перед обновлением (кадр 7б): база, настройки и
// ключи — копией, фото и видео — жёсткими ссылками на те же файлы: места не
// занимают и остаются, даже если новая версия удалит файл.
func copyStep(spec UpdateSpec, mark []byte, stopped, writeMark bool) step {
	dir := backupFor(spec.From)
	var db string
	if stopped {
		// Wynd стоит — база не меняется, файлы копируются как есть.
		db = `mkdir -p "$d.new" && cp -a data/wynd.db "$d.new/" && for f in wynd.db-wal wynd.db-shm config.json; do if [ -e data/$f ]; then cp -a data/$f "$d.new/"; fi; done && if [ -d data/keys ]; then cp -a data/keys "$d.new/"; fi`
	} else {
		// Wynd работает — снимок базы им самим, без файлов.
		db = `rm -rf data/.workshop-copy && ` + composeCmd + `exec -T wynd wynd backup -no-blobs /data/.workshop-copy </dev/null && mv data/.workshop-copy "$d.new"`
	}
	script := `set -e
cd ` + installDir + `
d=` + dir + `
mkdir -p ` + backupsDir + `
rm -rf "$d.new"
` + db + `
cp -al data/blobs "$d.new/blobs"
cp -a compose.yaml "$d.new/"
if [ -e Caddyfile ]; then cp -a Caddyfile "$d.new/"; fi
rm -rf "$d"
mv "$d.new" "$d"`
	return step{
		id:    "copy",
		title: "Копия данных",
		do: func(ctx context.Context, r *Run) error {
			if writeMark {
				if err := r.put(ctx, updateMark, string(mark)+"\n"); err != nil {
					return err
				}
			}
			need, err := r.must(ctx, "du -sk "+dataDir+"/wynd.db* | awk '{s+=$1} END {print s}'; df -Pk "+installDir+" | tail -n 1", shortTimeout)
			if err != nil {
				return err
			}
			lines := strings.Split(strings.TrimSpace(need), "\n")
			if len(lines) == 2 {
				kb, _ := strconv.ParseUint(strings.TrimSpace(lines[0]), 10, 64)
				if free, ok := freeBytes(lines[1]); ok && free < kb<<10+64<<20 {
					return &StepError{
						Message: "На сервере мало места для копии",
						Advice:  "Копии нужно " + formatGB(kb<<10) + ", свободно " + formatGB(free) + ". Освободите место на сервере и нажмите «Повторить».",
					}
				}
			}
			_, err = r.must(ctx, script, imageTimeout)
			return err
		},
	}
}

// startNewStep вписывает новую версию в compose.yaml (только строку образа,
// кадр 7ж) и запускает её; «оставить остановленным» (7б3) — только ставит
// контейнер, не запуская.
func startNewStep(spec UpdateSpec, ref string) step {
	title := "Запуск " + spec.To
	if !spec.StartAfter {
		title = "Wynd " + spec.To + " — не запуская"
	}
	return step{
		id:    "start",
		title: title,
		do: func(ctx context.Context, r *Run) error {
			cur, err := r.must(ctx, "cat "+composePath, shortTimeout)
			if err != nil {
				return err
			}
			next, ok := SetImage(cur, ref)
			if !ok {
				return &StepError{
					Message: "В " + composePath + " нет строки с версией Wynd",
					Advice:  "Файл меняли вручную. Поставьте у сервиса wynd строку «image: " + ref + "» и нажмите «Повторить».",
				}
			}
			if next != cur {
				if err := r.put(ctx, composePath, next); err != nil {
					return err
				}
			}
			if !spec.StartAfter {
				_, err := r.must(ctx, composeCmd+"up -d --no-start wynd 2>&1", shortTimeout)
				return err
			}
			_, err = r.must(ctx, composeCmd+"up -d wynd 2>&1 && rm -f "+stoppedMark, packageTimeout)
			return err
		},
		verify: func(ctx context.Context, r *Run) error {
			if !spec.StartAfter {
				return nil
			}
			if err := waitHealthy(ctx, r); err != nil {
				return &StepError{
					Message: "Wynd " + spec.To + " не запустился",
					Advice:  "Сайт сейчас не открывается. Данные целы: копия, сделанная перед обновлением, на месте.",
				}
			}
			return nil
		},
	}
}

// checkStep — сайт открывается новой версией; пометка начатого обновления
// снимается только после этого.
func checkStep(spec UpdateSpec) step {
	return step{
		id:    "check",
		title: "Проверка",
		do: func(ctx context.Context, r *Run) error {
			if spec.StartAfter && spec.Probe != nil && spec.Domain != "" {
				body, err := spec.Probe(ctx, "https://"+spec.Domain+"/api/v1/instance")
				if err != nil || !strings.Contains(body, `"version":"`+spec.To+`"`) {
					return &StepError{
						Message: "Сайт " + spec.Domain + " не открывается с новой версией",
						Advice:  "Wynd " + spec.To + " запущен, но сайт отвечает не им. Данные целы: копия перед обновлением на месте.",
					}
				}
			}
			_, err := r.must(ctx, "rm -f "+updateMark, shortTimeout)
			return err
		},
	}
}

// Update обновляет Wynd. Повтор после отказа — с начала: сделанные шаги
// проходят быстро (образ уже есть, копия перезаписывается).
func Update(ctx context.Context, sess *Session, spec UpdateSpec, report func(Progress)) Progress {
	return runSteps(ctx, sess, Spec{Domain: spec.Domain, Version: spec.To, Image: spec.Image}, updateSteps(spec), report,
		func(_ *Run, p *Progress) { p.Site = "https://" + spec.Domain })
}

// UpdateTitles — шаги обновления для окна до начала.
func UpdateTitles(spec UpdateSpec) []StepState { return titles(updateSteps(spec)) }

// ReturnPrevious возвращает прежнюю версию с данными из копии (кадры 7д, 7е):
// compose.yaml и база — из копии, удалённые новой версией файлы — ссылками
// обратно. start — запускать ли (сайт работал до обновления).
func ReturnPrevious(ctx context.Context, sess *Session, from string, start bool) ([]LogEntry, error) {
	r := &Run{sess: sess, steps: []StepState{{ID: "return"}}, report: func() {}}
	d := backupFor(from)
	script := `set -e
cd ` + installDir + `
d=` + d + `
test -f "$d/wynd.db"
docker compose stop wynd 2>&1 || true
cp -a "$d/compose.yaml" compose.yaml
if [ -e "$d/Caddyfile" ]; then cp -a "$d/Caddyfile" Caddyfile; fi
rm -f data/wynd.db-wal data/wynd.db-shm
cp -a "$d/wynd.db" data/wynd.db
for f in wynd.db-wal wynd.db-shm config.json; do if [ -e "$d/$f" ]; then cp -a "$d/$f" data/; fi; done
if [ -d "$d/keys" ]; then rm -rf data/keys && cp -a "$d/keys" data/keys; fi
cp -aln "$d/blobs/." data/blobs/`
	if _, err := r.must(ctx, script, imageTimeout); err != nil {
		return r.log, err
	}
	if start {
		if _, err := r.must(ctx, composeCmd+"up -d wynd 2>&1", packageTimeout); err != nil {
			return r.log, err
		}
		if err := waitHealthy(ctx, r); err != nil {
			return r.log, err
		}
	}
	_, err := r.must(ctx, "rm -f "+updateMark, shortTimeout)
	return r.log, err
}

// CopySize — сколько займёт копия перед обновлением: база (фото и видео —
// ссылками, места не занимают). Для плана 7б и отказа 7б2.
func CopySize(ctx context.Context, sess *Session) (uint64, error) {
	res, err := sess.Exec(ctx, "du -sk "+dataDir+"/wynd.db* 2>/dev/null | awk '{s+=$1} END {print s+0}'", nil, shortTimeout)
	if err != nil {
		return 0, err
	}
	kb, err := strconv.ParseUint(strings.TrimSpace(res.Output), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("copy size: %q", res.Output)
	}
	return kb << 10, nil
}
