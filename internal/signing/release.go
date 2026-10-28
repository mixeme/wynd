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
)

// Выпуски Мастерской на GitHub (кадры 12б–12г): черновик собирает
// .github/workflows/workshop.yaml по тегу workshop-vX.Y.Z. Говорим с GitHub
// через gh (GitHub CLI) — как scripts\sign-workshop.bat.

const (
	exeName   = "wynd-workshop.exe"
	tagPrefix = "workshop-v"
)

// GitHub — репозиторий выпусков и программа gh.
type GitHub struct {
	Repo string // mixeme/wynd
	Bin  string // gh; пусто — "gh" из PATH
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

// Release — выпуск Мастерской (12б).
type Release struct {
	Tag         string `json:"tag"`
	Version     string `json:"version"`
	Draft       bool   `json:"draft"`
	PublishedAt string `json:"published_at,omitempty"`
	Signed      bool   `json:"signed"`
	URL         string `json:"url,omitempty"`
}

// Releases — выпуски Мастерской, новые сверху.
func (g GitHub) Releases(ctx context.Context) ([]Release, error) {
	out, err := g.gh(ctx, "release", "list", "-R", g.Repo, "-L", "30", "--json", "tagName,isDraft,publishedAt")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		TagName     string `json:"tagName"`
		IsDraft     bool   `json:"isDraft"`
		PublishedAt string `json:"publishedAt"`
	}
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, err
	}
	var res []Release
	for _, r := range rows {
		if !strings.HasPrefix(r.TagName, tagPrefix) {
			continue
		}
		rel := Release{Tag: r.TagName, Version: strings.TrimPrefix(r.TagName, tagPrefix), Draft: r.IsDraft}
		if !r.IsDraft {
			rel.PublishedAt = r.PublishedAt
		}
		if v, err := g.gh(ctx, "release", "view", r.TagName, "-R", g.Repo, "--json", "assets,url"); err == nil {
			var view struct {
				URL    string `json:"url"`
				Assets []struct {
					Name string `json:"name"`
				} `json:"assets"`
			}
			if json.Unmarshal(v, &view) == nil {
				rel.URL = view.URL
				for _, a := range view.Assets {
					if a.Name == exeName+".sig" {
						rel.Signed = true
					}
				}
			}
		}
		res = append(res, rel)
	}
	return res, nil
}

// Check — проверки черновика до пароля (12в).
type Check struct {
	Tag    string `json:"tag"`
	Size   int64  `json:"size"`
	SumOK  bool   `json:"sum_ok"`
	Commit string `json:"commit,omitempty"`
	// AttestOK — exe собран GitHub Actions из этого тега (gh attestation verify).
	AttestOK bool   `json:"attest_ok"`
	Notes    string `json:"notes,omitempty"`
	// PubKey — открытый ключ из репозитория в этом теге: им проверяется подпись.
	PubKey  string `json:"-"`
	Problem string `json:"problem,omitempty"`

	exe string
	dir string
}

// Fetch скачивает exe черновика во временную папку и проверяет его: сумму из
// выпуска, удостоверение сборки по тегу; берёт «Что нового» и открытый ключ
// из репозитория в этом теге.
func (g GitHub) Fetch(ctx context.Context, tag string) (*Check, error) {
	dir, err := os.MkdirTemp("", "wynd-signer-")
	if err != nil {
		return nil, err
	}
	c := &Check{Tag: tag, dir: dir, exe: filepath.Join(dir, exeName)}
	if _, err := g.gh(ctx, "release", "download", tag, "-R", g.Repo, "-D", dir, "-p", exeName, "-p", exeName+".sha256", "--clobber"); err != nil {
		c.Close()
		return nil, fmt.Errorf("скачать выпуск не получилось: %w", err)
	}
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
	if out, err := g.gh(ctx, "attestation", "verify", c.exe, "--repo", g.Repo, "--format", "json"); err == nil {
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
	c.Notes = WhatsNew(g.file(ctx, "cmd/wynd-installer/whatsnew.md", tag), strings.TrimPrefix(tag, tagPrefix))
	c.PubKey = strings.TrimSpace(g.file(ctx, "cmd/wynd-installer/signing-key.pub", tag))
	switch {
	case !c.SumOK:
		c.Problem = "Сумма не сходится с той, что в выпуске"
	case !c.AttestOK:
		c.Problem = "Нет удостоверения, что exe собран GitHub Actions из тега " + tag
	case c.PubKey == "":
		c.Problem = "В теге " + tag + " нет открытого ключа cmd/wynd-installer/signing-key.pub — эта сборка подпись не проверит"
	}
	return c, nil
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
// подпись открытым ключом тега, выкладывает .sig в черновик и, если есть
// «Что нового», публикует выпуск с этим текстом.
func (g GitHub) SignAndPublish(ctx context.Context, c *Check, keyPath, pass string) (published bool, err error) {
	if c.Problem != "" {
		return false, errors.New(c.Problem)
	}
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
	if _, err := g.gh(ctx, "release", "upload", c.Tag, sigPath, "-R", g.Repo, "--clobber"); err != nil {
		return false, fmt.Errorf("выложить подпись не получилось: %w", err)
	}
	if c.Notes == "" {
		return false, nil
	}
	notes := filepath.Join(c.dir, "notes.md")
	if err := os.WriteFile(notes, []byte(c.Notes+"\n"), 0o644); err != nil {
		return false, err
	}
	if _, err := g.gh(ctx, "release", "edit", c.Tag, "-R", g.Repo, "--notes-file", notes, "--draft=false"); err != nil {
		return false, fmt.Errorf("подпись выложена, а опубликовать не получилось: %w", err)
	}
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
