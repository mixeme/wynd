package installer

import (
	"context"
	"encoding/json"
	"path"
	"strings"
)

// Wynd, поставленный не Мастерской и не так, как ставит она (кадр 7з; край
// переноса — владелец, 2026-10-25): не в /opt/wynd или не в Docker — служба
// systemd, контейнер из другой папки. Мастерская такой Wynd только находит —
// по работающему процессу, контейнеру (и остановленному) или файлу службы —
// и перевозит: папку данных и адрес сайта (путь 9б2). Обновлять, делать бэкап
// и убирать его она не берётся.

// ForeignWynd — найденный Wynd не на месте Мастерской.
type ForeignWynd struct {
	// Kind — как запущен: «service» (служба systemd), «container», «process».
	Kind string `json:"kind"`
	// Name — имя службы, папка контейнера (или его имя) или путь программы.
	Name    string `json:"name"`
	DataDir string `json:"data_dir"`
	URL     string `json:"url,omitempty"`
	Running bool   `json:"running"`
	Version string `json:"version,omitempty"`

	listen string
	free   uint64
	move   *MoveMark
	// stop и start — как остановить и запустить его на время переезда;
	// пусто — остановить Мастерская не может (процесс без службы).
	stop, start string
}

// foreignScript ищет Wynd не на месте Мастерской. Только читает: процессы
// (кроме тех, что в контейнерах), контейнеры с образом Wynd не из /opt/wynd,
// файлы служб systemd, которые запускают wynd.
const foreignScript = `for pid in $(pgrep -x wynd 2>/dev/null); do
  grep -qsE 'docker|containerd|kubepods|lxc' /proc/$pid/cgroup && continue
  e() { tr '\0' '\n' < /proc/$pid/environ 2>/dev/null | sed -n "s/^$1=//p" | head -n 1; }
  echo "@proc|$(ps -o unit= -p $pid 2>/dev/null | tr -d ' ')|$(readlink /proc/$pid/cwd)|$(e WYND_DATA_DIR)|$(e WYND_PUBLIC_URL)|$(e WYND_LISTEN)|$(readlink /proc/$pid/exe)"
done
docker ps -a --no-trunc --format '{{.ID}}|{{.Image}}|{{.State}}|{{.Label "com.docker.compose.project.working_dir"}}|{{.Label "com.docker.compose.service"}}|{{.Names}}' 2>/dev/null | while IFS='|' read -r id img st dir svc name; do
  case "$img" in *wynd*) ;; *) continue ;; esac
  [ "$dir" = /opt/wynd ] && continue
  echo "@ctr|$id|$st|$dir|$svc|$name|$(docker inspect -f '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Source}}{{end}}{{end}}' "$id")|$(docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$id" | sed -n 's/^WYND_PUBLIC_URL=//p' | head -n 1)|$img"
done
for f in /etc/systemd/system/*.service; do
  grep -qsE '^ExecStart=.*wynd' "$f" || continue
  u=$(basename "$f")
  echo "@unit|$u|$(systemctl is-active "$u" 2>/dev/null)|$(sed -n 's/^WorkingDirectory=//p' "$f" | head -n 1)|$(sed -n 's/^Environment=.*WYND_DATA_DIR=\([^ "]*\).*/\1/p' "$f" | head -n 1)|$(sed -n 's/^Environment=.*WYND_PUBLIC_URL=\([^ "]*\).*/\1/p' "$f" | head -n 1)"
done`

// parseForeign выбирает Wynd из ответа foreignScript: работающий процесс,
// потом контейнер, потом служба. Папку данных и адрес уточняет followUp.
func parseForeign(out string) (ForeignWynd, bool) {
	var procs, ctrs, units [][]string
	for _, line := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		f := strings.Split(strings.TrimSpace(line), "|")
		switch f[0] {
		case "@proc":
			if len(f) >= 7 {
				procs = append(procs, f)
			}
		case "@ctr":
			if len(f) >= 9 {
				ctrs = append(ctrs, f)
			}
		case "@unit":
			if len(f) >= 6 {
				units = append(units, f)
			}
		}
	}
	switch {
	case len(procs) > 0:
		p := procs[0]
		w := ForeignWynd{Running: true, DataDir: dataDirOf(p[2], p[3]), URL: p[4], listen: p[5]}
		if p[1] != "" && strings.HasSuffix(p[1], ".service") {
			w.Kind, w.Name = "service", strings.TrimSuffix(p[1], ".service")
			w.stop, w.start = "systemctl stop "+shQuote(p[1]), "systemctl start "+shQuote(p[1])
		} else {
			w.Kind, w.Name = "process", p[6]
		}
		return w, true
	case len(ctrs) > 0:
		c := ctrs[0]
		w := ForeignWynd{Kind: "container", Running: c[2] == "running", DataDir: c[6], URL: c[7]}
		if i := strings.LastIndex(c[8], ":"); i >= 0 {
			w.Version = c[8][i+1:]
		}
		if c[3] != "" && c[4] != "" {
			w.Name = c[3]
			cd := "cd " + shQuote(c[3]) + " && docker compose "
			w.stop, w.start = cd+"stop "+shQuote(c[4])+" 2>&1", cd+"up -d "+shQuote(c[4])+" 2>&1"
		} else {
			w.Name = c[5]
			w.stop, w.start = "docker stop "+shQuote(c[1])+" 2>&1", "docker start "+shQuote(c[1])+" 2>&1"
		}
		return w, true
	case len(units) > 0:
		u := units[0]
		return ForeignWynd{Kind: "service", Name: strings.TrimSuffix(u[1], ".service"), Running: u[2] == "active",
			DataDir: dataDirOf(u[3], u[4]), URL: u[5],
			stop: "systemctl stop " + shQuote(u[1]), start: "systemctl start " + shQuote(u[1])}, true
	}
	return ForeignWynd{}, false
}

