package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseListing(t *testing.T) {
	files := parseListing("120\tcompose.yaml\n5\tdata/blobs/ab/x\r\nbad line\n4096\tdata/wynd.db\n")
	if len(files) != 3 || files[0].Path != "compose.yaml" || files[2].Path != "data/wynd.db" || files[1].Size != 5 {
		t.Fatalf("список: %+v", files)
	}
}

// Докачка (8г): скачанное целиком не качается, оборванное докачивается,
// фото из прошлого бэкапа берутся ссылкой; зашифрованный кусок — заново.
func TestPlanFetch(t *testing.T) {
	blob := RemoteFile{Path: "data/blobs/ab/x", Size: 100}
	db := RemoteFile{Path: "data/wynd.db", Size: 50}
	prev := map[string]int64{"data/blobs/ab/x": 100, "data/wynd.db": 50}
	cases := []struct {
		name      string
		f         RemoteFile
		encrypted bool
		done      bool
		partial   int64
		prev      map[string]int64
		want      fetchAction
	}{
		{"скачан", blob, false, true, 100, prev, fetchSkip},
		{"наполовину", blob, false, false, 40, prev, fetchResume},
		{"зашифрованный кусок", blob, true, false, 40, prev, fetchLink},
		{"из прошлого", blob, false, false, 0, prev, fetchLink},
		{"другой размер", RemoteFile{Path: blob.Path, Size: 7}, false, false, 0, prev, fetchAll},
		{"база всегда заново", db, false, false, 0, prev, fetchAll},
		{"прошлого нет", blob, false, false, 0, nil, fetchAll},
	}
	for _, c := range cases {
		if got := planFetch(c.f, c.encrypted, c.done, c.partial, c.prev); got != c.want {
			t.Fatalf("%s: %v, нужно %v", c.name, got, c.want)
		}
	}
}

// Новый бэкап заменяет прошлый только целиком.
func TestSwapFolders(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "family")
	work := folder + ".new"
	for _, p := range []string{filepath.Join(folder, "old.txt"), filepath.Join(work, "new.txt")} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := swapFolders(folder, work); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(folder, "new.txt")); err != nil {
		t.Fatal("нового нет на месте")
	}
	for _, p := range []string{filepath.Join(folder, "old.txt"), work, folder + ".old"} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("осталось %s", p)
		}
	}
}
