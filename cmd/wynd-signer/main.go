//go:build dev || desktop || bindings

// «Wynd Подпись» (план 45, пункт 3а; кадры 12а–12д): окно, где владелец
// создаёт ключ подписи Мастерской и подписывает её выпуски — без командной
// строки. Отдельно от Мастерской: подпись нужна только тому, кто её выпускает,
// а ключ не должен жить в программе, которую раздают людям. Сами ключ и
// подпись — internal/signing (их же зовёт cmd/workshop-sign). Собирается
// scripts\build-signer.bat (`wails build`).
package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"gitea.mixdep.ru/mix/wynd/internal/signing"
)

//go:embed all:frontend/dist
var assets embed.FS

// App — то, что окно может попросить (window.go.main.App.*). Пароль ключа
// дальше signing.Unlock не уходит и нигде не сохраняется.
type App struct {
	ctx context.Context
	gh  signing.GitHub

	mu    sync.Mutex
	check *signing.Check
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

func (a *App) shutdown(context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.check.Close()
}

// settings — что окно помнит на этом компьютере: где лежит ключ (12а, 12б).
type settings struct {
	KeyPath string `json:"key_path"`
}

// Пробы окна не трогают настоящие файлы владельца:
//
//	WYND_SIGNER_SETTINGS — файл настроек вместо %APPDATA%\Wynd\signer.json
//	WYND_SIGNER_PUB      — куда класть открытый ключ вместо репозитория
func settingsPath() string {
	if p := os.Getenv("WYND_SIGNER_SETTINGS"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "Wynd", "signer.json")
}

func loadSettings() settings {
	var s settings
	if raw, err := os.ReadFile(settingsPath()); err == nil {
		_ = json.Unmarshal(raw, &s)
	}
	if s.KeyPath == "" {
		home, _ := os.UserHomeDir()
		// Там же, где его ищет scripts\sign-workshop.bat.
		s.KeyPath = filepath.Join(home, ".wynd", "workshop-signing.key")
	}
	return s
}

func saveSettings(s settings) {
	p := settingsPath()
	if p == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	raw, _ := json.MarshalIndent(s, "", "  ")
	_ = os.WriteFile(p, raw, 0o600)
}

// repoRoot — папка репозитория Wynd: туда (cmd/wynd-installer/signing-key.pub)
// ложится открытый ключ, его коммитят. Ищем вверх от exe и от рабочей папки.
func repoRoot() string {
	var starts []string
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}
	for _, dir := range starts {
		for d := dir; ; {
			if raw, err := os.ReadFile(filepath.Join(d, "go.mod")); err == nil && strings.Contains(string(raw), "module gitea.mixdep.ru/mix/wynd\n") {
				return d
			}
			up := filepath.Dir(d)
			if up == d {
				break
			}
			d = up
		}
	}
	return ""
}

// State — с чего начать (12а или 12б).
type State struct {
	KeyPath   string `json:"key_path"`
	KeyExists bool   `json:"key_exists"`
	// PubPath — куда ляжет открытый ключ; пусто — репозиторий не найден, и он
	// ляжет рядом с ключом (.pub).
	PubPath string `json:"pub_path"`
}

func pubPath(keyPath string) string {
	if p := os.Getenv("WYND_SIGNER_PUB"); p != "" {
		return p
	}
	if root := repoRoot(); root != "" {
		return filepath.Join(root, "cmd", "wynd-installer", "signing-key.pub")
	}
	return keyPath + ".pub"
}

func (a *App) State() State {
	s := loadSettings()
	_, err := os.Stat(s.KeyPath)
	return State{KeyPath: s.KeyPath, KeyExists: err == nil, PubPath: pubPath(s.KeyPath)}
}

// ChooseKeyPlace — где хранить новый ключ (12а).
func (a *App) ChooseKeyPlace(current string) string {
	p, _ := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title: "Где хранить ключ", DefaultDirectory: filepath.Dir(current), DefaultFilename: filepath.Base(current),
	})
	return p
}

// ChooseKey — другой файл ключа (12б: «другой»); выбранное окно запоминает.
func (a *App) ChooseKey() State {
	s := loadSettings()
	p, _ := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Файл ключа подписи", DefaultDirectory: filepath.Dir(s.KeyPath)})
	if p != "" {
		s.KeyPath = p
		saveSettings(s)
	}
	return a.State()
}

