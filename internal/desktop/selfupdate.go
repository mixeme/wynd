package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/releasepages"
	"gitea.mixdep.ru/mix/wynd/internal/relver"
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
// До 1.0.0 отлёжка снята (решение владельца 2026-11-26): выпуск предлагается
// сразу после публикации. Вернуть 72 * time.Hour при выходе 1.0.0.
var UpdateDelay = time.Duration(0)

// SigningKey — открытый ключ подписи выпусков (общий ключ экосистемы),
// base64; задаётся при сборке. Пусто — самообновление выключено.
var SigningKey = ""

// ErrBadSignature — подпись не сошлась: установка не начинается.
var ErrBadSignature = selfupdate.ErrBadSignature

// UpdateKind — чем обновляется эта сборка: exe Windows, AppImage или .deb.
// От этого зависит, какой файл выпуска скачивать и как его ставить.
type UpdateKind int

const (
	kindNone UpdateKind = iota
	kindExe
	kindAppImage
	kindDeb
)

// ErrNoUpdate — обновление не найдено для этой сборки (нет файла выпуска).
var ErrNoUpdate = errors.New("обновление для этой сборки не найдено")

// Update — что окно знает о новой версии.
type Update struct {
	Available   bool     `json:"available"`
	Version     string   `json:"version,omitempty"`
	Notes       []string `json:"notes,omitempty"`
	Attestation string   `json:"attestation,omitempty"`
	fileURL     string
	sigURL      string
	kind        UpdateKind
	target      string
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
	kind, target := DetectUpdateKind()
	if kind == kindNone {
		return Update{}
	}
	rels, err := updateReleases(ctx, client)
	if err != nil {
		return Update{}
	}
	return pickUpdateFor(rels, current, now, kind, target)
}

// detectUpdateKind — что подменять в этой сборке: AppImage, пакет .deb или
// exe. Отдельно от DetectUpdateKind, чтобы проверять без запуска образа.
func detectUpdateKind(goos, appImage, exe string, dpkgOwns func(string) bool) (UpdateKind, string) {
	if goos == "linux" {
		if appImage != "" {
			return kindAppImage, appImage
		}
		if exe != "" && dpkgOwns != nil && dpkgOwns(exe) {
			return kindDeb, exe
		}
		return kindNone, ""
	}
	if exe != "" {
		return kindExe, exe
	}
	return kindNone, ""
}

// updateReleases — страницы выпусков GitHub.
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

// matchesAsset — файл выпуска для этой сборки. Подписи сюда не попадают:
// «.sig» не оканчивается именем установочного файла.
func (k UpdateKind) matchesAsset(name string) bool {
	switch k {
	case kindExe:
		return name == desktopExe
	case kindAppImage:
		return strings.HasPrefix(name, "Wynd-") && strings.HasSuffix(name, "-x86_64.AppImage")
	case kindDeb:
		return strings.HasPrefix(name, "wynd-desktop_") && strings.HasSuffix(name, "_amd64.deb")
	}
	return false
}

// pickUpdate — выпуск для Windows-сборки: прежние тесты и поведение окна.
func pickUpdate(rels []ghRelease, current string, now time.Time) Update {
	return pickUpdateFor(rels, current, now, kindExe, "")
}

// pickUpdateFor — самый новый выпуск окна, вышедший не меньше UpdateDelay
// назад: черновики, пробные выпуски, чужие метки и версии не новее текущей не
// предлагаются. Для Linux берётся свой файл — AppImage или .deb.
func pickUpdateFor(rels []ghRelease, current string, now time.Time, kind UpdateKind, target string) Update {
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
		// Выпуск без файла для этой сборки не предлагаем: иначе окно
		// показало бы полосу и не смогло установить обновление.
		hasFile, hasSig := false, false
		for _, a := range r.Assets {
			if kind.matchesAsset(a.Name) {
				hasFile = true
			}
			if strings.HasSuffix(a.Name, ".sig") && kind.matchesAsset(strings.TrimSuffix(a.Name, ".sig")) {
				hasSig = true
			}
		}
		if !hasFile || !hasSig {
			continue
		}
		best, bestN = r, n
	}
	if best == nil {
		return Update{}
	}
	up := Update{
		Version:     strings.TrimPrefix(best.TagName, desktopTag),
		Attestation: AttestationsPage,
		kind:        kind,
		target:      target,
	}
	for _, a := range best.Assets {
		switch {
		case kind.matchesAsset(a.Name):
			up.fileURL = a.URL
		case strings.HasSuffix(a.Name, ".sig") && kind.matchesAsset(strings.TrimSuffix(a.Name, ".sig")):
			up.sigURL = a.URL
		}
	}
	for _, line := range strings.Split(best.Body, "\n") {
		if item, ok := strings.CutPrefix(strings.TrimSpace(line), "- "); ok {
			up.Notes = append(up.Notes, item)
		}
	}
	up.Available = up.fileURL != "" && up.sigURL != ""
	return up
}

// ApplyUpdate скачивает новую версию окна, проверяет подпись и ставит её на
// место своего файла; прежний остаётся рядом (.old) до следующего запуска.
// Windows — exe, Linux AppImage — сам .AppImage, .deb — установка через
// системный запрос прав (pkexec). Возвращает путь, который окно запускает и
// закрывается само. progress (необязательный) получает ход скачивания.
func ApplyUpdate(
	ctx context.Context,
	client *http.Client,
	up Update,
	progress func(done, total int64),
) (string, error) {
	if !up.Available {
		return "", ErrNoUpdate
	}
	switch up.kind {
	case kindExe:
		target := up.target
		if target == "" {
			target = SelfPath()
		}
		return selfupdate.ApplyTo(ctx, client, SigningKey, up.fileURL, up.sigURL, target, progress)
	case kindAppImage:
		target := up.target
		if target == "" {
			target = SelfPath()
		}
		return selfupdate.ApplyTo(ctx, client, SigningKey, up.fileURL, up.sigURL, target, progress)
	case kindDeb:
		return applyDeb(ctx, client, up, progress)
	}
	return "", ErrNoUpdate
}

// UpdateProgress — ход скачивания обновления (общий тип selfupdate.Progress).
type UpdateProgress = selfupdate.Progress

// CleanOldSelf убирает прежний файл после обновления (при запуске): у
// AppImage — рядом с самим .AppImage, а не с временным бинарником образа.
func CleanOldSelf() { selfupdate.CleanOldSelfAt(SelfPath()) }

// FreshlyReplaced — рядом с exe лежит прежний образ (.old): exe только что
// подменён самообновлением, и прежний процесс может ещё держать замок одного
// экземпляра. Окно ждёт его выхода до application.New: иначе новый запуск
// уходит в прежний и завершается, а полоса остаётся на «Обновляем…» (живая
// проверка 27.11, 0.5.2 → 0.5.5).
func FreshlyReplaced(exe string) bool {
	if exe == "" {
		return false
	}
	_, err := os.Stat(exe + ".old")
	return err == nil
}

// NotifyUpdateText — текст уведомления о новой версии окна (план 66, «вышла
// версия»); окно спрятано, и полосу человек не видит. ok=false — уведомление
// об этой версии уже показывали или обновления нет: дедуп по версии.
func NotifyUpdateText(up Update, notified string) (title, body string, ok bool) {
	if !up.Available || up.Version == "" || up.Version == notified {
		return "", "", false
	}
	return "Вышла версия " + up.Version, "Откройте Wynd Настольный, чтобы обновиться.", true
}