// dataDirOf — папка данных, как её найдёт сам Wynd: WYND_DATA_DIR, а без неё
// dev/data в рабочей папке (config.DefaultDataDir); относительная — от неё же.
func dataDirOf(cwd, env string) string {
	if env == "" {
		env = "dev/data"
	}
	if path.IsAbs(env) {
		return path.Clean(env)
	}
	if cwd == "" {
		cwd = "/"
	}
	return path.Join(cwd, env)
}

// DetectForeign ищет Wynd не на месте Мастерской. Только читает. Найден —
// если в его папке данных есть база: без неё переносить нечего.
func DetectForeign(ctx context.Context, sess *Session) (ForeignWynd, bool) {
	res, err := sess.Exec(ctx, foreignScript, nil, shortTimeout)
	if err != nil {
		return ForeignWynd{}, false
	}
	w, ok := parseForeign(res.Output)
	if !ok || w.DataDir == "" {
		return ForeignWynd{}, false
	}
	d := shQuote(w.DataDir)
	more := `test -s ` + d + `/wynd.db && echo @db
sed -n 's/.*"public_url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/@url \1/p' ` + d + `/config.json 2>/dev/null | head -n 1
sed -n 's/.*"listen"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/@listen \1/p' ` + d + `/config.json 2>/dev/null | head -n 1
echo "@free $(df -Pk ` + d + ` | tail -n 1)"
echo "@move $(cat ` + d + `/.workshop-move 2>/dev/null)"`
	res, err = sess.Exec(ctx, more, nil, shortTimeout)
	if err != nil || !strings.Contains(res.Output, "@db") {
		return ForeignWynd{}, false
	}
	for _, line := range strings.Split(strings.ReplaceAll(res.Output, "\r\n", "\n"), "\n") {
		key, val, _ := strings.Cut(strings.TrimSpace(line), " ")
		switch key {
		case "@url":
			if w.URL == "" {
				w.URL = val
			}
		case "@listen":
			if w.listen == "" {
				w.listen = val
			}
		case "@free":
			w.free, _ = freeBytes(val)
		case "@move":
			var m MoveMark
			if val != "" && json.Unmarshal([]byte(val), &m) == nil && m.To != "" {
				w.move = &m
			}
		}
	}
	if w.Running && w.Version == "" && w.Kind != "container" {
		listen := w.listen
		if listen == "" {
			listen = "127.0.0.1:7676"
		}
		if strings.HasPrefix(listen, ":") {
			listen = "127.0.0.1" + listen
		}
		if r, err := sess.Exec(ctx, "curl -s --max-time 5 "+shQuote("http://"+listen+"/api/v1/instance")+" | sed -n 's/.*\"version\":\"\\([^\"]*\\)\".*/\\1/p'", nil, shortTimeout); err == nil {
			w.Version = strings.TrimSpace(r.Output)
		}
	}
	return w, true
}

// markPath — пометка переезда этого Wynd: в его папке данных (своей папки
// установки у него нет).
func (w ForeignWynd) markPath() string { return w.DataDir + "/.workshop-move" }

// sendTar — архив его папки данных под именем data: так её ждёт установка
// Мастерской на новом сервере.
func (w ForeignWynd) sendTar() string {
	// Имя чужой папки не должно становиться регулярным выражением tar:
	// запятая ломала transform, а [ и . меняли совпадение (аудит 2026-10-29).
	return "tar -C " + shQuote(w.DataDir) + " --exclude='./backups' --exclude='./.workshop-*'" +
		" --transform 's,^\\.,data,' -cf - ."
}

// Domain — адрес сайта без https:// и косой черты.
func (w ForeignWynd) Domain() string {
	return strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(w.URL, "https://"), "http://"), "/")
}
