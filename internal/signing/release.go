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
	"path/filepath"
	"strings"

	"gitea.mixdep.ru/mix/wynd/internal/releasepages"
)

// Выпуски Wynd на GitHub (кадры 12б–12г): Мастерскую собирает
// .github/workflows/workshop.yaml по тегу workshop-vX.Y.Z, окно для
// компьютера — desktop-client.yaml по тегу desktop-vX.Y.Z. Говорим с GitHub
// через gh (GitHub CLI) — как scripts\sign-workshop.bat.

// Подписываемые программы и их выпуски в mixeme/wynd.
const (
	// ProductWorkshop — Мастерская: выпуск workshop-vX.Y.Z, wynd-workshop.exe.
	ProductWorkshop = "workshop"
	// ProductDesktop — Wynd для компьютера: выпуск desktop-vX.Y.Z, wynd-desktop.exe.
	ProductDesktop = "desktop"
)

type product struct {
	ID     string // ProductWorkshop / ProductDesktop — уходит в окно как product.
	Exe    string // имя exe в выпуске.
	Prefix string // префикс тега выпуска.
}

var products = []product{
	{ID: ProductWorkshop, Exe: "wynd-workshop.exe", Prefix: "workshop-v"},
	{ID: ProductDesktop, Exe: "wynd-desktop.exe", Prefix: "desktop-v"},
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
}

func (g GitHub) ReleaseURL(tag string) string {
	return "https://github.com/" + g.Repo + "/releases/tag/" + tag
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
	Product     string `json:"product"`
	Tag         string `json:"tag"`
	Version     string `json:"version"`
	Draft       bool   `json:"draft"`
	PublishedAt string `json:"published_at,omitempty"`
	Signed      bool   `json:"signed"`
	URL         string `json:"url,omitempty"`
}

// Releases — выпуски Мастерской, новые сверху.
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
		rel := Release{Product: prod.ID, Tag: r.TagName, Version: strings.TrimPrefix(r.TagName, prod.Prefix), Draft: r.IsDraft, URL: r.URL}
		if !r.IsDraft {
			rel.PublishedAt = r.PublishedAt
		}
		for _, a := range r.Assets {
			if a.Name == prod.Exe+".sig" {
				rel.Signed = true
			}
		}
		res = append(res, rel)
	}
	return res, nil
}

