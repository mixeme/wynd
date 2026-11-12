package installer

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// Layout — размещение установки Wynd на сервере: корень и всё, что от него
// зависит (план «другой каталог установки Docker», проба 2026-11-12).
//
// Один объект на операцию, а не глобальная переменная: обновление или удаление
// профиля не должны уметь уйти в папку другой установки из-за общего
// состояния. Сейчас Мастерская работает с DefaultLayout(); произвольный корень
// проверяется тестами проб, пока поле в окне не добавлено.
type Layout struct {
	root string
}

// DefaultInstallRoot — каталог установки по умолчанию: тот же, что у ручной
// установки по deploy/docker и у прежних версий Мастерской.
const DefaultInstallRoot = "/opt/wynd"

// DefaultLayout — размещение по умолчанию (/opt/wynd).
func DefaultLayout() Layout {
	return Layout{root: DefaultInstallRoot}
}

// NewLayout проверяет корень каталога установки и возвращает размещение.
//
// Принимается абсолютный путь, не корень файловой системы и не системный
// каталог целиком: каталог установки Мастерская создаёт и, при удалении,
// стирает целиком, поэтому «/» и «/etc» здесь означали бы уничтожение
// сервера. Пробелы, кавычки и нелатинский текст в именах допустимы — в
// командах путь подставляется через shellRoot и экранируется.
func NewLayout(root string) (Layout, error) {
	raw := strings.TrimSpace(root)
	if raw == "" {
		return Layout{}, fmt.Errorf("каталог установки не указан")
	}
	if !strings.HasPrefix(raw, "/") {
		return Layout{}, fmt.Errorf("каталог установки должен быть абсолютным: %q", root)
	}
	// Управляющие символы ломают команды и журнал; NUL и переводы строк в имени
	// серверу всё равно не нужны.
	if strings.ContainsAny(raw, "\x00\n\r\t") {
		return Layout{}, fmt.Errorf("в каталоге установки недопустимый символ: %q", root)
	}
	clean := path.Clean(raw)
	if strings.Contains(clean, "..") {
		return Layout{}, fmt.Errorf("каталог установки с «..» не принимается: %q", root)
	}
	if clean == "/" {
		return Layout{}, fmt.Errorf("корень файловой системы каталогом установки быть не может")
	}
	// Системные каталоги целиком: их содержимое принадлежит серверу, а не
	// Wynd. Глубже (/etc/wynd, /var/lib/wynd) — уже папка установки.
	if systemRoots[clean] {
		return Layout{}, fmt.Errorf("%s — системный каталог, выберите папку внутри него или свой путь", clean)
	}
	return Layout{root: clean}, nil
}

// systemRoots — каталоги, которые целиком каталогом установки не принимаются.
var systemRoots = map[string]bool{
	"/bin": true, "/boot": true, "/dev": true, "/etc": true, "/home": true,
	"/lib": true, "/lib32": true, "/lib64": true, "/media": true, "/mnt": true,
	"/opt": true, "/proc": true, "/root": true, "/run": true, "/sbin": true,
	"/srv": true, "/sys": true, "/tmp": true, "/usr": true, "/var": true,
}

// Root — корень установки как он записан (без кавычек).
func (l Layout) Root() string { return l.root }

var plainPath = regexp.MustCompile(`^[A-Za-z0-9/._-]+$`)

// shellRoot — корень для подстановки в команду оболочки. Обычный путь идёт
// как есть (так же, как раньше писалось /opt/wynd); с пробелами, кавычками
// или нелатинскими буквами — экранируется целиком.
func (l Layout) shellRoot() string {
	if plainPath.MatchString(l.root) {
		return l.root
	}
	return shQuote(l.root)
}

// ShellRoot — корень для подстановки в команду: экранированный, если нужно.
func (l Layout) ShellRoot() string { return l.shellRoot() }

// Data — данные Wynd (база и файлы), рядом с настройкой, не томом Docker.
func (l Layout) Data() string { return path.Join(l.root, "data") }

