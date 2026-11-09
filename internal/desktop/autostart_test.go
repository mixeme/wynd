package desktop

import (
	"runtime"
	"strings"
	"testing"
)

func TestAutostartCommandQuotesExe(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("autostart is Windows-only")
	}
	command, err := AutostartCommand()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(command, `"`) || !strings.Contains(command, `" --from-startup`) {
		t.Fatalf("command must quote the exe and mark startup: %q", command)
	}
}

func TestAutostartStale(t *testing.T) {
	if autostartStale("", "now") {
		t.Fatal("empty record is not stale")
	}
	if autostartStale("old", "now") != true {
		t.Fatal("different command is stale")
	}
	if autostartStale("now", "now") {
		t.Fatal("same command is current")
	}
}
