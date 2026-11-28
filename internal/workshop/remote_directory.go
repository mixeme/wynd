package workshop

import (
	"context"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const remoteDirectoryLimit = 256

// RemoteDirectoryList is a read-only view of a directory on the SSH server.
type RemoteDirectoryList struct {
	OK        bool                    `json:"ok"`
	Path      string                  `json:"path,omitempty"`
	Parent    string                  `json:"parent,omitempty"`
	Folders   []RemoteDirectoryFolder `json:"folders"`
	CanSelect bool                    `json:"can_select"`
	Truncated bool                    `json:"truncated,omitempty"`
	Message   string                  `json:"message,omitempty"`
}

type RemoteDirectoryFolder struct {
	Name       string `json:"name"`
	HasFolders bool   `json:"has_folders"`
	CanSelect  bool   `json:"can_select"`
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
	// parent. Report physical children and stop at the first subdirectory of
	// each child so the tree can show expansion controls without loading it.
	script := "dir=" + ShellPath(path.Clean(directory)) + "\nlimit=" + strconv.Itoa(remoteDirectoryLimit+1) + `
while [ ! -d "$dir" ]; do
  if [ -e "$dir" ] || [ -L "$dir" ] || [ "$dir" = / ]; then exit 2; fi
  dir=${dir%/*}
  [ -n "$dir" ] || dir=/
done
cd -P "$dir" 2>/dev/null || exit 2
[ -r . ] && [ -x . ] || exit 2
current=$(pwd -P) || exit 2
printf '%s\000' "$current"
count=0
for folder in ./* ./.[!.]* ./..?*; do
  [ ! -L "$folder" ] && [ -d "$folder" ] || continue
  has_folders=$(find -P "$folder" -mindepth 1 -maxdepth 1 -type d -printf 1 -quit 2>/dev/null) || has_folders=1
  printf '%s%s\000' "${has_folders:-0}" "${folder#./}"
  count=$((count + 1))
  [ "$count" -lt "$limit" ] || { printf '!\000'; break; }
done
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

func selectableRemoteDirectory(directory string) bool {
	layout, err := NewLayout(directory)
	return err == nil && layout.Root() == directory
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
	list := RemoteDirectoryList{
		OK: true, Path: current, Parent: parent, Folders: []RemoteDirectoryFolder{},
		CanSelect: selectableRemoteDirectory(current),
	}
	for {
		entry, remaining, complete := strings.Cut(rest, "\x00")
		if !complete {
			list.Truncated = true
			break
		}
		if entry == "" {
			break
		}
		rest = remaining
		marker, folder := entry[0], entry[1:]
		if marker != '0' && marker != '1' {
			list.Truncated = true
			break
		}
		if validRemoteDirectoryName(folder) && !strings.Contains(folder, "/") && folder != "." && folder != ".." {
			if len(list.Folders) == remoteDirectoryLimit {
				list.Truncated = true
				break
			}
			list.Folders = append(list.Folders, RemoteDirectoryFolder{
				Name: folder, HasFolders: marker == '1', CanSelect: selectableRemoteDirectory(path.Join(current, folder)),
			})
		}
	}
	slices.SortFunc(list.Folders, func(a, b RemoteDirectoryFolder) int { return strings.Compare(a.Name, b.Name) })
	return list
}
