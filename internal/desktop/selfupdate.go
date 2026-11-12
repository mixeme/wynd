package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/relver"
	"gitea.mixdep.ru/mix/wynd/internal/releasepages"
	"gitea.mixdep.ru/mix/wynd/internal/selfupdate"
)

// Самообновление Wynd Настольного (план 63) — по модели Мастерской: не молча,
// не сразу, только по подписи. Подпись — общим ключом экосистемы (Ed25519):
// открытая часть зашита в сборку (SigningKey), закрытая у владельца, не на
// GitHub. Без подписи не ставим.

const (
	// ReleasesURL — выпуски программ Wynd на GitHub.
	ReleasesURL = "https://api.github.com/repos/mixeme/wynd/releases?per_page=100"
	// ReleasesPage — страница выпусков для человека.
	ReleasesPage = "https://github.com/mixeme/wynd/releases"
	// AttestationsPage — удостоверения сборки: куда ведёт «проверить».
	AttestationsPage = "https://github.com/mixeme/wynd/attestations"
	// desktopTag — метка выпуска окна: «desktop-v0.1.1».
	desktopTag = "desktop-v"
	// desktopExe — файл выпуска, который ставится на место своего exe.
	desktopExe = "wynd-desktop.exe"
)

// UpdateDelay — через сколько после выхода предлагать: подменённый выпуск
// успеют заметить (то же решение, что у Мастерской).
var UpdateDelay = 72 * time.Hour

// SigningKey — открытый ключ подписи выпусков (общий ключ экосистемы),
// base64; задаётся при сборке. Пусто — самообновление выключено.
var SigningKey = ""

// ErrBadSignature — подпись не сошлась: установка не начинается.
var ErrBadSignature = selfupdate.ErrBadSignature

// Update — что окно знает о новой версии.
type Update struct {
	Available   bool     `json:"available"`
	Version     string   `json:"version,omitempty"`
	Notes       []string `json:"notes,omitempty"`
	Attestation string   `json:"attestation,omitempty"`
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

// CheckUpdate спрашивает GitHub, есть ли окно новее current.
func CheckUpdate(ctx context.Context, client *http.Client, current string, now time.Time) Update {
	if SigningKey == "" {
		return Update{}
	}
	rels, err := updateReleases(ctx, client)
	if err != nil {
		return Update{}
	}
	return pickUpdate(rels, current, now)
}

func updateReleases(ctx context.Context, client *http.Client) ([]ghRelease, error) {
	return releasepages.Collect(ctx, func(page int) ([]ghRelease, bool, error) {
		// Страницы собираем на фиксированном адресе API, чужие Link не ходим.
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s&page=%d", ReleasesURL, page), nil)
		if err != nil {
			return nil, false, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		res, err := client.Do(req)
		if err != nil {
			return nil, false, err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return nil, false, fmt.Errorf("GitHub releases: HTTP %d", res.StatusCode)
		}
		var rows []ghRelease
		if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&rows); err != nil {
			return nil, false, err
		}
		return rows, strings.Contains(res.Header.Get("Link"), `rel="next"`), nil
	})
}

// pickUpdate — самый новый выпуск окна, вышедший не меньше UpdateDelay назад:
// черновики, пробные выпуски, чужие метки и версии не новее текущей не
// предлагаются.
func pickUpdate(rels []ghRelease, current string, now time.Time) Update {
	cur, _ := relver.Number(current)
	var best *ghRelease
	var bestN [3]int
	for i := range rels {
		r := &rels[i]
		v, ok := strings.CutPrefix(r.TagName, desktopTag)
		if !ok || r.Draft || r.Prerelease || now.Sub(r.PublishedAt) < UpdateDelay {
			continue
		}
		n, ok := relver.Number(v)
		if !ok || !relver.Less(cur, n) || best != nil && !relver.Less(bestN, n) {
			continue
		}
		best, bestN = r, n
	}
	if best == nil {
		return Update{}
	}
	up := Update{Version: strings.TrimPrefix(best.TagName, desktopTag), Attestation: AttestationsPage}
	for _, a := range best.Assets {
		switch a.Name {
		case desktopExe:
			up.exeURL = a.URL
		case desktopExe + ".sig":
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

// ApplyUpdate скачивает новую версию окна, проверяет подпись и ставит её на
// место своего exe; прежний остаётся рядом (.old) до следующего запуска.
// Возвращает путь нового exe — его окно запускает и закрывается само.
func ApplyUpdate(ctx context.Context, client *http.Client, up Update) (string, error) {
	if !up.Available {
		return "", ErrBadSignature
	}
	return selfupdate.Apply(ctx, client, SigningKey, up.exeURL, up.sigURL)
}

// CleanOldSelf убирает прежний exe после обновления (при запуске).
func CleanOldSelf() { selfupdate.CleanOldSelf() }