// Check — проверки выпуска до пароля (12в).
type Check struct {
	CheckID   string `json:"check_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	// Product — чья это программа: ProductWorkshop или ProductDesktop.
	Product   string `json:"product"`
	Tag       string `json:"tag"`
	Draft     bool   `json:"draft"`
	Size      int64  `json:"size"`
	SumOK     bool   `json:"sum_ok"`
	Commit    string `json:"commit,omitempty"`
	// AttestOK — exe собран GitHub Actions из этого тега (gh attestation verify).
	AttestOK bool   `json:"attest_ok"`
	Notes    string `json:"notes,omitempty"`
	// KeyInRelease — сборка уже содержит ключ для проверки самообновлений.
	KeyInRelease bool `json:"key_in_release"`
	// PubKey — открытый ключ из тега; пусто у старых сборок без самообновления.
	PubKey  string `json:"-"`
	Problem string `json:"problem,omitempty"`
	Steps   []Step `json:"steps"`

	exe string
	dir string
}

// Fetch скачивает exe выпуска во временную папку и проверяет его: сумму из
// выпуска, удостоверение сборки по тегу; берёт «Что нового» и открытый ключ
// из репозитория в этом теге.
func (g GitHub) Fetch(ctx context.Context, tag string) (*Check, error) {
	return g.FetchWithProgress(ctx, tag, nil)
}

// FetchWithProgress выполняет те же проверки, сообщая о настоящих этапах.
// Первый отказ останавливает проверку; подпись такого выпуска недоступна.
// У окна для компьютера самообновления ещё нет: ключа и «Что нового» в его
// выпуске не бывает, поэтому таких этапов у него нет.
func (g GitHub) FetchWithProgress(ctx context.Context, tag string, report func(Progress)) (c *Check, err error) {
	prod := productOf(tag)
	if prod == nil {
		return nil, fmt.Errorf("%s — не выпуски Wynd: подписываемый exe должен быть из тега workshop-v… или desktop-v…", tag)
	}
	steps := []Step{
		{ID: "release", Title: "Получаем сведения о выпуске"},
		{ID: "download", Title: "Скачиваем программу и контрольную сумму"},
		{ID: "checksum", Title: "Проверяем контрольную сумму"},
		{ID: "attestation", Title: "Проверяем происхождение сборки"},
	}
	if prod.ID == ProductWorkshop {
		steps = append(steps,
			Step{ID: "key", Title: "Проверяем открытый ключ"},
			Step{ID: "notes", Title: "Читаем описание выпуска"},
		)
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
	c = &Check{Product: prod.ID, Tag: tag, Draft: draft, dir: dir, exe: filepath.Join(dir, prod.Exe)}
	if _, err := g.gh(ctx, "release", "download", tag, "-R", g.Repo, "-D", dir, "-p", prod.Exe, "-p", prod.Exe+".sha256", "--clobber"); err != nil {
		c.Close()
		return nil, fmt.Errorf("скачать выпуск не получилось: %w", err)
	}
	p.done("download", "")
	p.start("checksum")
	exe, err := os.ReadFile(c.exe)
	if err != nil {
		c.Close()
		return nil, err
	}
	c.Size = int64(len(exe))
	sum := sha256.Sum256(exe)
	if want, err := os.ReadFile(c.exe + ".sha256"); err == nil {
		c.SumOK = strings.HasPrefix(strings.TrimSpace(string(want)), hex.EncodeToString(sum[:]))
	}
	if !c.SumOK {
		c.Problem = "Сумма не сходится с той, что в выпуске"
		return c, nil
	}
	p.done("checksum", "")
	p.start("attestation")
	out, attestErr := g.gh(ctx, "attestation", "verify", c.exe, "--repo", g.Repo, "--format", "json")
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
		c.Problem = "Нет удостоверения, что exe собран GitHub Actions из тега " + tag
		if attestErr != nil {
			c.Problem = "Проверить происхождение сборки не получилось: " + attestErr.Error()
		}
		return c, nil
	}
	p.done("attestation", "коммит "+c.Commit[:min(8, len(c.Commit))])
	if prod.ID != ProductWorkshop {
		// Окно для компьютера пока не проверяет подпись при обновлении:
		// встроенного ключа в сборке нет, «Что нового» дописывает владелец.
		return c, nil
	}
	p.start("key")
	c.PubKey, err = g.releaseKey(ctx, tag)
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
	p.start("notes")
	c.Notes = WhatsNew(g.file(ctx, "cmd/wynd-installer/whatsnew.md", tag), strings.TrimPrefix(tag, prod.Prefix))
	notesNote := ""
	if c.Notes == "" {
		notesNote = "описание версии не получено"
	}
	p.done("notes", notesNote)
	return c, nil
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

// Отличаем отсутствие ключа в старом теге от ошибки GitHub: при ошибке
// проверки нельзя молча перейти на подпись любым выбранным ключом.
func (g GitHub) releaseKey(ctx context.Context, tag string) (string, error) {
	out, err := g.gh(ctx, "api", "repos/"+g.Repo+"/contents/cmd/wynd-installer?ref="+tag)
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
		if f.Name != "signing-key.pub" {
			continue
		}
		pub := strings.TrimSpace(g.file(ctx, "cmd/wynd-installer/signing-key.pub", tag))
		if pub == "" {
			return "", errors.New("не удалось прочитать signing-key.pub")
		}
		return pub, nil
	}
	return "", nil
}

// file — файл репозитория в теге; нет — пусто.
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

// SignAndPublish подписывает скачанный exe ключом из keyPath, проверяет
// подпись открытым ключом тега (у старых сборок — выбранного ключа), выкладывает
// .sig и, если это черновик и есть «Что нового», публикует его с этим текстом.
// Описание и дату уже опубликованного выпуска не меняет.
func (g GitHub) SignAndPublish(ctx context.Context, c *Check, keyPath, pass string) (published bool, err error) {
	return g.SignAndPublishWithProgress(ctx, c, keyPath, pass, nil)
}

// SignAndPublishWithProgress сообщает этапы подписи, загрузки и публикации.
// Ключ и пароль в сообщениях о ходе работы не передаются.
func (g GitHub) SignAndPublishWithProgress(ctx context.Context, c *Check, keyPath, pass string, report func(Progress)) (published bool, err error) {
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
		return false, errors.New(c.Problem)
	}
	// За время проверок черновик могли опубликовать другим способом.
	draft, err := g.isDraft(ctx, c.Tag)
	if err != nil {
		return false, err
	}
	if draft && c.Notes != "" {
		p.value.Steps = append(p.value.Steps, Step{ID: "publish", Title: "Публикуем выпуск", Status: "pending"})
	}
	p.done("release", "")
	p.start("signature")
	priv, err := Unlock(keyPath, pass)
	if err != nil {
		return false, err
	}
	exe, err := os.ReadFile(c.exe)
	if err != nil {
		return false, err
	}
	sig, err := Sign(priv, exe, c.PubKey)
	if err != nil {
		return false, err
	}
	sigPath := c.exe + ".sig"
	if err := os.WriteFile(sigPath, []byte(sig+"\n"), 0o644); err != nil {
		return false, err
	}
	p.done("signature", "")
	p.start("upload")
	if _, err := g.gh(ctx, "release", "upload", c.Tag, sigPath, "-R", g.Repo, "--clobber"); err != nil {
		return false, fmt.Errorf("выложить подпись не получилось: %w", err)
	}
	p.done("upload", "")
	if !draft {
		return true, nil
	}
	if c.Notes == "" {
		return false, nil
	}
	p.start("publish")
	notes := filepath.Join(c.dir, "notes.md")
	if err := os.WriteFile(notes, []byte(c.Notes+"\n"), 0o644); err != nil {
		return false, err
	}
	if _, err := g.gh(ctx, "release", "edit", c.Tag, "-R", g.Repo, "--notes-file", notes, "--draft=false"); err != nil {
		return false, fmt.Errorf("подпись выложена, а опубликовать не получилось: %w", err)
	}
	p.done("publish", "")
	return true, nil
}

// SignFile подписывает exe на этом компьютере (12д): .sig — рядом, под его
// именем. pub — открытый ключ для проверки (пусто — свой).
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
