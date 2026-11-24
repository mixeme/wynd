package signing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"gitea.mixdep.ru/mix/wynd/internal/releasepages"
)

// Выпуски программ Wynd на GitHub (кадры 12б–12г): каждая программа
// собирается своим workflow из своего тега (Мастерская — workshop.yaml,
// Wynd Настольный — desktop-client.yaml). Говорим с GitHub через gh
// (GitHub CLI) — как scripts\sign-workshop.bat.

// Подписываемые программы и их выпуски в mixeme/wynd. Новая программа
// экосистемы — строка в products; окно берёт имя, exe и особенности из неё.
const (
	// ProductWorkshop — Мастерская: выпуск workshop-vX.Y.Z, wynd-workshop.exe.
	ProductWorkshop = "workshop"
	// ProductDesktop — Wynd Настольный: выпуск desktop-vX.Y.Z, wynd-desktop.exe.
	ProductDesktop = "desktop"
	// ProductAndroid — Wynd Android: выпуск android-vX.Y.Z, wynd-android.apk
	// (план 65; подпись apksigner, шаги — в android.go).
	ProductAndroid = "android"
)

type product struct {
	ID     string // ProductWorkshop / ProductDesktop / ProductAndroid — уходит в окно как product.
	Name   string // как программа называется в окне.
	Exe    string // имя файла выпуска.
	Prefix string // префикс тега выпуска.
	// Key — открытый ключ, вшитый в сборку программы (общий ключ экосистемы):
	// по нему программа проверяет самообновление. Пусто — самообновления нет.
	Key string
	// Notes — «Что нового» в репозитории тега; раздел версии публикуется
	// вместе с выпуском. Пусто — описание не читается.
	Notes string
	// Unsigned — имя файла черновика для скачивания (Android); пусто — Exe.
	Unsigned string
	// SignedAsset — файл-признак «выпуск подписан» в списке; пусто — Exe+".sig".
	SignedAsset string
	// KeyAlias — alias в keystore Android; пусто — подпись exe-ключом из файла.
	KeyAlias string
	// APK — Android-выпуск: свои шаги проверки и подпись apksigner.
	APK bool
	// Extras — дополнительные файлы выпуска, подписываемые тем же ключом
	// (Linux .deb и AppImage): шаблоны имён в выпуске.
	Extras []string
}

var products = []product{
	{ID: ProductWorkshop, Name: "Wynd Мастерская", Exe: "wynd-workshop.exe", Prefix: "workshop-v", Key: "signing-key.pub", Notes: "cmd/wynd-workshop/whatsnew.md"},
	{ID: ProductDesktop, Name: "Wynd Настольный", Exe: "wynd-desktop.exe", Prefix: "desktop-v", Key: "signing-key.pub", Notes: "cmd/wynd-desktop/whatsnew.md", Extras: []string{"wynd-desktop_*_amd64.deb", "Wynd-*-x86_64.AppImage"}},
	{ID: ProductAndroid, Name: "Wynd Android", Exe: "wynd-android.apk", Unsigned: "wynd-android-unsigned.apk", SignedAsset: "wynd-android.apk", Prefix: "android-v", Notes: "android/whatsnew.md", KeyAlias: androidKeyAlias, APK: true},
}

// downloadName — имя файла, который скачивают из выпуска: у Android это
// unsigned APK черновика, у остальных — сам exe.
func (p product) downloadName() string {
	if p.Unsigned != "" {
		return p.Unsigned
	}
	return p.Exe
}

// signedAsset — файл, по наличию которого выпуск считается подписанным.
func (p product) signedAsset() string {
	if p.SignedAsset != "" {
		return p.SignedAsset
	}
	return p.Exe + ".sig"
}

// signedAssets — все подписи, которые должны быть в выпуске: exe/APK и
// подписи найденных Linux-файлов. Старый выпуск без Linux-файлов требует
// только основную подпись.
func (p product) signedAssets(names []string) []string {
	needed := []string{p.signedAsset()}
	seen := map[string]bool{needed[0]: true}
	for _, pattern := range p.Extras {
		for _, name := range names {
			if ok, _ := path.Match(pattern, name); !ok {
				continue
			}
			sig := name + ".sig"
			if !seen[sig] {
				seen[sig] = true
				needed = append(needed, sig)
			}
		}
	}
	return needed
}

