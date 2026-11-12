package installer

import (
	"strings"
	"testing"
)

// Проба параметризованного размещения (план «другой каталог установки Docker»,
// шаг 1): пути и команды собираются из проверенного корня, а не из константы
// пакета. Для /opt/wynd вывод обязан совпадать с прежним байт в байт — иначе
// проба меняет поведение Мастерской на живых серверах.

func TestDefaultLayoutMatchesCurrentPaths(t *testing.T) {
	l := DefaultLayout()
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"root", l.Root(), "/opt/wynd"},
		{"default_root", DefaultInstallRoot, "/opt/wynd"},
		{"data", l.Data(), dataDir},
		{"caddy", l.Caddy(), caddyDir},
		{"compose", l.Compose(), composePath},
		{"caddyfile", l.Caddyfile(), installDir + "/Caddyfile"},
		{"backups", l.Backups(), backupsDir},
		{"stopped", l.StoppedMark(), stoppedMark},
		{"update", l.UpdateMark(), updateMark},
		{"move", l.MoveMark(), moveMark},
		{"acme", l.Acme(), acmeRoot},
		{"snapshot", l.SnapshotDir(), snapshotDir},
		{"running", l.RunningFilter(), wyndRunning},
		{"composeCmd", l.ComposeCommand(), "cd " + installDir + " && docker compose "},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: получено %q, ожидалось %q", c.name, c.got, c.want)
		}
	}
}

// Скрипты состояния и списка копии — те же, что были: прежние константы
// собраны здесь заново из оставшихся констант пакета.
func TestLayoutScriptsMatchPrevious(t *testing.T) {
	wantState := `cd ` + installDir + ` 2>/dev/null || { echo '@none'; exit 0; }
test -f compose.yaml || { echo '@none'; exit 0; }
echo "@image $(sed -n 's/^[[:space:]]*image:[[:space:]]*\([^[:space:]]*wynd:[^[:space:]]*\)[[:space:]]*$/\1/p' compose.yaml | head -n 1)"
echo "@url $(sed -n 's/^[[:space:]]*WYND_PUBLIC_URL:[[:space:]]*//p' compose.yaml | head -n 1)"
echo "@running $(` + wyndRunning + ` | head -n 1)"
echo "@container $(docker ps -aq --filter label=com.docker.compose.project.working_dir=` + installDir + ` --filter label=com.docker.compose.service=wynd 2>/dev/null | head -n 1)"
echo "@stopped $(cat ` + stoppedMark + ` 2>/dev/null)"
echo "@update $(cat ` + updateMark + ` 2>/dev/null)"
echo "@move $(cat ` + moveMark + ` 2>/dev/null)"
echo "@free $(df -Pk ` + installDir + ` | tail -n 1)"`
	if got := DefaultLayout().StateScript(); got != wantState {
		t.Fatalf("скрипт состояния изменился:\n%s", got)
	}

	wantList := `cd ` + installDir + ` && find . \( -path ./backups -o -path ./data/backups -o -path ./data/.workshop-copy -o -path ./data/.workshop-backup -o -path ./src \) -prune -o -type f ! -path './data/wynd.db*' -printf '%s\t%P\n'
cd ` + snapshotDir + ` && find . -type f -printf '%s\tdata/%P\n'`
	if got := DefaultLayout().BackupListScript(); got != wantList {
		t.Fatalf("список копии изменился:\n%s", got)
	}
}

func TestLayoutCustomRoot(t *testing.T) {
	l, err := NewLayout("/srv/wynd")
	if err != nil {
		t.Fatalf("каталог не принят: %v", err)
	}
	if l.Root() != "/srv/wynd" || l.Data() != "/srv/wynd/data" ||
		l.Compose() != "/srv/wynd/compose.yaml" || l.Caddyfile() != "/srv/wynd/Caddyfile" ||
		l.Backups() != "/srv/wynd/backups" || l.Acme() != "/srv/wynd/acme" {
		t.Fatalf("пути не от корня: %+v", l)
	}
	// Фильтр контейнера — по своей папке, а не по /opt/wynd: иначе обновление
	// чужой установки задело бы нашу.
	if strings.Contains(l.RunningFilter(), "/opt/wynd") || !strings.Contains(l.RunningFilter(), "working_dir=/srv/wynd") {
		t.Fatalf("фильтр контейнера: %s", l.RunningFilter())
	}
	if !strings.Contains(l.StateScript(), "cd /srv/wynd 2>/dev/null") ||
		strings.Contains(l.StateScript(), "/opt/wynd") {
		t.Fatalf("скрипт состояния: %s", l.StateScript())
	}
	if !strings.Contains(l.BackupListScript(), "cd /srv/wynd &&") {
		t.Fatalf("список копии: %s", l.BackupListScript())
	}
}

func TestLayoutQuotesUnsafeRoot(t *testing.T) {
	l, err := NewLayout("/srv/Мой wynd'ы")
	if err != nil {
		t.Fatalf("каталог с пробелом и нелатинским именем отклонён: %v", err)
	}
	cmd := l.ComposeCommand()
	if !strings.HasPrefix(cmd, "cd '") {
		t.Fatalf("путь не экранирован: %s", cmd)
	}
	// Экранирование одинарной кавычки — штатное для оболочки.
	if !strings.Contains(cmd, `'\''`) {
		t.Fatalf("кавычка в пути не экранирована: %s", cmd)
	}
	// Кавычки появляются только там, где нужны: обычный путь идёт как есть.
	if got := DefaultLayout().ComposeCommand(); got != "cd /opt/wynd && docker compose " {
		t.Fatalf("обычный путь изменился: %s", got)
	}
}

func TestLayoutRejectsDangerousRoots(t *testing.T) {
	bad := []string{
		"", "   ", "opt/wynd", "./wynd", "../wynd",
		"/", "/etc", "/usr", "/var", "/opt", "/srv", "/tmp",
		"/opt/wynd/../../etc", "/srv/wynd\nrm -rf /", "/srv/wynd\x00",
	}
	for _, root := range bad {
		if _, err := NewLayout(root); err == nil {
			t.Errorf("%q принят, а не должен", root)
		}
	}
	good := []string{"/opt/wynd", "/srv/wynd", "/mnt/storage/wynd", "/opt/wynd2", "/srv/Мой wynd"}
	for _, root := range good {
		if _, err := NewLayout(root); err != nil {
			t.Errorf("%q отклонён: %v", root, err)
		}
	}
}

// Хвостовой слэш убирается; путь с «..», который после очистки безопасен,
// принимается очищенным.
func TestLayoutCleansRoot(t *testing.T) {
	l, err := NewLayout("/srv/wynd/")
	if err != nil {
		t.Fatal(err)
	}
	if l.Root() != "/srv/wynd" {
		t.Fatalf("корень не очищен: %q", l.Root())
	}
	if _, err := NewLayout("/srv/wynd/../wynd2"); err != nil {
		t.Fatalf("чистый путь с .. отклонён: %v", err)
	}
}
