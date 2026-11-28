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
			!strings.Contains(command, `find -P "$folder" -mindepth 1 -maxdepth 1 -type d -printf 1 -quit`) ||
			len(input) != 0 {
			t.Errorf("unexpected directory command: %q", command)
		}
		return "/srv\x001семья\x000apps\x00\x00", 0
	})
	w := &Wizard{session: sess}
	list := w.BrowseInstallDirectories(t.Context(), requested)
	if !list.OK || list.Path != "/srv" || list.Parent != "/" || list.CanSelect || list.Truncated ||
		!slices.Equal(list.Folders, []RemoteDirectoryFolder{
			{Name: "apps", CanSelect: true}, {Name: "семья", HasFolders: true, CanSelect: true},
		}) {
		t.Fatalf("unexpected directory list: %+v", list)
	}
	if invalid := w.BrowseInstallDirectories(t.Context(), "../srv"); invalid.OK {
		t.Fatalf("accepted relative path: %+v", invalid)
	}
}

func TestRemoteDirectoryListKeepsCanonicalSelectablePaths(t *testing.T) {
	root := parseRemoteDirectories("/\x001opt\x000srv\x00\x00")
	if !root.OK || root.Parent != "" || root.CanSelect || !slices.Equal(root.Folders, []RemoteDirectoryFolder{
		{Name: "opt", HasFolders: true}, {Name: "srv"},
	}) {
		t.Fatalf("root: %+v", root)
	}
	data := parseRemoteDirectories("/srv/Моя папка\x001nested\x000unused/slash\x000bad\nname\x00partial")
	if !data.OK || !data.CanSelect || data.Parent != "/srv" || !data.Truncated ||
		!slices.Equal(data.Folders, []RemoteDirectoryFolder{{Name: "nested", HasFolders: true, CanSelect: true}}) {
		t.Fatalf("partial directory list: %+v", data)
	}
	if list := parseRemoteDirectories("/srv/folder \x00\x00"); list.CanSelect {
		t.Fatalf("the selected path would change on save: %+v", list)
	}
	out := strings.Builder{}
	out.WriteString("/srv/wynd\x00")
	for i := 0; i <= remoteDirectoryLimit; i++ {
		out.WriteString("0folder-" + strconv.Itoa(i) + "\x00")
	}
	out.WriteByte(0)
	if list := parseRemoteDirectories(out.String()); len(list.Folders) != remoteDirectoryLimit || !list.Truncated {
		t.Fatalf("listing limit: %d, truncated = %v", len(list.Folders), list.Truncated)
	}
}

func TestCreateInstallDirectoryChecksParentAndCreatesOneLevel(t *testing.T) {
	const parent = "/srv/Мои папки"
	const name = "Wynd 'семья'; $(не-команда)"
	for _, test := range []struct {
		code    int
		message string
	}{
		{code: 0},
		{code: 2, message: "Не удалось открыть выбранный каталог"},
		{code: 3, message: "с таким именем уже есть"},
		{code: 4, message: "Не удалось создать папку"},
	} {
		t.Run(strconv.Itoa(test.code), func(t *testing.T) {
			sess := dialTestHost(t, func(command string, input []byte) (string, int) {
				if len(input) != 0 ||
					!strings.HasPrefix(command, "parent="+ShellPath(parent)+"\nname="+shQuote(name)+"\n") ||
					!strings.Contains(command, `[ "$current" = "$parent" ] || exit 2`) ||
					strings.Count(command, "mkdir") != 1 || !strings.Contains(command, `mkdir -- "$name"`) {
					t.Errorf("unexpected creation command: %q", command)
				}
				return "", test.code
			})
			w := &Wizard{session: sess}
			result := w.CreateInstallDirectory(t.Context(), parent, name)
			if test.code == 0 {
				if !result.OK || result.Path != parent+"/"+name {
					t.Fatalf("created folder: %+v", result)
				}
			} else if result.OK || !strings.Contains(result.Message, test.message) {
				t.Fatalf("wanted %q, got %+v", test.message, result)
			}
		})
	}
}

func TestCreateInstallDirectoryRejectsInvalidPathsBeforeSSH(t *testing.T) {
	sess := dialTestHost(t, func(command string, _ []byte) (string, int) {
		t.Errorf("unexpected remote command: %q", command)
		return "", 1
	})
	w := &Wizard{session: sess}
	for _, test := range []struct{ parent, name string }{
		{"srv", "wynd"},
		{"/srv//apps", "wynd"},
		{"/srv", ""},
		{"/srv", "../wynd"},
		{"/srv", "apps/wynd"},
		{"/srv", "wynd\n"},
		{"/srv", " wynd"},
		{"/", "etc"},
	} {
		if result := w.CreateInstallDirectory(t.Context(), test.parent, test.name); result.OK {
			t.Errorf("accepted %q in %q: %+v", test.name, test.parent, result)
		}
	}
}
