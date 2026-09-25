package version

import (
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

func TestStringIndependentOfCWD(t *testing.T) {
	t.Chdir(t.TempDir())
	if String() != Number {
		t.Fatalf("String()=%q after chdir, want %q", String(), Number)
	}
	if String() == "0.0.0-dev" {
		t.Fatal("installed binary must not fall back to 0.0.0-dev")
	}
}
