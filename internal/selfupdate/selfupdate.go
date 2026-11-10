// Package selfupdate — общая часть самообновления программ Wynd: проверка
// подписи выпуска общим ключом экосистемы и подмена собственного exe
// скачанным. Мастерская (internal/installer) и окно (internal/desktop) берут
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
func Apply(ctx context.Context, client *http.Client, key, exeURL, sigURL string) (string, error) {
	if key == "" || exeURL == "" || sigURL == "" {
		return "", ErrBadSignature
	}
	exe, err := fetch(ctx, client, exeURL, 200<<20)
	if err != nil {
		return "", err
	}
	sig, err := fetch(ctx, client, sigURL, 4<<10)
	if err != nil {
		return "", err
	}
	if err := VerifySignature(key, exe, sig); err != nil {
		return "", err
	}
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	next := self + ".new"
	if err := os.WriteFile(next, exe, 0o755); err != nil {
		return "", err
	}
	old := self + ".old"
	_ = os.Remove(old)
	// Работающий exe в Windows переименовать можно, удалить — нет.
	if err := os.Rename(self, old); err != nil {
		os.Remove(next)
		return "", err
	}
	if err := os.Rename(next, self); err != nil {
		_ = os.Rename(old, self)
		return "", err
	}
	return self, nil
}

// CleanOldSelf убирает прежний exe после обновления (при запуске).
func CleanOldSelf() {
	if self, err := os.Executable(); err == nil {
		_ = os.Remove(filepath.Clean(self + ".old"))
	}
}

func fetch(ctx context.Context, client *http.Client, url string, limit int64) ([]byte, error) {
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
	return io.ReadAll(io.LimitReader(res.Body, limit))
}
