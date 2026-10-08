package installer

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Level — что находка значит для установки.
type Level string

const (
	// LevelOK — в порядке.
	LevelOK Level = "ok"
	// LevelNote — не так, как на чистом сервере, но установке не мешает:
	// поставим сами или обойдём.
	LevelNote Level = "note"
	// LevelBlock — ставить нельзя, пока человек это не поправит.
	LevelBlock Level = "block"
)

// Finding — строка осмотра словами человека. Text — что найдено; Plan — что
// установщик с этим сделает (у LevelNote); Advice — что сделать человеку (у
// LevelBlock).
type Finding struct {
	ID     string `json:"id"`
	Level  Level  `json:"level"`
	Text   string `json:"text"`
	Plan   string `json:"plan,omitempty"`
	Advice string `json:"advice,omitempty"`
}

// Report — итог осмотра. Сервер при осмотре не меняется.
type Report struct {
	Findings []Finding `json:"findings"`
	// Addresses — адреса IPv4 сервера: с ними сверяется домен.
	Addresses []string `json:"addresses"`
	// Proxy — чужой веб-сервер на 80/443 («nginx», «apache», «caddy») или пусто.
	Proxy string `json:"proxy,omitempty"`
	// Raw — вывод команд осмотра, для «подробнее, что проверили».
	Raw string `json:"raw"`
}

// Blocked сообщает, есть ли помеха, с которой ставить нельзя.
func (r Report) Blocked() bool {
	for _, f := range r.Findings {
		if f.Level == LevelBlock {
			return true
		}
	}
	return false
}

const (
	// Меньше — не ставим: образ, база и первые фотографии не поместятся.
	minFreeBytes = 5 << 30
	// Расхождение часов, с которого коды входа и сертификаты начинают
	// отказывать. Синхронизацию установщик настроит сам.
	maxClockSkew = 60 * time.Second
)

// inspectScript только читает. Каждая секция — от строки «## имя» до
// следующей; порядок и наличие секций разбор не предполагает. Команды, которых
// на сервере нет, молчат: пустая секция — тоже ответ.
const inspectScript = `export LC_ALL=C
echo '## os'; cat /etc/os-release 2>/dev/null
echo '## arch'; uname -m 2>/dev/null
echo '## uid'; id -u 2>/dev/null
echo '## sudo'; if command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then echo yes; fi
echo '## disk'; df -Pk / 2>/dev/null | tail -n 1
echo '## time'; date -u +%s 2>/dev/null
echo '## ntp'; timedatectl show -p NTPSynchronized --value 2>/dev/null
echo '## docker'; docker --version 2>/dev/null
echo '## compose'; docker compose version 2>/dev/null
echo '## ports'; ss -H -ltnp 2>/dev/null
echo '## addr'; ip -4 -o addr show scope global 2>/dev/null
echo '## wynd'; ls -d /opt/wynd/compose.yaml /etc/wynd 2>/dev/null
`

// Inspect осматривает сервер: одна команда, только чтение.
func Inspect(ctx context.Context, s *Session) (Report, error) {
	sent := time.Now()
	out, err := s.Run(ctx, inspectScript)
	if err != nil {
		return Report{}, err
	}
	// Часы сервера сверяем с серединой запроса: команда идёт доли секунды,
	// но на медленной связи — секунды.
	mid := sent.Add(time.Since(sent) / 2)
	return ParseInspection(out, s.User, mid), nil
}

func sections(out string) map[string][]string {
	res := map[string][]string{}
	name := ""
	for _, line := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		if rest, ok := strings.CutPrefix(line, "## "); ok {
			name = strings.TrimSpace(rest)
			res[name] = nil
			continue
		}
		if name != "" && strings.TrimSpace(line) != "" {
			res[name] = append(res[name], line)
		}
	}
	return res
}

func first(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[0])
}

// ParseInspection превращает вывод inspectScript в находки. now — время на
// этом компьютере в момент, когда сервер назвал своё.
func ParseInspection(out, user string, now time.Time) Report {
	sec := sections(out)
	rep := Report{Raw: out, Addresses: parseAddresses(sec["addr"])}
	add := func(f Finding) { rep.Findings = append(rep.Findings, f) }

	add(osFinding(sec["os"], first(sec["arch"])))
	if f, ok := rootFinding(user, first(sec["uid"]), first(sec["sudo"]) == "yes"); ok {
		add(f)
	}
	add(diskFinding(first(sec["disk"])))
	add(clockFinding(first(sec["time"]), first(sec["ntp"]), now))
	add(dockerFinding(first(sec["docker"]), first(sec["compose"])))
	proxy, f := portsFinding(sec["ports"])
	rep.Proxy = proxy
	add(f)
	if len(sec["wynd"]) > 0 {
		add(Finding{
			ID:    "wynd",
			Level: LevelNote,
			Text:  "Wynd на этом сервере уже стоит",
			Plan:  "ставить заново не будем — обновим",
		})
	}
	return rep
}