// productOf — чей это выпуск по тегу; nil — не выпуски Wynd.
func productOf(tag string) *product {
	for i := range products {
		if strings.HasPrefix(tag, products[i].Prefix) {
			return &products[i]
		}
	}
	return nil
}

// GitHub — репозиторий выпусков и программа gh.
type GitHub struct {
	Repo string // mixeme/wynd
	Bin  string // gh; пусто — "gh" из PATH
	// AndroidEnv — среда Android для aapt/zipalign/apksigner; пусто —
	// D:\android-env (решение владельца, план 50).
	AndroidEnv string
}

func (g GitHub) ReleaseURL(tag string) string {
	return "https://github.com/" + g.Repo + "/releases/tag/" + tag
}

// AttestationURL — страница удостоверений сборки репозитория: по ней окно
// даёт ссылку «проверить» на экране выпуска (12в), как полоса обновления
// Мастерской ведёт на удостоверение сборки.
func (g GitHub) AttestationURL() string {
	return "https://github.com/" + g.Repo + "/attestations"
}

// ErrNoGh — программы gh нет.
var ErrNoGh = errors.New("нет программы gh (GitHub CLI): поставьте её и войдите — gh auth login")

func (g GitHub) gh(ctx context.Context, args ...string) ([]byte, error) {
	bin := g.Bin
	if bin == "" {
		bin = "gh"
	}
	if _, err := exec.LookPath(bin); err != nil {
		return nil, ErrNoGh
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	hideWindow(cmd)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return out.Bytes(), errors.New(msg)
	}
	return out.Bytes(), nil
}

// Release — выпуск программы Wynd (12б).
type Release struct {
	// Product — чья это программа: ProductWorkshop или ProductDesktop.
	Product string `json:"product"`
	// Name — как программа называется в окне, Exe — имя файла выпуска.
	Name        string `json:"name"`
	Exe         string `json:"exe"`
	Tag         string `json:"tag"`
	Version     string `json:"version"`
	Draft       bool   `json:"draft"`
	PublishedAt string `json:"published_at,omitempty"`
	Signed      bool   `json:"signed"`
	URL         string `json:"url,omitempty"`
}

// Releases — выпуски программ Wynd, новые сверху.
func (g GitHub) Releases(ctx context.Context) ([]Release, error) {
	// Каждая страница содержит и статус публикации, и файлы: медленный или
	// неудачный запрос отдельного выпуска не превращает его в «без подписи».
	type releaseRow struct {
		TagName     string `json:"tag_name"`
		IsDraft     bool   `json:"draft"`
		PublishedAt string `json:"published_at"`
		URL         string `json:"html_url"`
		Assets      []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	rows, err := releasepages.Collect(ctx, func(page int) ([]releaseRow, bool, error) {
		out, err := g.gh(ctx, "api", fmt.Sprintf("repos/%s/releases?per_page=%d&page=%d", g.Repo, releasepages.Size, page))
		if err != nil {
			return nil, false, err
		}
		var rows []releaseRow
		if err := json.Unmarshal(out, &rows); err != nil {
			return nil, false, err
		}
		return rows, len(rows) == releasepages.Size, nil
	})
	if err != nil {
		return nil, err
	}
	var res []Release
	for _, r := range rows {
		prod := productOf(r.TagName)
		if prod == nil {
			continue
		}
		rel := Release{Product: prod.ID, Name: prod.Name, Exe: prod.Exe, Tag: r.TagName, Version: strings.TrimPrefix(r.TagName, prod.Prefix), Draft: r.IsDraft, URL: r.URL}
		if !r.IsDraft {
			rel.PublishedAt = r.PublishedAt
		}
		names := make([]string, 0, len(r.Assets))
		for _, a := range r.Assets {
			names = append(names, a.Name)
		}
		rel.Signed = true
		for _, needed := range prod.signedAssets(names) {
			found := false
			for _, name := range names {
				if name == needed {
					found = true
					break
				}
			}
			if !found {
				rel.Signed = false
				break
			}
		}
		res = append(res, rel)
	}
	return res, nil
}

// ExtraAsset — дополнительный файл выпуска (Linux .deb/AppImage), который
// подписывается тем же ключом. Сумма и удостоверение проверяются до пароля.
type ExtraAsset struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	SumOK    bool   `json:"sum_ok"`
	AttestOK bool   `json:"attest_ok"`
	Commit   string `json:"commit,omitempty"`
}

