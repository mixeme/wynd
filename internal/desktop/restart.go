package desktop

import (
	"os"
	"strconv"
	"strings"
)

// updateFromPIDEnv — прежний процесс обновления: новый запуск ждёт его выхода,
// иначе Wails отдаёт управление прежнему и новый закрывается (C153).
const updateFromPIDEnv = "WYND_UPDATE_FROM_PID"

// UpdateFromPID — pid прежнего окна из окружения нового запуска; 0 — обычный
// запуск.
func UpdateFromPID() int {
	pid, err := strconv.Atoi(os.Getenv(updateFromPIDEnv))
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}

// RestartEnv — окружение нового процесса после обновления. У AppImage из него
// убираются следы смонтированного образа: иначе новый образ унаследует пути
// библиотек и данных исчезнувшего каталога. Переменные-списки (PATH,
// LD_LIBRARY_PATH, *_PATH, *_DIRS) чистятся по частям, остальные с путём
// образа отбрасываются целиком. WYND_UPDATE_FROM_PID говорит новому процессу,
// кого ждать перед замком одного экземпляра.
func RestartEnv(pid int) []string {
	oldDir := os.Getenv("APPDIR")
	out := make([]string, 0, len(os.Environ())+1)
	for _, kv := range os.Environ() {
		name, value, ok := strings.Cut(kv, "=")
		if !ok {
			out = append(out, kv)
			continue
		}
		switch name {
		case "APPIMAGE", "APPDIR", "ARGV0", "OWD", updateFromPIDEnv:
			continue
		}
		if oldDir == "" || !strings.Contains(value, oldDir) {
			out = append(out, kv)
			continue
		}
		if !isPathList(name) {
			continue
		}
		kept := make([]string, 0, 4)
		for _, part := range strings.Split(value, string(os.PathListSeparator)) {
			if part != "" && !strings.Contains(part, oldDir) {
				kept = append(kept, part)
			}
		}
		if len(kept) > 0 {
			out = append(out, name+"="+strings.Join(kept, string(os.PathListSeparator)))
		}
	}
	return append(out, updateFromPIDEnv+"="+strconv.Itoa(pid))
}

func isPathList(name string) bool {
	return name == "PATH" || name == "LD_LIBRARY_PATH" ||
		strings.HasSuffix(name, "_PATH") || strings.HasSuffix(name, "_DIRS")
}
