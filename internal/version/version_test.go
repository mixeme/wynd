package version

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMatchesVERSIONFile(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	repo := filepath.Join(filepath.Dir(file), "..", "..")
	data, err := os.ReadFile(filepath.Join(repo, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(data))
	if got != Number {
		t.Fatalf("VERSION is %q, version.Number is %q", got, Number)
	}

	pkgRaw, err := os.ReadFile(filepath.Join(repo, "web", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(pkgRaw, &pkg); err != nil {
		t.Fatal(err)
	}
	if pkg.Version != Number {
		t.Fatalf("web/package.json version is %q, want %q", pkg.Version, Number)
	}
}

// Сторож на поля info.version обеих спек: README обещает, что они равны
// VERSION, а на деле спеки отставали на три минорных версии — TestMatchesVERSIONFile
// yaml не читал (DOC-2).
func TestOpenAPIVersionMatchesVERSION(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	repo := filepath.Join(filepath.Dir(file), "..", "..")
	for _, name := range []string{"openapi-participant.yaml", "openapi-admin.yaml"} {
		path := filepath.Join(repo, "internal", "api", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		got := ""
		inInfo := false
		sc := bufio.NewScanner(bytes.NewReader(data))
		for sc.Scan() {
			// TrimSpace, а не сравнение строк: в рабочем дереве файлы бывают CRLF.
			line := strings.TrimSpace(sc.Text())
			if line == "info:" {
				inInfo = true
				continue
			}
			if !inInfo {
				continue
			}
			// Блок info кончился: начался следующий ключ верхнего уровня.
			if line != "" && !strings.HasPrefix(sc.Text(), " ") {
				break
			}
			if strings.HasPrefix(line, "version:") {
				got = strings.TrimSpace(strings.TrimPrefix(line, "version:"))
				break
			}
		}
		if got != Number {
			t.Fatalf("%s: info.version is %q, want %q", name, got, Number)
		}
	}
}

func TestStringIndependentOfCWD(t *testing.T) {
	t.Chdir(t.TempDir())
	if String() != Number {
		t.Fatalf("String()=%q after chdir, want %q", String(), Number)
	}
	if String() == "0.0.0-dev" {
		t.Fatal("installed binary must not fall back to 0.0.0-dev")
	}
}
