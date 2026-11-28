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
