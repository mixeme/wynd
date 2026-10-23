package installer

import (
	"strings"
	"testing"
	"time"
)

// Команда видна в журнале, пока идёт: сразу при запуске и с выводом по ходу.
// Раньше она попадала туда только в конце — пока Docker ставился минутами,
// «показать, что выполняется» было пустым (2026-10-23).
func TestLiveLogShowsRunningCommand(t *testing.T) {
	reports := 0
	r := &Run{report: func() { reports++ }}

	live := r.begin("apt-get install -y docker-ce")
	if len(r.log) != 1 || r.log[0].Command != "apt-get install -y docker-ce" || r.log[0].Output != "" {
		t.Fatalf("command not in log at start: %+v", r.log)
	}
	if reports != 1 {
		t.Fatalf("reports after begin = %d, want 1", reports)
	}

	live.last = time.Now().Add(-time.Second)
	if _, err := live.Write([]byte("Unpacking docker-ce …\n")); err != nil {
		t.Fatal(err)
	}
	if got := r.log[0].Output; got != "Unpacking docker-ce …" {
		t.Fatalf("live output = %q", got)
	}

	// Частые куски — без нового отчёта, пока не прошло liveReport.
	before := reports
	_, _ = live.Write([]byte("Setting up docker-ce\n"))
	if reports != before {
		t.Fatalf("reported too often: %d → %d", before, reports)
	}

	live.finish(strings.Repeat("x", maxLogOutput+10))
	if got := r.log[0].Output; !strings.HasPrefix(got, "…\n") || len(got) != maxLogOutput+len("…\n") {
		t.Fatalf("final output not trimmed to tail: len %d", len(got))
	}
}