// Result — чем кончилось действие.
type Result struct {
	OK bool `json:"ok"`
	// WrongPassword — пароль ключа не подошёл (12в, 12д: «Пароль не подошёл»).
	WrongPassword bool   `json:"wrong_password,omitempty"`
	Message       string `json:"message,omitempty"`
	// Path — файл ключа (12а), подпись (12д).
	Path string `json:"path,omitempty"`
	// PubPath — куда записан открытый ключ (12а).
	PubPath   string `json:"pub_path,omitempty"`
	Published bool   `json:"published,omitempty"`
	URL       string `json:"url,omitempty"`
}

func fail(err error) Result {
	if errors.Is(err, signing.ErrWrongPassword) {
		return Result{WrongPassword: true, Message: "Пароль не подошёл"}
	}
	return Result{Message: err.Error()}
}

// CreateKey — «Создать ключ» (12а).
func (a *App) CreateKey(keyPath, pass string) Result {
	pub := pubPath(keyPath)
	if _, err := signing.Keygen(keyPath, pub, pass); err != nil {
		return fail(err)
	}
	s := loadSettings()
	s.KeyPath = keyPath
	saveSettings(s)
	return Result{OK: true, Path: keyPath, PubPath: pub}
}

// Releases — выпуски Мастерской (12б).
type Releases struct {
	OK       bool              `json:"ok"`
	Message  string            `json:"message,omitempty"`
	Releases []signing.Release `json:"releases"`
}

func (a *App) Releases() Releases {
	rels, err := a.gh.Releases(a.ctx)
	if err != nil {
		return Releases{Message: err.Error()}
	}
	return Releases{OK: true, Releases: rels}
}

// CheckRelease — проверки черновика до пароля (12в).
func (a *App) CheckRelease(tag string) (*signing.Check, error) {
	c, err := a.gh.Fetch(a.ctx, tag)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.check.Close()
	a.check = c
	a.mu.Unlock()
	return c, nil
}

// SignRelease — «Подписать и опубликовать» (12в → 12г).
func (a *App) SignRelease(pass string) Result {
	a.mu.Lock()
	c := a.check
	a.mu.Unlock()
	if c == nil {
		return Result{Message: "Сначала выберите выпуск"}
	}
	published, err := a.gh.SignAndPublish(a.ctx, c, loadSettings().KeyPath, pass)
	if err != nil {
		return fail(err)
	}
	a.mu.Lock()
	c.Close()
	a.check = nil
	a.mu.Unlock()
	return Result{OK: true, Published: published, URL: "https://github.com/" + a.gh.Repo + "/releases/tag/" + c.Tag}
}

// PickExe — «Подписать файл на этом компьютере» (12б).
func (a *App) PickExe() string {
	p, _ := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Что подписать", Filters: []runtime.FileFilter{{DisplayName: "Программа (*.exe)", Pattern: "*.exe"}},
	})
	return p
}

// FileSize — размер выбранного файла.
func (a *App) FileSize(path string) int64 {
	if st, err := os.Stat(path); err == nil {
		return st.Size()
	}
	return 0
}

// SignFile — подписать выбранный exe (→ 12д). Проверка — открытым ключом из
// репозитория, если он там есть, иначе своим.
func (a *App) SignFile(path, pass string) Result {
	pub := ""
	if raw, err := os.ReadFile(pubPath(loadSettings().KeyPath)); err == nil {
		pub = strings.TrimSpace(string(raw))
	}
	sig, err := signing.SignFile(loadSettings().KeyPath, pass, path, pub)
	if err != nil {
		return fail(err)
	}
	return Result{OK: true, Path: sig}
}

// OpenLink — страница выпуска в браузере (12г).
func (a *App) OpenLink(url string) {
	if strings.HasPrefix(url, "https://github.com/") {
		runtime.BrowserOpenURL(a.ctx, url)
	}
}

// ShowInFolder — «Показать в папке» (12д).
func (a *App) ShowInFolder(path string) {
	_ = exec.Command("explorer", "/select,", path).Start()
}

func main() {
	app := &App{gh: signing.GitHub{Repo: "mixeme/wynd"}}
	err := wails.Run(&options.App{
		Title:            "Wynd Подпись",
		Width:            640,
		Height:           560,
		MinWidth:         560,
		MinHeight:        480,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 0xF4, G: 0xF0, B: 0xE9, A: 0xFF},
		Frameless:        true,
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []any{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