func osRelease(lines []string) map[string]string {
	res := map[string]string{}
	for _, line := range lines {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		res[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return res
}

func osFinding(lines []string, arch string) Finding {
	rel := osRelease(lines)
	name := rel["NAME"]
	if v := rel["VERSION_ID"]; v != "" && name != "" {
		name += " " + v
	}
	if name == "" {
		name = rel["PRETTY_NAME"]
	}
	supported := rel["ID"] == "debian" || rel["ID"] == "ubuntu"
	switch {
	case len(lines) == 0:
		return Finding{
			ID: "os", Level: LevelBlock,
			Text:   "Не удалось узнать, какая система стоит на сервере",
			Advice: "Wynd ставится на Debian и Ubuntu. Выберите одну из них при заказе сервера или переустановите систему в панели хостинга.",
		}
	case !supported:
		return Finding{
			ID: "os", Level: LevelBlock,
			Text:   fmt.Sprintf("На сервере %s", name),
			Advice: "Wynd ставится на Debian и Ubuntu. Переустановите систему в панели хостинга — обычно это кнопка «Переустановить ОС».",
		}
	case arch != "" && arch != "x86_64" && arch != "aarch64":
		return Finding{
			ID: "os", Level: LevelBlock,
			Text:   fmt.Sprintf("Процессор сервера (%s) не подходит", arch),
			Advice: "Wynd собран для обычных 64-разрядных серверов (x86-64 и ARM64). Закажите сервер такого типа.",
		}
	}
	return Finding{ID: "os", Level: LevelOK, Text: name + " — подходит"}
}

// rootFinding молчит, когда прав хватает: человеку незачем знать слово root,
// пока оно не мешает.
func rootFinding(user, uid string, sudo bool) (Finding, bool) {
	if uid == "0" || sudo {
		return Finding{}, false
	}
	return Finding{
		ID: "root", Level: LevelBlock,
		Text:   fmt.Sprintf("Пользователю %s нельзя ставить программы на сервер", user),
		Advice: "Подключитесь пользователем root — он указан в письме хостинга рядом с паролем.",
	}, true
}

func diskFinding(line string) Finding {
	// df -Pk: файловая система, всего, занято, свободно (КБ), процент, точка.
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return Finding{
			ID: "disk", Level: LevelNote,
			Text: "Не удалось узнать, сколько свободного места",
			Plan: "проверим ещё раз перед установкой",
		}
	}
	kb, err := strconv.ParseUint(fields[3], 10, 64)
	if err != nil {
		return Finding{
			ID: "disk", Level: LevelNote,
			Text: "Не удалось узнать, сколько свободного места",
			Plan: "проверим ещё раз перед установкой",
		}
	}
	free := kb * 1024
	text := "Свободно " + formatGB(free)
	if free < minFreeBytes {
		return Finding{
			ID: "disk", Level: LevelBlock,
			Text:   text + " — этого мало",
			Advice: fmt.Sprintf("Нужно хотя бы %s: сам Wynd и первые фотографии. Освободите место или увеличьте диск в панели хостинга.", formatGB(minFreeBytes)),
		}
	}
	return Finding{ID: "disk", Level: LevelOK, Text: text}
}

func formatGB(n uint64) string {
	gb := float64(n) / (1 << 30)
	if gb >= 10 {
		return fmt.Sprintf("%.0f ГБ", gb)
	}
	return strings.Replace(fmt.Sprintf("%.1f ГБ", gb), ".", ",", 1)
}

func clockFinding(epoch, ntp string, now time.Time) Finding {
	sec, err := strconv.ParseInt(epoch, 10, 64)
	if err != nil {
		return Finding{
			ID: "clock", Level: LevelNote,
			Text: "Не удалось сверить часы сервера",
			Plan: "настроим синхронизацию времени",
		}
	}
	skew := time.Unix(sec, 0).Sub(now)
	if skew < 0 {
		skew = -skew
	}
	if skew > maxClockSkew {
		return Finding{
			ID: "clock", Level: LevelNote,
			Text: "Часы сервера расходятся с вашими на " + formatSkew(skew),
			Plan: "настроим синхронизацию времени",
		}
	}
	if ntp == "no" {
		return Finding{
			ID: "clock", Level: LevelNote,
			Text: "Часы идут верно, но сами не сверяются",
			Plan: "настроим синхронизацию времени",
		}
	}
	return Finding{ID: "clock", Level: LevelOK, Text: "Часы идут верно"}
}

func formatSkew(d time.Duration) string {
	if d < 2*time.Minute {
		return fmt.Sprintf("%d с", int(d.Seconds()))
	}
	if d < 2*time.Hour {
		return fmt.Sprintf("%d мин", int(d.Minutes()))
	}
	return fmt.Sprintf("%d ч", int(d.Hours()))
}

func dockerFinding(docker, compose string) Finding {
	switch {
	case docker == "":
		return Finding{ID: "docker", Level: LevelNote, Text: "Docker не установлен", Plan: "поставим"}
	case compose == "":
		return Finding{
			ID: "docker", Level: LevelNote,
			Text: "Docker есть, но без Compose",
			Plan: "доставим Compose",
		}
	}
	return Finding{ID: "docker", Level: LevelOK, Text: "Docker установлен"}
}

// knownProxies — веб-серверы, за которые Wynd умеет встать (deploy/proxy).
var knownProxies = map[string]string{
	"nginx":   "nginx",
	"apache2": "apache",
	"httpd":   "apache",
	"caddy":   "caddy",
}

// portsFinding читает `ss -H -ltnp`: кто слушает 80 и 443.
func portsFinding(lines []string) (string, Finding) {
	type holder struct{ ports map[int]bool }
	holders := map[string]*holder{}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		local := fields[3]
		i := strings.LastIndex(local, ":")
		if i < 0 {
			continue
		}
		port, err := strconv.Atoi(local[i+1:])
		if err != nil || (port != 80 && port != 443) {
			continue
		}
		name := "неизвестная программа"
		if _, rest, ok := strings.Cut(line, `users:(("`); ok {
			if proc, _, ok := strings.Cut(rest, `"`); ok && proc != "" {
				name = proc
			}
		}
		if holders[name] == nil {
			holders[name] = &holder{ports: map[int]bool{}}
		}
		holders[name].ports[port] = true
	}
	if len(holders) == 0 {
		return "", Finding{ID: "ports", Level: LevelOK, Text: "Порты 80 и 443 свободны"}
	}
	names := make([]string, 0, len(holders))
	for name := range holders {
		names = append(names, name)
	}
	sort.Strings(names)
	// docker-proxy на 80/443 — чей-то контейнер; какой именно, осмотр не знает.
	if len(names) == 1 {
		if proxy, ok := knownProxies[names[0]]; ok {
			return proxy, Finding{
				ID: "ports", Level: LevelNote,
				Text: fmt.Sprintf("Уже работает %s и занимает %s", proxy, portList(holders[names[0]].ports)),
				Plan: "его не трогаем, Wynd встанет за ним",
			}
		}
	}
	all := map[int]bool{}
	for _, h := range holders {
		for p := range h.ports {
			all[p] = true
		}
	}
	return "", Finding{
		ID: "ports", Level: LevelBlock,
		Text:   fmt.Sprintf("%s на сервере уже занят: %s", capitalize(portList(all)), strings.Join(names, ", ")),
		Advice: "Через эти порты сайты открываются в браузере. Wynd умеет встать за nginx, Apache и Caddy, а эту программу не знает — мы её не трогаем. Освободите порты или возьмите для Wynd отдельный сервер.",
	}
}

func portList(ports map[int]bool) string {
	switch {
	case ports[80] && ports[443]:
		return "порты 80 и 443"
	case ports[80]:
		return "порт 80"
	}
	return "порт 443"
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// parseAddresses читает `ip -4 -o addr show scope global`.
func parseAddresses(lines []string) []string {
	var res []string
	for _, line := range lines {
		fields := strings.Fields(line)
		for i, f := range fields {
			if f == "inet" && i+1 < len(fields) {
				addr, _, _ := strings.Cut(fields[i+1], "/")
				res = append(res, addr)
			}
		}
	}
	return res
}
