//go:build linux

package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutostartLinuxXDGFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	status, err := SetAutostart(true)
	if err != nil {
		t.Fatalf("SetAutostart(true): %v", err)
	}
	if !status.Enabled || status.Stale {
		t.Fatalf("после включения: %+v", status)
	}
	path, err := autostartFile()
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "Exec="+status.Command) ||
		!strings.Contains(string(body), "X-GNOME-Autostart-enabled=true") {
		t.Fatalf("файл автозапуска: %s", body)
	}

	read, err := AutostartState()
	if err != nil {
		t.Fatalf("AutostartState: %v", err)
	}
	if !read.Enabled || read.Stale || read.Command != status.Command {
		t.Fatalf("прочитанное состояние: %+v", read)
	}

	if _, err := SetAutostart(false); err != nil {
		t.Fatalf("SetAutostart(false): %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("файл автозапуска остался: %v", err)
	}
	read, err = AutostartState()
	if err != nil || read.Enabled {
		t.Fatalf("после выключения: %+v, %v", read, err)
	}
}

func TestSchemeLinuxDesktopEntry(t *testing.T) {
	entry := schemeDesktopEntry("/opt/Wynd/wynd-desktop")
	for _, want := range []string{
		"MimeType=x-scheme-handler/wynd;",
		`Exec="/opt/Wynd/wynd-desktop" %u`,
		"Type=Application",
	} {
		if !strings.Contains(entry, want) {
			t.Errorf("в .desktop нет %q:\n%s", want, entry)
		}
	}
}

func TestEnsureInviteSchemeLinux(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)

	if err := EnsureInviteScheme(); err != nil {
		t.Fatalf("EnsureInviteScheme: %v", err)
	}
	path := filepath.Join(data, "applications", schemeFileName)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "x-scheme-handler/wynd") {
		t.Fatalf(".desktop без схемы: %s", body)
	}
	command, err := InviteSchemeCommand()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(command, "%u") {
		t.Fatalf("команда обработчика: %q", command)
	}
}
