//go:build linux

package desktop

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gitea.mixdep.ru/mix/wynd/internal/selfupdate"
)

// dpkgList — список файлов установленного пакета .deb. Если в нём есть наш
// exe, сборка пришла из пакета: обновляемся .deb, а не подменой файла.
const dpkgList = "/var/lib/dpkg/info/wynd-desktop.list"

func dpkgOwns(exe string) bool {
	body, err := os.ReadFile(dpkgList)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(body), "\n") {
		if strings.TrimSpace(line) == exe {
			return true
		}
	}
	return false
}

// DetectUpdateKind — чем обновляется эта сборка: AppImage, пакет .deb или
// ничем (локальная сборка без установки). Символические ссылки у AppImage
// раскрываются: подменять надо настоящий файл, а не ссылку на него.
func DetectUpdateKind() (UpdateKind, string) {
	exe, _ := os.Executable()
	appImage := os.Getenv("APPIMAGE")
	if appImage != "" {
		if resolved, err := filepath.EvalSymlinks(appImage); err == nil {
			appImage = resolved
		}
	}
	return detectUpdateKind("linux", appImage, exe, dpkgOwns)
}

// applyDeb скачивает .deb, проверяет подпись и ставит пакет через pkexec:
// подмена /usr/bin/wynd-desktop обычным пользователем невозможна.
func applyDeb(
	ctx context.Context,
	client *http.Client,
	up Update,
	progress func(done, total int64),
) (string, error) {
	file, err := selfupdate.FetchVerified(ctx, client, SigningKey, up.fileURL, up.sigURL, progress)
	if err != nil {
		return "", err
	}
	target := up.target
	if target == "" {
		target = SelfPath()
	}
	return installDeb(ctx, file, target, runPkexecDeb)
}

// installDeb пишет проверенный файл во временный и отдаёт установщику.
// install отделён, чтобы проверять без настоящего pkexec.
func installDeb(
	ctx context.Context,
	file []byte,
	target string,
	install func(context.Context, string) error,
) (string, error) {
	tmp, err := os.CreateTemp("", "wynd-desktop-*.deb")
	if err != nil {
		return "", err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(file); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := install(ctx, name); err != nil {
		return "", err
	}
	return target, nil
}

// runPkexecDeb — системный запрос прав: pkexec показывает окно пароля и
// запускает dpkg -i от администратора.
func runPkexecDeb(ctx context.Context, deb string) error {
	cmd, err := debInstallCommand(ctx, deb)
	if err != nil {
		return err
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			switch exit.ExitCode() {
			case 126, 127:
				return errors.New("установка отменена")
			}
		}
		if msg := lastLine(string(out)); msg != "" {
			return fmt.Errorf("не удалось установить обновление: %s", msg)
		}
		return fmt.Errorf("не удалось установить обновление: %w", err)
	}
	return nil
}

// debInstallCommand — что именно запускается: pkexec <dpkg> -i <файл>.
func debInstallCommand(ctx context.Context, deb string) (*exec.Cmd, error) {
	pkexec, err := exec.LookPath("pkexec")
	if err != nil {
		return nil, errors.New("для установки .deb нужен pkexec (PolicyKit)")
	}
	dpkg, err := exec.LookPath("dpkg")
	if err != nil {
		return nil, errors.New("не найден dpkg")
	}
	return exec.CommandContext(ctx, pkexec, dpkg, "-i", deb), nil
}

// lastLine — последняя непустая строка вывода, чтобы ошибка помещалась в
// полосу обновления.
func lastLine(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if runes := []rune(line); len(runes) > 300 {
			line = string(runes[:300]) + "…"
		}
		return line
	}
	return ""
}
