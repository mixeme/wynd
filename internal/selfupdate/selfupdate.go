// Package selfupdate — общая часть самообновления программ Wynd: проверка
// подписи выпуска общим ключом экосистемы и подмена собственного exe
// скачанным. Мастерская (internal/workshop) и окно (internal/desktop) берут
// это отсюда, чтобы проверка подписи и порядок подмены были одни на все
// программы.
package selfupdate

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ErrBadSignature — подпись не сошлась (кадр 10в у Мастерской, у окна — тот же
// отказ): установка не начинается.
var ErrBadSignature = errors.New("selfupdate: release signature does not match")

// Progress — ход скачивания обновления: байты и известный размер (Total = 0 —
// размер неизвестен). Общий для окна и Мастерской: обе полосы показывают ход.
type Progress struct {
	Done  int64 `json:"done"`
	Total int64 `json:"total"`
}

// VerifySignature проверяет подпись выпуска ключом key (base64).
func VerifySignature(key string, exe, sig []byte) error {
	pub, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return ErrBadSignature
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sig)))
	if err != nil || !ed25519.Verify(ed25519.PublicKey(pub), exe, raw) {
		return ErrBadSignature
	}
	return nil
}

// Apply скачивает новую программу и её подпись, проверяет подпись и ставит
// программу на место своего exe; прежний остаётся рядом (.old) до следующего
// запуска. Возвращает путь нового exe — его окно запускает и закрывается само.
// progress (необязательный) получает ход скачивания: сколько байт уже есть и
// известный размер (0 — размер неизвестен).
func Apply(
	ctx context.Context,
	client *http.Client,
	key, exeURL, sigURL string,
	progress func(done, total int64),
) (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	return ApplyTo(ctx, client, key, exeURL, sigURL, self, progress)
}

// ApplyTo — то же, но подменяет заданный файл: у Linux-окна это AppImage
// (os.Executable() внутри образа указывает на временный бинарник). Прежний
// файл остаётся рядом (.old) до следующего запуска.
func ApplyTo(
	ctx context.Context,
	client *http.Client,
	key, fileURL, sigURL, target string,
	progress func(done, total int64),
) (string, error) {
	if target == "" {
		return "", fmt.Errorf("selfupdate: пустой путь установки")
	}
	file, err := FetchVerified(ctx, client, key, fileURL, sigURL, progress)
	if err != nil {
		return "", err
	}
	next := target + ".new"
	if err := os.WriteFile(next, file, 0o755); err != nil {
		return "", err
	}
	old := target + ".old"
	_ = os.Remove(old)
	// Работающий exe в Windows переименовать можно, удалить — нет; на Linux
	// так же подменяется работающий AppImage.
	if err := os.Rename(target, old); err != nil {
		os.Remove(next)
		return "", err
	}
	if err := os.Rename(next, target); err != nil {
		_ = os.Rename(old, target)
		return "", err
	}
	return target, nil
}

// FetchVerified скачивает файл выпуска и проверяет его подпись общим ключом.
// Для .deb файл не подменяет программу, а отдаётся установщику.
func FetchVerified(
	ctx context.Context,
	client *http.Client,
	key, fileURL, sigURL string,
	progress func(done, total int64),
) ([]byte, error) {
	if key == "" || fileURL == "" || sigURL == "" {
		return nil, ErrBadSignature
	}
	file, err := fetch(ctx, client, fileURL, 200<<20, progress)
	if err != nil {
		return nil, err
	}
	sig, err := fetch(ctx, client, sigURL, 4<<10, nil)
	if err != nil {
		return nil, err
	}
	if err := VerifySignature(key, file, sig); err != nil {
		return nil, err
	}
	return file, nil
}

// CleanOldSelf убирает прежний exe после обновления (при запуске).
func CleanOldSelf() {
	if self, err := os.Executable(); err == nil {
		CleanOldSelfAt(self)
	}
}

// CleanOldSelfAt убирает прежний файл по пути, который запускает человек:
// у Linux-окна это AppImage, а не временный бинарник внутри образа.
func CleanOldSelfAt(self string) {
	if self == "" {
		return
	}
	_ = os.Remove(filepath.Clean(self + ".old"))
}

func fetch(
	ctx context.Context,
	client *http.Client,
	url string,
	limit int64,
	progress func(done, total int64),
) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("selfupdate: %s: %s", url, res.Status)
	}
	var body io.Reader = io.LimitReader(res.Body, limit)
	if progress != nil {
		body = &progressReader{r: body, total: res.ContentLength, report: progress}
	}
	return io.ReadAll(body)
}

// progressReader сообщает, сколько байт уже прочитано: полоса обновления
// показывает ход скачивания, а не молчит до конца.
type progressReader struct {
	r      io.Reader
	total  int64
	done   int64
	report func(done, total int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.done += int64(n)
		p.report(p.done, p.total)
	}
	return n, err
}
