package version

import (
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
	root := filepath.Join(filepath.Dir(file), "..", "..", "VERSION")
	data, err := os.ReadFile(root)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(data))
	if got != Number {
		t.Fatalf("VERSION is %q, version.Number is %q", got, Number)
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
