package workshop

import (
	"context"
	"path"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const remoteDirectoryLimit = 256

// RemoteDirectoryList is a read-only view of a directory on the SSH server.
type RemoteDirectoryList struct {
	OK        bool     `json:"ok"`
	Path      string   `json:"path,omitempty"`
	Parent    string   `json:"parent,omitempty"`
	Folders   []string `json:"folders"`
	CanSelect bool     `json:"can_select"`
	Truncated bool     `json:"truncated,omitempty"`
	Message   string   `json:"message,omitempty"`
}

type RemoteDirectoryCreation struct {
	OK      bool   `json:"ok"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message,omitempty"`
}

func (w *Wizard) BrowseInstallDirectories(ctx context.Context, directory string) RemoteDirectoryList {
	w.mu.Lock()
	sess := w.session
	if directory == "" {
		directory = w.layout().Root()
	}
	w.mu.Unlock()
	if sess == nil {
		return RemoteDirectoryList{Message: "Сначала подключитесь к серверу"}
	}
	return browseInstallDirectories(ctx, sess, directory)
}

// CreateInstallDirectory creates one folder inside the parent shown in the picker.
func (w *Wizard) CreateInstallDirectory(ctx context.Context, parent, name string) RemoteDirectoryCreation {
	if !path.IsAbs(parent) || !validRemoteDirectoryName(parent) || path.Clean(parent) != parent {
		return RemoteDirectoryCreation{Message: "Сначала выберите, где создать папку"}
	}
	if !validRemoteDirectoryName(name) || name == "." || name == ".." ||
		strings.ContainsRune(name, '/') || strings.TrimSpace(name) != name {
		return RemoteDirectoryCreation{Message: "Укажите имя новой папки без пути"}
	}
	root := path.Join(parent, name)
	layout, err := NewLayout(root)
	if err != nil || layout.Root() != root {
		return RemoteDirectoryCreation{Message: "Такой путь нельзя выбрать для установки"}
	}

	w.mu.Lock()
	sess := w.session
	w.mu.Unlock()
	if sess == nil {
		return RemoteDirectoryCreation{Message: "Сначала подключитесь к серверу"}
	}

	// The picker sends a physical path. If it now resolves elsewhere, ask the
	// operator to choose the parent again. mkdir either creates one directory
	// or fails if its name is occupied, including by a broken symlink.
	script := "parent=" + ShellPath(parent) + "\nname=" + shQuote(name) + `
cd -P "$parent" 2>/dev/null || exit 2
current=$(pwd -P) || exit 2
[ "$current" = "$parent" ] || exit 2
if mkdir -- "$name" 2>/dev/null; then exit 0; fi
if [ -e "$name" ] || [ -L "$name" ]; then exit 3; fi
exit 4`
	result, err := sess.Exec(ctx, script, nil, shortTimeout)
	if err != nil {
		return RemoteDirectoryCreation{Message: "Связь с сервером прервалась. Проверьте список папок перед повтором."}
	}
	switch result.Code {
	case 0:
		return RemoteDirectoryCreation{OK: true, Path: root}
	case 2:
		return RemoteDirectoryCreation{Message: "Не удалось открыть выбранный каталог. Укажите, где создать папку, ещё раз."}
	case 3:
		return RemoteDirectoryCreation{Message: "Папка или файл с таким именем уже есть"}
	default:
		return RemoteDirectoryCreation{Message: "Не удалось создать папку на сервере"}
	}
}

func browseInstallDirectories(ctx context.Context, sess *Session, directory string) RemoteDirectoryList {
	if !path.IsAbs(directory) || !validRemoteDirectoryName(directory) {
		return RemoteDirectoryList{Message: "Путь к папке на сервере должен начинаться с «/»"}
	}
	// If a draft install directory does not exist, open its closest existing
	// parent. List physical directories only, without reading their contents.
	script := "dir=" + ShellPath(path.Clean(directory)) + `
while [ ! -d "$dir" ]; do
  if [ -e "$dir" ] || [ -L "$dir" ] || [ "$dir" = / ]; then exit 2; fi
  dir=${dir%/*}
  [ -n "$dir" ] || dir=/
done
cd -P "$dir" 2>/dev/null || exit 2
current=$(pwd -P) || exit 2
printf '%s\000' "$current"
find -P . -mindepth 1 -maxdepth 1 -type d -printf '%f\0' 2>/dev/null || exit 2
printf '\000'`
	result, err := sess.Exec(ctx, script, nil, shortTimeout)
	if err != nil {
		return RemoteDirectoryList{Message: "Не удалось получить список папок на сервере"}
	}
	if result.Code != 0 {
		return RemoteDirectoryList{Message: "Не удалось открыть каталог на сервере"}
	}
	return parseRemoteDirectories(result.Output)
}

func validRemoteDirectoryName(name string) bool {
	return name != "" && utf8.ValidString(name) && strings.IndexFunc(name, unicode.IsControl) < 0
}

func parseRemoteDirectories(output string) RemoteDirectoryList {
	current, rest, ok := strings.Cut(output, "\x00")
	if !ok || !path.IsAbs(current) || !validRemoteDirectoryName(current) || path.Clean(current) != current {
		return RemoteDirectoryList{Message: "Не удалось определить путь к папке на сервере"}
	}
	parent := ""
	if current != "/" {
		parent = path.Dir(current)
	}
	layout, err := NewLayout(current)
	list := RemoteDirectoryList{
		OK: true, Path: current, Parent: parent, Folders: []string{},
		CanSelect: err == nil && layout.Root() == current,
	}
	for {
		folder, remaining, complete := strings.Cut(rest, "\x00")
		if !complete {
			list.Truncated = true
			break
		}
		if folder == "" {
			break
		}
		if validRemoteDirectoryName(folder) && !strings.Contains(folder, "/") && folder != "." && folder != ".." {
			if len(list.Folders) == remoteDirectoryLimit {
				list.Truncated = true
				break
			}
			list.Folders = append(list.Folders, folder)
		}
		rest = remaining
	}
	slices.Sort(list.Folders)
	return list
}
