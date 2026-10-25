package installer

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Самообновление Мастерской (план 45; кадры 10а–10в). Не молча: Мастерская
// только сообщает о новой версии, обновляет человек. Не сразу: выпуск
// предлагается через UpdateDelay после выхода — подменённый успеют заметить.
// Подпись — ключом владельца (Ed25519); открытая часть зашита в сборку
// (SigningKey), закрытая у владельца, не на GitHub. Без подписи не ставим.

// ReleasesURL — выпуски Мастерской: GitHub Releases открытого репозитория.
const ReleasesURL = "https://api.github.com/repos/mixeme/wynd/releases?per_page=30"

// ReleasesPage — страница выпусков для человека (10в, 7ж2).
const ReleasesPage = "https://github.com/mixeme/wynd/releases"

// workshopTag — метка выпуска Мастерской: «workshop-v0.2.0».
const workshopTag = "workshop-v"

// UpdateDelay — через сколько после выхода предлагать (решение 2 плана 45:
// «через несколько дней»; три дня — мой выбор, не замер).
var UpdateDelay = 72 * time.Hour

// SigningKey — открытый ключ подписи выпусков, base64; задаётся при сборке
// (-ldflags "-X …SigningKey=…"). Пусто — самообновление выключено.
var SigningKey = ""

// SelfUpdate — что знает окно о новой Мастерской.
type SelfUpdate struct {
	Available bool     `json:"available"`
	Version   string   `json:"version,omitempty"`
	Notes     []string `json:"notes,omitempty"`
	// Attestation — страница удостоверения сборки на GitHub (10б, «проверить»).
	Attestation string `json:"attestation,omitempty"`
	exeURL      string
	sigURL      string
}

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Body        string    `json:"body"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Assets      []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// CheckSelfUpdate спрашивает GitHub, есть ли Мастерская новее current.
func CheckSelfUpdate(ctx context.Context, client *http.Client, current string, now time.Time) SelfUpdate {
	if SigningKey == "" {
		return SelfUpdate{}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ReleasesURL, nil)
	if err != nil {
		return SelfUpdate{}
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := client.Do(req)
	if err != nil {
		return SelfUpdate{}
	}
	defer res.Body.Close()
	var rels []ghRelease
	if res.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&rels) != nil {
		return SelfUpdate{}
	}
	return pickRelease(rels, current, now)
}

func pickRelease(rels []ghRelease, current string, now time.Time) SelfUpdate {
	cur, _ := releaseNumber(current)
	var best *ghRelease
	var bestN [3]int
	for i := range rels {
		r := &rels[i]
		v, ok := strings.CutPrefix(r.TagName, workshopTag)
		if !ok || r.Draft || r.Prerelease || now.Sub(r.PublishedAt) < UpdateDelay {
			continue
		}
		n, ok := releaseNumber(v)
		if !ok || !versionLess(cur, n) || best != nil && !versionLess(bestN, n) {
			continue
		}
		best, bestN = r, n
	}
	if best == nil {
		return SelfUpdate{}
	}
	up := SelfUpdate{Version: strings.TrimPrefix(best.TagName, workshopTag), Attestation: ReleasesPage + "/tag/" + best.TagName}
	for _, a := range best.Assets {
		switch a.Name {
		case "wynd-workshop.exe":
			up.exeURL = a.URL
		case "wynd-workshop.exe.sig":
			up.sigURL = a.URL
		}
	}
	for _, line := range strings.Split(best.Body, "\n") {
		if item, ok := strings.CutPrefix(strings.TrimSpace(line), "- "); ok {
			up.Notes = append(up.Notes, item)
		}
	}
	up.Available = up.exeURL != "" && up.sigURL != ""
	return up
}

// ErrBadSignature — подпись не сошлась (10в).
var ErrBadSignature = errors.New("installer: workshop update signature does not match")

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

// ApplySelfUpdate скачивает новую Мастерскую, проверяет подпись и ставит её
// на место своего exe; прежний остаётся рядом (.old) до следующего запуска.
// Возвращает путь нового exe — его окно запускает и закрывается само.
func ApplySelfUpdate(ctx context.Context, client *http.Client, up SelfUpdate) (string, error) {
	if !up.Available {
		return "", ErrBadSignature
	}
	exe, err := fetch(ctx, client, up.exeURL, 200<<20)
	if err != nil {
		return "", err
	}
	sig, err := fetch(ctx, client, up.sigURL, 4<<10)
	if err != nil {
		return "", err
	}
	if err := VerifySignature(SigningKey, exe, sig); err != nil {
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
		return nil, fmt.Errorf("installer: %s: %s", url, res.Status)
	}
	return io.ReadAll(io.LimitReader(res.Body, limit))
}
