package workshop

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestBrowseInstallDirectoriesUsesSelectedSSHAndQuotesThePath(t *testing.T) {
	const requested = "/srv/Мой 'каталог'; $(не-команда)"
	sess := dialTestHost(t, func(command string, input []byte) (string, int) {
		if !strings.HasPrefix(command, "dir="+ShellPath(requested)+"\n") ||
			!strings.Contains(command, "cd -P") ||
			!strings.Contains(command, "find -P . -mindepth 1 -maxdepth 1 -type d") ||
			len(input) != 0 {
			t.Errorf("unexpected directory command: %q", command)
		}
		return "/srv\x00семья\x00apps\x00\x00", 0
	})
	w := &Wizard{session: sess}
	list := w.BrowseInstallDirectories(t.Context(), requested)
	if !list.OK || list.Path != "/srv" || list.Parent != "/" || list.CanSelect || list.Truncated ||
		!slices.Equal(list.Folders, []string{"apps", "семья"}) {
		t.Fatalf("unexpected directory list: %+v", list)
	}
	if invalid := w.BrowseInstallDirectories(t.Context(), "../srv"); invalid.OK {
		t.Fatalf("accepted relative path: %+v", invalid)
	}
}

func TestRemoteDirectoryListKeepsCanonicalSelectablePaths(t *testing.T) {
	root := parseRemoteDirectories("/\x00opt\x00srv\x00\x00")
	if !root.OK || root.Parent != "" || root.CanSelect {
		t.Fatalf("root: %+v", root)
	}
	data := parseRemoteDirectories("/srv/Моя папка\x00nested\x00unused/slash\x00bad\nname\x00partial")
	if !data.OK || !data.CanSelect || data.Parent != "/srv" || !data.Truncated ||
		!slices.Equal(data.Folders, []string{"nested"}) {
		t.Fatalf("partial directory list: %+v", data)
	}
	if list := parseRemoteDirectories("/srv/folder \x00\x00"); list.CanSelect {
		t.Fatalf("the selected path would change on save: %+v", list)
	}
	out := strings.Builder{}
	out.WriteString("/srv/wynd\x00")
	for i := 0; i <= remoteDirectoryLimit; i++ {
		out.WriteString("folder-" + strconv.Itoa(i) + "\x00")
	}
	out.WriteByte(0)
	if list := parseRemoteDirectories(out.String()); len(list.Folders) != remoteDirectoryLimit || !list.Truncated {
		t.Fatalf("listing limit: %d, truncated = %v", len(list.Folders), list.Truncated)
	}
}
