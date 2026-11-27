//go:build linux

package desktop

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// .deb ставится из временного файла: установщик получает проверенные байты,
// а временный файл после установки убирается.
func TestInstallDeb(t *testing.T) {
	var seen string
	target := "/usr/bin/wynd-desktop"
	got, err := installDeb(context.Background(), []byte("deb-bytes"), target, func(_ context.Context, path string) error {
		seen = path
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if string(body) != "deb-bytes" {
			return errors.New("установщик получил не тот файл")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != target {
		t.Fatalf("путь после установки: %q, ждали %q", got, target)
	}
	if seen == "" {
		t.Fatal("установщик не вызван")
	}
	if _, err := os.Stat(seen); !os.IsNotExist(err) {
		t.Fatalf("временный .deb остался: %v", err)
	}
}

func TestInstallDebError(t *testing.T) {
	want := errors.New("pkexec отменён")
	if _, err := installDeb(context.Background(), []byte("deb"), "/usr/bin/wynd-desktop", func(context.Context, string) error {
		return want
	}); !errors.Is(err, want) {
		t.Fatalf("ошибка установщика потерялась: %v", err)
	}
}

// Новый процесс ждёт выхода прежнего, но не дольше срока: иначе Wails отдаст
// управление прежнему и новый закроется (C153).
func TestWaitForProcessExit(t *testing.T) {
	short := exec.Command("sleep", "0.1")
	if err := short.Start(); err != nil {
		t.Fatal(err)
	}
	pid := short.Process.Pid
	// Дождаться надо и забрать код выхода: убитый процесс-зомби всё ещё
	// виден kill(pid, 0), и ожидание не закончится.
	done := make(chan error, 1)
	go func() { done <- short.Wait() }()
	if !WaitForProcessExit(pid, 5*time.Second) {
		t.Fatal("короткий процесс не дождались")
	}
	<-done

	long := exec.Command("sleep", "5")
	if err := long.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = long.Process.Kill()
		_, _ = long.Process.Wait()
	}()
	if WaitForProcessExit(long.Process.Pid, 50*time.Millisecond) {
		t.Fatal("долгий процесс не должен дожидаться за 50 мс")
	}
}

func TestDebInstallCommand(t *testing.T) {
	if _, err := exec.LookPath("pkexec"); err != nil {
		t.Skip("pkexec не установлен")
	}
	cmd, err := debInstallCommand(context.Background(), "/tmp/up.deb")
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(cmd.Args, " ")
	if !strings.Contains(args, "dpkg") || !strings.Contains(args, "-i /tmp/up.deb") {
		t.Fatalf("команда установки: %q", args)
	}
}
