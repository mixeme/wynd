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
	got := r.log[0].Output
	if !strings.Contains(got, "начало вывода не сохранено") || len(got) > maxLogOutput+128 {
		t.Fatalf("final output not trimmed to tail: len %d", len(got))
	}
}

// C71.1/C71.2: последняя непустая строка сборки идёт в шаг, мусор ANSI/CR
// убирается, а при молчании подпись говорит, сколько прошло.
func TestBuildLiveLogTracksLastLine(t *testing.T) {
	r := &Run{steps: []StepState{{ID: "image", Status: StepRunning}}, current: 0, report: func() {}}
	live := r.beginTracked("docker build --progress=plain")
	live.last = time.Now().Add(-time.Second)
	if _, err := live.Write([]byte("#1 [build 1/2] FROM golang\r\n")); err != nil {
		t.Fatal(err)
	}
	if got := r.steps[0].Latest; got != "#1 [build 1/2] FROM golang" {
		t.Fatalf("latest = %q", got)
	}
	if got := r.steps[0].LatestNote; got != "Последнее сообщение — сейчас" {
		t.Fatalf("note = %q", got)
	}

	live.last = time.Now().Add(-time.Second)
	if _, err := live.Write([]byte("\x1b[31m#2 ERROR: no space left on device\x1b[0m\n")); err != nil {
		t.Fatal(err)
	}
	if got := r.steps[0].Latest; got != "#2 ERROR: no space left on device" {
		t.Fatalf("latest after ANSI = %q", got)
	}

	live.mu.Lock()
	live.latestAt = time.Now().Add(-2 * time.Minute)
	live.mu.Unlock()
	live.publishLatest()
	if got := r.steps[0].LatestNote; got != "Последняя строка вывода — 2 минуты назад" {
		t.Fatalf("quiet note = %q", got)
	}
	live.finish("#2 ERROR: no space left on device")
}

// C71: сборка идёт без -q, BuildKit печатает строки; код ошибки остаётся в
// журнале, а не только в тексте отказа.
func TestBuildScriptStreamsOutput(t *testing.T) {
	script := buildScript("https://example.org/wynd", "0.49.0", "wynd:0.49.0")
	if strings.Contains(script, "docker build -q") || !strings.Contains(script, "--progress=plain") {
		t.Fatalf("build script hides output: %s", script)
	}
	if got := logOutput(strings.Repeat("y", maxLogOutput+20)); !strings.Contains(got, "начало вывода не сохранено") {
		t.Fatalf("lost prefix not marked: %q", got[:80])
	}
}

func TestImageStepTitleForSourceBuild(t *testing.T) {
	step := imageStep(Spec{Version: "0.49.0", Image: SourceImage{Repo: "https://example.org/wynd"}})
	if step.title != "Собираем образ Wynd" {
		t.Fatalf("title = %q", step.title)
	}
	step = imageStep(Spec{Version: "0.49.0", Image: RegistryImage{Repo: "ghcr.io/mixeme/wynd"}})
	if step.title != "Wynd 0.49.0" {
		t.Fatalf("registry title = %q", step.title)
	}
}

// Служебная строка кода ошибки встаёт в пустой журнал без ведущего перевода
// строки и не подменяет последнюю строку вывода в шаге.
func TestBuildErrorLineStartsCleanLog(t *testing.T) {
	r := &Run{steps: []StepState{{ID: "image", Status: StepRunning}}, current: 0, report: func() {}}
	live := r.beginTracked("docker build --progress=plain")
	live.finish("")
	live.appendLine("Сборка завершилась с кодом 1")
	if got := r.log[0].Output; got != "Сборка завершилась с кодом 1" {
		t.Fatalf("error line in log = %q", got)
	}
	if r.steps[0].Latest != "" {
		t.Fatalf("service line replaced build output: %+v", r.steps[0])
	}
}
