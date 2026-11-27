package desktop

import (
	"os"
	"strings"
	"testing"
)

func envMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, kv := range env {
		name, value, ok := strings.Cut(kv, "=")
		if ok {
			out[name] = value
		}
	}
	return out
}

// Новый процесс после обновления AppImage не должен наследовать пути
// исчезнувшего смонтированного образа, а прежний pid передаётся отдельно.
func TestRestartEnvDropsAppImagePaths(t *testing.T) {
	sep := string(os.PathListSeparator)
	oldDir := "/tmp/.mount_Wynd"
	t.Setenv("APPDIR", oldDir)
	t.Setenv("APPIMAGE", "/tmp/Wynd.AppImage")
	t.Setenv("ARGV0", "/tmp/Wynd.AppImage")
	t.Setenv("OWD", "/home/mx")
	t.Setenv("PATH", oldDir+"/usr/bin"+sep+"/usr/bin"+sep+"/bin")
	t.Setenv("LD_LIBRARY_PATH", oldDir+"/usr/lib")
	t.Setenv("APPIMAGE_EXTRACT_AND_RUN", "1")
	t.Setenv(updateFromPIDEnv, "999")

	env := envMap(RestartEnv(4242))
	for _, name := range []string{"APPIMAGE", "APPDIR", "ARGV0", "OWD"} {
		if _, ok := env[name]; ok {
			t.Errorf("%s остался в окружении", name)
		}
	}
	if got, want := env["PATH"], "/usr/bin"+sep+"/bin"; got != want {
		t.Errorf("PATH: %q, ждали %q", got, want)
	}
	if _, ok := env["LD_LIBRARY_PATH"]; ok {
		t.Error("LD_LIBRARY_PATH со старым образом остался")
	}
	if got := env["APPIMAGE_EXTRACT_AND_RUN"]; got != "1" {
		t.Errorf("APPIMAGE_EXTRACT_AND_RUN: %q", got)
	}
	if got := env[updateFromPIDEnv]; got != "4242" {
		t.Errorf("%s: %q, ждали 4242", updateFromPIDEnv, got)
	}
}

func TestUpdateFromPID(t *testing.T) {
	t.Setenv(updateFromPIDEnv, "")
	if got := UpdateFromPID(); got != 0 {
		t.Fatalf("пустой pid: %d", got)
	}
	t.Setenv(updateFromPIDEnv, "321")
	if got := UpdateFromPID(); got != 321 {
		t.Fatalf("pid: %d", got)
	}
	t.Setenv(updateFromPIDEnv, "не число")
	if got := UpdateFromPID(); got != 0 {
		t.Fatalf("мусор в pid: %d", got)
	}
}