// Check — проверки выпуска до пароля (12в).
type Check struct {
	CheckID   string `json:"check_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	// Product — чья это программа: ProductWorkshop или ProductDesktop.
	Product string `json:"product"`
	// Name — как программа называется в окне, Exe — имя файла выпуска.
	Name  string `json:"name"`
	Exe   string `json:"exe"`
	Tag   string `json:"tag"`
	Draft bool   `json:"draft"`
	Size  int64  `json:"size"`
	SumOK bool   `json:"sum_ok"`
	// Commit — коммит из удостоверения сборки.
	Commit string `json:"commit,omitempty"`
	// AttestOK — exe собран GitHub Actions из этого тега (gh attestation verify).
	AttestOK bool   `json:"attest_ok"`
	Notes    string `json:"notes,omitempty"`
	// SelfUpdate — программа с самообновлением: в её сборке должен быть ключ.
	SelfUpdate bool `json:"self_update"`
	// KeyInRelease — сборка уже содержит ключ для проверки самообновлений.
	KeyInRelease bool `json:"key_in_release"`
	// PubKey — открытый ключ из тега; пусто у старых сборок без самообновления.
	PubKey string `json:"-"`
	// Publishes — черновик будет опубликован после подписи; решает Go
	// (для Android — только при готовности к публикации в теге).
	Publishes bool `json:"publishes"`
	// PublishNote — почему публикация не состоится (Android без готовности).
	PublishNote string `json:"publish_note,omitempty"`
	Problem     string `json:"problem,omitempty"`
	Steps       []Step `json:"steps"`
	// Extra — проверенные Linux-файлы выпуска (пусто у старых выпусков).
	Extra []ExtraAsset `json:"extra,omitempty"`

	exe        string
	dir        string
	extraFiles []string
}

// Fetch скачивает exe выпуска во временную папку и проверяет его: сумму из
// выпуска, удостоверение сборки по тегу; берёт «Что нового» и открытый ключ
// из репозитория в этом теге.
func (g GitHub) Fetch(ctx context.Context, tag string) (*Check, error) {
	return g.FetchWithProgress(ctx, tag, nil)
}

// FetchWithProgress выполняет те же проверки, сообщая о настоящих этапах.
// Первый отказ останавливает проверку; подпись такого выпуска недоступна.
// Набор этапов — из описания программы: ключ проверяется у той, что
// самообновляется, «Что нового» — у той, чей whatsnew.md читается; у Android
// вместо ключа — пакет, выравнивание и готовность к публикации.
func (g GitHub) FetchWithProgress(ctx context.Context, tag string, report func(Progress)) (c *Check, err error) {
	prod := productOf(tag)
	if prod == nil {
		return nil, fmt.Errorf("%s — не выпуски Wynd: подписываемый файл должен быть из тега workshop-v…, desktop-v… или android-v…", tag)
	}
	downloadTitle := "Скачиваем программу и контрольную сумму"
	if prod.APK {
		downloadTitle = "Скачиваем APK и контрольную сумму"
	}
	steps := []Step{
		{ID: "release", Title: "Получаем сведения о выпуске"},
		{ID: "download", Title: downloadTitle},
		{ID: "checksum", Title: "Проверяем контрольную сумму"},
		{ID: "attestation", Title: "Проверяем происхождение сборки"},
	}
	if prod.APK {
		steps = append(steps,
			Step{ID: "package", Title: "Проверяем пакет и версию"},
			Step{ID: "align", Title: "Проверяем выравнивание"})
	}
	if prod.Key != "" {
		steps = append(steps, Step{ID: "key", Title: "Проверяем открытый ключ"})
	}
	if prod.Notes != "" {
		steps = append(steps, Step{ID: "notes", Title: "Читаем описание выпуска"})
	}
	if prod.APK {
		steps = append(steps, Step{ID: "readiness", Title: "Проверяем готовность к публикации"})
	}
	p := newProgress(tag, "check", report, steps...)
	defer func() {
		if err != nil || c != nil && c.Problem != "" {
			p.fail()
		}
		if c != nil {
			c.Steps = p.steps()
		}
	}()
	p.start("release")
	draft, err := g.isDraft(ctx, tag)
	if err != nil {
		return nil, err
	}
	p.done("release", "")
	p.start("download")
	dir, err := os.MkdirTemp("", "wynd-signer-")
	if err != nil {
		return nil, err
	}
	// У Android из черновика скачивается unsigned APK; у остальных — сам exe.
	dl := prod.downloadName()
	c = &Check{Product: prod.ID, Name: prod.Name, Exe: prod.Exe, Tag: tag, Draft: draft, SelfUpdate: prod.Key != "", dir: dir, exe: filepath.Join(dir, dl)}
	if _, err := g.gh(ctx, "release", "download", tag, "-R", g.Repo, "-D", dir, "-p", dl, "-p", dl+".sha256", "--clobber"); err != nil {
		c.Close()
		return nil, fmt.Errorf("скачать выпуск не получилось: %w", err)
	}
	p.done("download", "")
	p.start("checksum")
	downloaded, err := os.ReadFile(c.exe)
	if err != nil {
		c.Close()
		return nil, err
	}
	c.Size = int64(len(downloaded))
	sum := sha256.Sum256(downloaded)
	if want, err := os.ReadFile(c.exe + ".sha256"); err == nil {
		c.SumOK = strings.HasPrefix(strings.TrimSpace(string(want)), hex.EncodeToString(sum[:]))
	}
	if !c.SumOK {
		c.Problem = "Сумма не сходится с той, что в выпуске"
		return c, nil
	}
	p.done("checksum", "")
	p.start("attestation")
	verifyArgs := []string{"attestation", "verify", c.exe, "--repo", g.Repo, "--format", "json"}
	if prod.APK {
		// Как в скрипте подписи: удостоверение именно этого workflow и тега.
		verifyArgs = append(verifyArgs, "--source-ref", "refs/tags/"+tag,
			"--signer-workflow", g.Repo+"/.github/workflows/android.yaml")
	}
	out, attestErr := g.gh(ctx, verifyArgs...)
	if attestErr == nil {
		var att []struct {
			VerificationResult struct {
				Signature struct {
					Certificate struct {
						Ref    string `json:"sourceRepositoryRef"`
						Digest string `json:"sourceRepositoryDigest"`
					} `json:"certificate"`
				} `json:"signature"`
			} `json:"verificationResult"`
		}
		if json.Unmarshal(out, &att) == nil {
			for _, a := range att {
				cert := a.VerificationResult.Signature.Certificate
				if cert.Ref == "refs/tags/"+tag {
					c.AttestOK, c.Commit = true, cert.Digest
				}
			}
		}
	}
	if !c.AttestOK {
		c.Problem = "Нет удостоверения, что файл собран GitHub Actions из тега " + tag
		if attestErr != nil {
			c.Problem = "Проверить происхождение сборки не получилось: " + attestErr.Error()
		}
		return c, nil
	}
	p.doneLink("attestation", "коммит "+c.Commit[:min(8, len(c.Commit))], "проверить", g.AttestationURL())
	if len(prod.Extras) > 0 {
		if problem := g.checkExtras(ctx, c, prod); problem != "" {
			c.Problem = problem
			return c, nil
		}
	}
	if prod.APK {
		// Пакет, выравнивание, описание и готовность — шаги android.go;
		// отказы ставят c.Problem и останавливают проверку.
		g.androidCheck(ctx, c, prod, p, draft)
		return c, nil
	}
	if prod.Key != "" {
		p.start("key")
		c.PubKey, err = g.releaseKey(ctx, tag, prod.Key)
		c.KeyInRelease = c.PubKey != ""
		if err != nil {
			c.Problem = "Проверить открытый ключ в теге " + tag + " не получилось: " + err.Error()
			return c, nil
		}
		keyNote := ""
		if !c.KeyInRelease {
			keyNote = "в этой сборке нет ключа самообновления"
		}
		p.done("key", keyNote)
	}
	if prod.Notes != "" {
		p.start("notes")
		c.Notes = WhatsNew(g.notes(ctx, prod, tag), strings.TrimPrefix(tag, prod.Prefix))
		notesNote := ""
		if c.Notes == "" {
			notesNote = "описание версии не получено"
		}
		p.done("notes", notesNote)
	}
	c.Publishes = c.Draft && c.Notes != ""
	return c, nil
}

// checkExtras проверяет Linux-файлы выпуска: сумму из SHA256SUMS и
// удостоверение сборки. В старых выпусках Linux-файлов нет — это не ошибка.
func (g GitHub) checkExtras(ctx context.Context, c *Check, prod *product) string {
	out, err := g.gh(ctx, "release", "view", c.Tag, "-R", g.Repo, "--json", "assets", "--jq", ".assets[].name")
	if err != nil {
		return "Не удалось прочитать файлы выпуска: " + err.Error()
	}
	names := strings.Fields(string(out))
	var matched, missing []string
	seen := make(map[string]bool)
	for _, pattern := range prod.Extras {
		found := false
		for _, name := range names {
			if ok, _ := path.Match(pattern, name); ok {
				found = true
				if !seen[name] {
					seen[name] = true
					matched = append(matched, name)
				}
			}
		}
		if !found {
			missing = append(missing, pattern)
		}
	}
	if len(matched) == 0 {
		return ""
	}
	if len(missing) > 0 {
		return "В выпуске нет файлов: " + strings.Join(missing, ", ")
	}
	hasSums := false
	for _, name := range names {
		if name == "SHA256SUMS" {
			hasSums = true
		}
	}
	if !hasSums {
		return "В выпуске нет SHA256SUMS"
	}

	args := []string{"release", "download", c.Tag, "-R", g.Repo, "-D", c.dir}
	for _, name := range matched {
		args = append(args, "-p", name)
	}
	args = append(args, "-p", "SHA256SUMS", "--clobber")
	if _, err := g.gh(ctx, args...); err != nil {
		return "Скачать Linux-файлы не получилось: " + err.Error()
	}
	sums := make(map[string]string)
	body, err := os.ReadFile(filepath.Join(c.dir, "SHA256SUMS"))
	if err != nil {
		return "Не удалось прочитать SHA256SUMS: " + err.Error()
	}
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			sums[fields[1]] = fields[0]
		}
	}

	for _, name := range matched {
		full := filepath.Join(c.dir, name)
		data, err := os.ReadFile(full)
		if err != nil {
			return name + ": не удалось прочитать"
		}
		sum := sha256.Sum256(data)
		asset := ExtraAsset{Name: name, Size: int64(len(data))}
		asset.SumOK = strings.EqualFold(sums[name], hex.EncodeToString(sum[:]))
		if !asset.SumOK {
			return name + ": сумма не сходится с SHA256SUMS"
		}
		ok, commit, err := g.verifyAttestation(ctx, full, c.Tag)
		if err != nil {
			return name + ": проверить удостоверение не получилось: " + err.Error()
		}
		asset.AttestOK, asset.Commit = ok, commit
		if !ok {
			return name + ": нет удостоверения сборки из тега " + c.Tag
		}
		c.Extra = append(c.Extra, asset)
		c.extraFiles = append(c.extraFiles, full)
	}
	return ""
}

// verifyAttestation проверяет, что файл собран GitHub Actions из тега.
func (g GitHub) verifyAttestation(ctx context.Context, file, tag string) (bool, string, error) {
	out, err := g.gh(ctx, "attestation", "verify", file, "--repo", g.Repo, "--format", "json")
	if err != nil {
		return false, "", err
	}
	var att []struct {
		VerificationResult struct {
			Signature struct {
				Certificate struct {
					Ref    string `json:"sourceRepositoryRef"`
					Digest string `json:"sourceRepositoryDigest"`
				} `json:"certificate"`
			} `json:"signature"`
		} `json:"verificationResult"`
	}
	if json.Unmarshal(out, &att) != nil {
		return false, "", errors.New("GitHub вернул неожиданный ответ удостоверения")
	}
	for _, a := range att {
		cert := a.VerificationResult.Signature.Certificate
		if cert.Ref == "refs/tags/"+tag {
			return true, cert.Digest, nil
		}
	}
	return false, "", nil
}

func (g GitHub) isDraft(ctx context.Context, tag string) (bool, error) {
	out, err := g.gh(ctx, "release", "view", tag, "-R", g.Repo, "--json", "isDraft")
	if err != nil {
		return false, err
	}
	var view struct {
		Draft *bool `json:"isDraft"`
	}
	if err := json.Unmarshal(out, &view); err != nil {
		return false, err
	}
	if view.Draft == nil {
		return false, errors.New("GitHub не сообщил, опубликован ли выпуск")
	}
	return *view.Draft, nil
}

// releaseKey — открытый ключ, который сборка программы вшивает из репозитория
// тега (keyPath, общий ключ экосистемы). Файла в теге нет — это старая сборка
// без самообновления, а не ошибка. Ошибка — только если GitHub не ответил:
// при ошибке нельзя молча перейти на подпись любым выбранным ключом.
func (g GitHub) releaseKey(ctx context.Context, tag, keyPath string) (string, error) {
	dir := path.Dir(keyPath) // «.» — ключ в корне репозитория
	api := "repos/" + g.Repo + "/contents"
	if dir != "." {
		api += "/" + dir
	}
	out, err := g.gh(ctx, "api", api+"?ref="+tag)
	if err != nil {
		return "", err
	}
	var files []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &files); err != nil {
		return "", err
	}
	if files == nil {
		return "", errors.New("GitHub не вернул список файлов тега")
	}
	for _, f := range files {
		if f.Name != path.Base(keyPath) {
			continue
		}
		pub := strings.TrimSpace(g.file(ctx, keyPath, tag))
		if pub == "" {
			return "", errors.New("не удалось прочитать " + keyPath)
		}
		return pub, nil
	}
	return "", nil
}

// file — файл репозитория в теге; нет — пусто.
// notes — «Что нового» из тега. Каталог Мастерской переименован 2026-11-15;
// в тегах до 0.4.4 включительно файл лежит по старому пути — читаем его,
// если по новому пусто (черновик 0.4.4 подписывается пересобранной «Подписью»).
func (g GitHub) notes(ctx context.Context, prod *product, tag string) string {
	out := g.file(ctx, prod.Notes, tag)
	if out != "" || prod.ID != ProductWorkshop {
		return out
	}
	return g.file(ctx, "cmd/wynd-installer/whatsnew.md", tag)
}

// fileE — файл репозитория в теге с ошибкой вместо пустоты: сетевой отказ
// не выглядит как «файла нет» (нужно для готовности Android к публикации).
func (g GitHub) fileE(ctx context.Context, path, ref string) (string, error) {
	out, err := g.gh(ctx, "api", "repos/"+g.Repo+"/contents/"+path+"?ref="+ref, "--jq", ".content")
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(string(out)), ""))
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (g GitHub) file(ctx context.Context, path, ref string) string {
	out, err := g.gh(ctx, "api", "repos/"+g.Repo+"/contents/"+path+"?ref="+ref, "--jq", ".content")
	if err != nil {
		return ""
	}
	// gh печатает base64 с переносами строк каждые 60 знаков.
	raw, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(string(out)), ""))
	if err != nil {
		return ""
	}
	return string(raw)
}

// Close убирает скачанное: на компьютере после подписи ничего не остаётся (12г).
func (c *Check) Close() {
	if c != nil && c.dir != "" {
		_ = os.RemoveAll(c.dir)
	}
}

// SignAndPublish подписывает файлы выпуска (exe и Linux-пакеты) ключом из
// keyPath, проверяет подписи открытым ключом тега (у старых сборок — выбранного
// ключа), выкладывает .sig и, если это черновик и есть «Что нового», публикует
// его с этим текстом. Описание и дату уже опубликованного выпуска не меняет.
func (g GitHub) SignAndPublish(ctx context.Context, c *Check, keyPath, pass string) (published bool, err error) {
	published, _, err = g.SignAndPublishWithProgress(ctx, c, keyPath, pass, nil)
	return published, err
}

// SignAndPublishWithProgress сообщает этапы подписи, загрузки и публикации.
// Ключ и пароль в сообщениях о ходе работы не передаются. Второй результат —
// готовый текст итога для окна (Android); у exe он пуст — шаблон в окне.
func (g GitHub) SignAndPublishWithProgress(ctx context.Context, c *Check, keyPath, pass string, report func(Progress)) (published bool, detail string, err error) {
	prod := productOf(c.Tag)
	if prod != nil && prod.APK {
		p := newProgress(c.Tag, "sign", report,
			Step{ID: "release", Title: "Проверяем состояние выпуска"},
			Step{ID: "signature", Title: "Подписываем и проверяем подпись"},
			Step{ID: "upload", Title: "Загружаем подписанный APK на GitHub"},
		)
		defer func() {
			if err != nil {
				p.fail()
			}
		}()
		return g.signAndPublishAndroid(ctx, c, keyPath, pass, p)
	}
	p := newProgress(c.Tag, "sign", report,
		Step{ID: "release", Title: "Проверяем состояние выпуска"},
		Step{ID: "signature", Title: "Подписываем и проверяем подпись"},
		Step{ID: "upload", Title: "Загружаем подпись на GitHub"},
	)
	defer func() {
		if err != nil {
			p.fail()
		}
	}()
	p.start("release")
	if c.Problem != "" {
		return false, "", errors.New(c.Problem)
	}
	// За время проверок черновик могли опубликовать другим способом.
	draft, err := g.isDraft(ctx, c.Tag)
	if err != nil {
		return false, "", err
	}
	if draft && c.Publishes {
		p.value.Steps = append(p.value.Steps, Step{ID: "publish", Title: "Публикуем выпуск", Status: "pending"})
	}
	p.done("release", "")
	p.start("signature")
	priv, err := Unlock(keyPath, pass)
	if err != nil {
		return false, "", err
	}
	signOne := func(file string) (string, error) {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		sig, err := Sign(priv, data, c.PubKey)
		if err != nil {
			return "", err
		}
		sigPath := file + ".sig"
		return sigPath, os.WriteFile(sigPath, []byte(sig+"\n"), 0o644)
	}
	sigPaths := make([]string, 0, 1+len(c.extraFiles))
	sigPath, err := signOne(c.exe)
	if err != nil {
		return false, "", err
	}
	sigPaths = append(sigPaths, sigPath)
	for _, file := range c.extraFiles {
		sigPath, err := signOne(file)
		if err != nil {
			return false, "", err
		}
		sigPaths = append(sigPaths, sigPath)
	}
	p.done("signature", "")
	p.start("upload")
	uploadArgs := []string{"release", "upload", c.Tag}
	uploadArgs = append(uploadArgs, sigPaths...)
	uploadArgs = append(uploadArgs, "-R", g.Repo, "--clobber")
	if _, err := g.gh(ctx, uploadArgs...); err != nil {
		return false, "", fmt.Errorf("выложить подпись не получилось: %w", err)
	}
	p.done("upload", "")
	if !draft {
		return true, "", nil
	}
	if c.Notes == "" {
		return false, "", nil
	}
	p.start("publish")
	notes := filepath.Join(c.dir, "notes.md")
	if err := os.WriteFile(notes, []byte(c.Notes+"\n"), 0o644); err != nil {
		return false, "", err
	}
	if _, err := g.gh(ctx, "release", "edit", c.Tag, "-R", g.Repo, "--notes-file", notes, "--draft=false"); err != nil {
		return false, "", fmt.Errorf("подпись выложена, а опубликовать не получилось: %w", err)
	}
	p.done("publish", "")
	return true, "", nil
}

// SignFile подписывает выбранный файл на этом компьютере (12д): .sig — рядом,
// под его именем. pub — открытый ключ для проверки (пусто — свой).
func SignFile(keyPath, pass, exePath, pub string) (string, error) {
	priv, err := Unlock(keyPath, pass)
	if err != nil {
		return "", err
	}
	exe, err := os.ReadFile(exePath)
	if err != nil {
		return "", err
	}
	sig, err := Sign(priv, exe, pub)
	if err != nil {
		return "", err
	}
	sigPath := exePath + ".sig"
	return sigPath, os.WriteFile(sigPath, []byte(sig+"\n"), 0o644)
}