// Caddy — данные своего Caddy.
func (l Layout) Caddy() string { return path.Join(l.root, "caddy") }

// Acme — сертификаты и служебные файлы ACME своего прокси.
func (l Layout) Acme() string { return path.Join(l.root, "acme") }

// Compose — файл compose.yaml установки.
func (l Layout) Compose() string { return path.Join(l.root, "compose.yaml") }

// Caddyfile — настройка своего Caddy.
func (l Layout) Caddyfile() string { return path.Join(l.root, "Caddyfile") }

// Backups — копии данных, которые Мастерская делает на сервере.
func (l Layout) Backups() string { return path.Join(l.root, "backups") }

// StoppedMark — пометка «сайт остановлен Мастерской».
func (l Layout) StoppedMark() string { return path.Join(l.root, ".workshop-stopped") }

// UpdateMark — пометка начатого обновления.
func (l Layout) UpdateMark() string { return path.Join(l.root, ".workshop-update") }

// MoveMark — пометка начатого переезда.
func (l Layout) MoveMark() string { return path.Join(l.root, ".workshop-move") }

// RunningFilter — docker ps по контейнеру Wynd этой установки.
func (l Layout) RunningFilter() string {
	return "docker ps -q --filter label=com.docker.compose.project.working_dir=" +
		l.shellRoot() + " --filter label=com.docker.compose.service=wynd 2>/dev/null"
}

// ContainerFilter — любой контейнер Wynd этой установки, включая остановленный.
func (l Layout) ContainerFilter() string {
	return "docker ps -aq --filter label=com.docker.compose.project.working_dir=" +
		l.shellRoot() + " --filter label=com.docker.compose.service=wynd 2>/dev/null"
}

// ComposeCommand — начало команды docker compose в этом каталоге.
func (l Layout) ComposeCommand() string {
	return "cd " + l.shellRoot() + " && docker compose "
}

// StateScript — что стоит на сервере: состояние, пометки, свободное место.
func (l Layout) StateScript() string {
	return `cd ` + l.shellRoot() + ` 2>/dev/null || { echo '@none'; exit 0; }
test -f compose.yaml || { echo '@none'; exit 0; }
echo "@image $(sed -n 's/^[[:space:]]*image:[[:space:]]*\([^[:space:]]*wynd:[^[:space:]]*\)[[:space:]]*$/\1/p' compose.yaml | head -n 1)"
echo "@url $(sed -n 's/^[[:space:]]*WYND_PUBLIC_URL:[[:space:]]*//p' compose.yaml | head -n 1)"
echo "@running $(` + l.RunningFilter() + ` | head -n 1)"
echo "@container $(` + l.ContainerFilter() + ` | head -n 1)"
echo "@stopped $(cat ` + l.StoppedMark() + ` 2>/dev/null)"
echo "@update $(cat ` + l.UpdateMark() + ` 2>/dev/null)"
echo "@move $(cat ` + l.MoveMark() + ` 2>/dev/null)"
echo "@free $(df -Pk ` + l.shellRoot() + ` | tail -n 1)"`
}

// BackupListScript — файлы установки для копии на компьютер, кроме копий и
// живой базы; база приходит из снимка.
// SnapshotDir — снимок базы, сделанный на сервере перед копией.
func (l Layout) SnapshotDir() string { return path.Join(l.Data(), ".workshop-backup") }

// BackupListScript — файлы установки для копии на компьютер, кроме копий и
// живой базы; база приходит из снимка.
func (l Layout) BackupListScript() string {
	return `cd ` + l.shellRoot() + ` && find . \( -path ./backups -o -path ./data/backups -o -path ./data/.workshop-copy -o -path ./data/.workshop-backup -o -path ./src \) -prune -o -type f ! -path './data/wynd.db*' -printf '%s\t%P\n'
cd ` + l.SnapshotDir() + ` && find . -type f -printf '%s\tdata/%P\n'`
}
