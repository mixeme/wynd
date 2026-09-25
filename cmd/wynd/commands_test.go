package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Инвариант: служебные команды при опечатке в WYND_DATA_DIR завершаются с
// ошибкой и ничего не создают — раньше после них оставался каталог с
// blobs/, keys/ и пустым wynd.db.
func TestCommandsWithMissingDataDirCreateNothing(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "wynd-test.exe")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = packageDir(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	for _, args := range [][]string{
		{"backup", filepath.Join(t.TempDir(), "dest")},
		{"admin-password"},
	} {
		missing := filepath.Join(t.TempDir(), "typo")
		cmd := exec.Command(bin, args...)
		cmd.Env = append(os.Environ(), "WYND_DATA_DIR="+missing, "WYND_PUBLIC_URL=", "WYND_LISTEN=")
		cmd.Stdin = strings.NewReader("new-secret-1\n")
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("%s: команда прошла без базы:\n%s", args[0], out)
		}
		if !strings.Contains(string(out), "WYND_DATA_DIR") {
			t.Fatalf("%s: нет подсказки про WYND_DATA_DIR:\n%s", args[0], out)
		}
		if _, err := os.Stat(missing); !os.IsNotExist(err) {
			t.Fatalf("%s: каталог данных создан: %v", args[0], err)
		}
	}
}
