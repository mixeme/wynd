package desktop

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNormalizeBackupConfig(t *testing.T) {
	base := DefaultBackupConfig()
	base.Enabled = true
	base.Folder = ` C:\Копии `
	cfg, err := NormalizeBackupConfig(base)
	if err != nil || cfg.Folder != `C:\Копии` {
		t.Fatalf("нормальные значения: %+v, %v", cfg, err)
	}
	cases := []struct {
		name   string
		mutate func(*BackupConfig)
	}{
		{"расписание", func(c *BackupConfig) { c.Schedule = "hourly" }},
		{"свой интервал 0", func(c *BackupConfig) { c.Schedule = ScheduleCustom; c.CustomDays = 0 }},
		{"свой интервал 366", func(c *BackupConfig) { c.Schedule = ScheduleCustom; c.CustomDays = 366 }},
		{"хранение 0", func(c *BackupConfig) { c.Keep = 0 }},
		{"хранение-дни 4000", func(c *BackupConfig) { c.KeepMode = KeepDays; c.Keep = 4000 }},
		{"режим хранения", func(c *BackupConfig) { c.KeepMode = "forever" }},
		{"папка пуста", func(c *BackupConfig) { c.Folder = "  " }},
	}
	for _, c := range cases {
		cfg := base
		c.mutate(&cfg)
		if _, err := NormalizeBackupConfig(cfg); err == nil {
			t.Errorf("%s: принято", c.name)
		}
	}
	// Выключенным копиям папка не нужна.
	off := base
	off.Enabled = false
	off.Folder = ""
	if _, err := NormalizeBackupConfig(off); err != nil {
		t.Errorf("выключено без папки: %v", err)
	}
}

func TestBackupDue(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	cfg := DefaultBackupConfig() // daily
	cfg.Enabled = true
	if !BackupDue(time.Time{}, cfg, now) {
		t.Fatal("первая копия должна собираться сразу")
	}
	if BackupDue(now.Add(-23*time.Hour), cfg, now) {
		t.Fatal("суточная копия собрана раньше срока")
	}
	if !BackupDue(now.Add(-24*time.Hour), cfg, now) {
		t.Fatal("суточная копия не собрана по сроку")
	}
	cfg.Schedule = ScheduleWeekly
	if BackupDue(now.AddDate(0, 0, -6), cfg, now) {
		t.Fatal("недельная копия собрана раньше срока")
	}
	if !BackupDue(now.AddDate(0, 0, -7), cfg, now) {
		t.Fatal("недельная копия не собрана по сроку")
	}
	cfg.Schedule = ScheduleMonthly
	last := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	dueAt := last.AddDate(0, 1, 0) // 23.11 12:00 — ровно месяц
	if BackupDue(last, cfg, dueAt.Add(-time.Hour)) {
		t.Fatal("месячная копия собрана раньше срока")
	}
	if !BackupDue(last, cfg, dueAt) {
		t.Fatal("месячная копия не собрана через месяц")
	}
	cfg.Schedule = ScheduleCustom
	cfg.CustomDays = 3
	if BackupDue(now.AddDate(0, 0, -2), cfg, now) {
		t.Fatal("свой интервал собран раньше срока")
	}
	if !BackupDue(now.AddDate(0, 0, -3), cfg, now) {
		t.Fatal("свой интервал не собран по сроку")
	}
	cfg.Enabled = false
	if BackupDue(time.Time{}, cfg, now) {
		t.Fatal("выключенные копии собраны")
	}
}

func TestBackupFileName(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	if got := backupFileName(dir, "Дача", day); got != "Дача 2026-11-23.zip" {
		t.Fatalf("имя: %q", got)
	}
	if got := backupFileName(dir, `a/b:c*?"<>|`, day); got != "a_b_c______ 2026-11-23.zip" {
		t.Fatalf("санитизация: %q", got)
	}
	if got := backupFileName(dir, "   ", day); got != "круг 2026-11-23.zip" {
		t.Fatalf("пустое имя: %q", got)
	}
	// Занятое имя — суффикс, а не перезапись.
	for _, name := range []string{"Дача 2026-11-23.zip", "Дача 2026-11-23 (2).zip"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if got := backupFileName(dir, "Дача", day); got != "Дача 2026-11-23 (3).zip" {
		t.Fatalf("суффикс: %q", got)
	}
}

func TestPruneBackups(t *testing.T) {
	dir := t.TempDir()
	day := func(d int) string { return time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC).Format("2006-01-02") }
	write := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{
		"Дача " + day(18) + ".zip",
		"Дача " + day(19) + ".zip",
		"Дача " + day(20) + ".zip",
		"Дача " + day(21) + ".zip",
		"Семья " + day(21) + ".zip",
		"чужой.txt",
	} {
		write(name)
	}
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)

	// По количеству: 2 новейших на круг, чужие файлы не трогаем.
	cfg := DefaultBackupConfig()
	cfg.Keep = 2
	removed, err := pruneBackups(dir, cfg, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 2 {
		t.Fatalf("удалено: %v", removed)
	}
	entries, _ := os.ReadDir(dir)
	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name()] = true
	}
	for _, want := range []string{"Дача " + day(20) + ".zip", "Дача " + day(21) + ".zip", "Семья " + day(21) + ".zip", "чужой.txt"} {
		if !names[want] {
			t.Fatalf("пропал %s, остались %v", want, names)
		}
	}

	// По сроку: старше 2 дней (21-е и новее остаются при today=23-е).
	cfg.KeepMode = KeepDays
	cfg.Keep = 2
	removed, err = pruneBackups(dir, cfg, now)
	if err != nil {
		t.Fatal(err)
	}
	// 20-е осталось от предыдущего шага — старше 23-2=21-го, его убираем.
	if len(removed) != 1 || removed[0] != "Дача "+day(20)+".zip" {
		t.Fatalf("по сроку: %v", removed)
	}
}

func TestRunBackupOnce(t *testing.T) {
	archiveBytes := []byte("PK\x03\x04 фейковый архив")
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		switch {
		case r.URL.Path == "/api/v1/circles":
			_, _ = io.WriteString(w, `{"circles":[{"id":"c1","name":"Дача"},{"id":"c2","name":"Семья"}]}`)
		case strings.HasPrefix(r.URL.Path, "/api/v1/circles/") && strings.HasSuffix(r.URL.Path, "/archive/download"):
			_, _ = w.Write(archiveBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfg := DefaultBackupConfig()
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	saved, err := RunBackupOnce(context.Background(), srv.Client(), srv.URL, "tok1", dir, cfg, now)
	if err != nil || saved != 2 {
		t.Fatalf("собрано %d, err=%v", saved, err)
	}
	if gotAuth != "Bearer tok1" {
		t.Fatalf("токен: %q", gotAuth)
	}
	for _, name := range []string{"Дача 2026-11-23.zip", "Семья 2026-11-23.zip"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(data) != string(archiveBytes) {
			t.Fatalf("%s: %v %q", name, err, data)
		}
	}
	// Повторный прогон в тот же день не перезаписывает, а добавляет (2).
	saved, err = RunBackupOnce(context.Background(), srv.Client(), srv.URL, "tok1", dir, cfg, now)
	if err != nil || saved != 2 {
		t.Fatalf("повтор: %d, %v", saved, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Дача 2026-11-23 (2).zip")); err != nil {
		t.Fatalf("повторная копия: %v", err)
	}
}
