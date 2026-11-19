//go:build dev || desktop || bindings

// «Wynd Подпись» (план 45, пункт 3а; кадры 12а–12д): окно, где владелец
// создаёт ключ подписи выпусков Wynd и подписывает их — Мастерскую и окно
// для компьютера (план 63). Отдельно от Мастерской: подпись нужна только
// тому, кто её выпускает, а ключ не должен жить в программе, которую раздают
// людям. Сами ключ и подпись — internal/signing (их же зовёт cmd/workshop-sign).
// Собирается scripts\build-signer.bat (`wails build`).
package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"gitea.mixdep.ru/mix/wynd/internal/signing"
)

//go:embed all:frontend/dist
var assets embed.FS

// App — то, что окно может попросить (window.go.main.App.*). Пароль ключа
// дальше signing.Unlock не уходит и нигде не сохраняется.
type releaseBackend interface {
	Releases(context.Context) ([]signing.Release, error)
	FetchWithProgress(context.Context, string, func(signing.Progress)) (*signing.Check, error)
	SignAndPublishWithProgress(context.Context, *signing.Check, string, string, func(signing.Progress)) (bool, string, error)
	ReleaseURL(string) string
}

type App struct {
	ctx  context.Context
	gh   releaseBackend
	emit func(signing.Progress)

	cancel        context.CancelFunc
	operationOnce sync.Once
	operationGate chan struct{}
	closed        bool
	check         *signing.Check
}

// ServiceStartup и ServiceShutdown — хуки Wails v3: приложение передаёт
// контекст, который отменяется при закрытии окна.
func (a *App) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	a.ctx, a.cancel = context.WithCancel(ctx)
	return nil
}

func (a *App) acquireOperation(ctx context.Context) error {
	a.operationOnce.Do(func() { a.operationGate = make(chan struct{}, 1) })
	select {
	case a.operationGate <- struct{}{}:
		if err := ctx.Err(); err != nil {
			a.releaseOperation()
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (a *App) releaseOperation() { <-a.operationGate }

func (a *App) ServiceShutdown() error {
	if a.cancel != nil {
		a.cancel()
	}
	_ = a.acquireOperation(context.Background())
	defer a.releaseOperation()
	a.closed = true
	a.check.Close()
	a.check = nil
	return nil
}

// settings — что окно помнит на этом компьютере: где лежит ключ (12а, 12б),
// файл ключа Android и среда для aapt/zipalign/apksigner (план 65).
type settings struct {
	KeyPath string `json:"key_path"`
	// KeyStorePath — файл постоянного ключа Android (.p12).
	KeyStorePath string `json:"key_store_path"`
	// AndroidEnv — среда Android; пусто — D:\android-env (план 50).
	AndroidEnv string `json:"android_env,omitempty"`
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
	if s.KeyStorePath == "" {
		// По умолчанию — путь scripts\sign-android-release.ps1 (план 50:
		// постоянный ключ держит владелец вне репозитория).
		s.KeyStorePath = `D:\Local\Git\myenv\wynd\wynd-android.p12`
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

// repoRoot — папка репозитория Wynd: туда (signing-key.pub, общий ключ
// экосистемы) ложится открытый ключ, его коммитят. Ищем вверх от exe и от
// рабочей папки.
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

// State — с чего начать (12а или 12б) и как настроены ключи (план 65).
type State struct {
	KeyPath   string `json:"key_path"`
	KeyExists bool   `json:"key_exists"`
	// PubPath — куда ляжет открытый ключ; пусто — репозиторий не найден, и он
	// ляжет рядом с ключом (.pub).
	PubPath string `json:"pub_path"`
	// KeyStorePath — файл ключа Android; KeyStoreExists — он выбран и найден.
	KeyStorePath   string `json:"key_store_path"`
	KeyStoreExists bool   `json:"key_store_exists"`
}

func pubPath(keyPath string) string {
	if p := os.Getenv("WYND_SIGNER_PUB"); p != "" {
		return p
	}
	if root := repoRoot(); root != "" {
		return filepath.Join(root, "signing-key.pub")
	}
	return keyPath + ".pub"
}

func (a *App) State() State {
	s := loadSettings()
	_, err := os.Stat(s.KeyPath)
	storeExists := false
	if s.KeyStorePath != "" {
		if _, statErr := os.Stat(s.KeyStorePath); statErr == nil {
			storeExists = true
		}
	}
	return State{
		KeyPath: s.KeyPath, KeyExists: err == nil, PubPath: pubPath(s.KeyPath),
		KeyStorePath: s.KeyStorePath, KeyStoreExists: storeExists,
	}
}

// Отмена диалога в Wails v3 — ошибка внутренней библиотеки
// (internal/go-common-file-dialog: «cancelled by user»), а не пустой путь, как
// было в v2. Наружу её не отдаём: отмена — это отказ, а не сбой. Импортировать
// ту библиотеку нельзя (internal модуля Wails), поэтому сверяем текст.
func dialogCancelled(err error) bool {
	return err != nil && err.Error() == "cancelled by user"
}

// ChooseKeyPlace — где хранить новый ключ (12а).
func (a *App) ChooseKeyPlace(current string) (string, error) {
	p, err := application.Get().Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title: "Где хранить ключ", Directory: keyDialogDirectory(current), Filename: filepath.Base(current),
	}).PromptForSingleSelection()
	if dialogCancelled(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("Не удалось открыть окно выбора файла: %w", err)
	}
	return p, nil
}

// ChooseKey — другой файл ключа (12б: «другой»); выбранное окно запоминает.
func (a *App) ChooseKey() (State, error) {
	s := loadSettings()
	p, err := application.Get().Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title: "Файл ключа подписи", Directory: keyDialogDirectory(s.KeyPath),
	}).PromptForSingleSelection()
	if dialogCancelled(err) {
		return a.State(), nil
	}
	if err != nil {
		return State{}, fmt.Errorf("Не удалось открыть окно выбора файла: %w", err)
	}
	if p != "" {
		s.KeyPath = p
		saveSettings(s)
	}
	return a.State(), nil
}

// Wails не открывает диалог, если DefaultDirectory не существует. При первом
// запуске .wynd ещё нет; сохранённая папка также могла быть удалена. Предлагаем
// ближайшего существующего родителя, не создавая папок при выборе пути.
func keyDialogDirectory(keyPath string) string {
	for dir := filepath.Dir(keyPath); ; {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "" // Системный диалог сам выберет начальную папку.
		}
		dir = parent
	}
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
	PubPath   string         `json:"pub_path,omitempty"`
	Published bool           `json:"published,omitempty"`
	URL       string         `json:"url,omitempty"`
	Steps     []signing.Step `json:"steps,omitempty"`
	// Detail — готовый текст итога подписи для Android; у exe пусто.
	Detail string `json:"detail,omitempty"`
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

// CheckRelease — проверки выпуска до пароля (12в).
func (a *App) CheckRelease(tag, requestID string) (*signing.Check, error) {
	if err := a.acquireOperation(a.ctx); err != nil {
		return nil, err
	}
	defer a.releaseOperation()
	if a.closed {
		return nil, errors.New("Приложение закрывается")
	}
	a.check.Close()
	a.check = nil
	checkID := rand.Text()
	c, err := a.gh.FetchWithProgress(a.ctx, tag, func(p signing.Progress) { p.CheckID = checkID; p.RequestID = requestID; a.reportProgress(p) })
	if err != nil {
		return nil, err
	}
	c.CheckID = checkID
	c.RequestID = requestID
	a.check = c
	return c, nil
}

func (a *App) reportProgress(p signing.Progress) {
	if a.ctx.Err() != nil {
		return
	}
	if a.emit != nil {
		a.emit(p)
		return
	}
	runtime := application.Get()
	if runtime == nil {
		return
	}
	runtime.Event.Emit("signer:progress", p)
}

// SignRelease — «Подписать и опубликовать» (12в → 12г).
func (a *App) SignRelease(checkID, tag, requestID, pass string) Result {
	if err := a.acquireOperation(a.ctx); err != nil {
		return fail(err)
	}
	defer a.releaseOperation()
	c := a.check
	if a.closed || c == nil || c.CheckID != checkID || c.Tag != tag || c.Problem != "" {
		return Result{Message: "Проверка выпуска устарела. Проверьте выпуск заново"}
	}
	var steps []signing.Step
	published, detail, err := a.gh.SignAndPublishWithProgress(a.ctx, c, loadSettings().KeyPath, pass, func(p signing.Progress) {
		p.CheckID = checkID
		p.RequestID = requestID
		steps = p.Steps
		a.reportProgress(p)
	})
	if err != nil {
		res := fail(err)
		res.Steps = steps
		return res
	}
	c.Close()
	a.check = nil
	return Result{OK: true, Published: published, URL: a.gh.ReleaseURL(c.Tag), Steps: steps, Detail: detail}
}

// ChooseKeystore — файл ключа Android (план 65): на экране «Ключи» или на
// экране Android-выпуска; выбор окно запоминает.
func (a *App) ChooseKeystore() (State, error) {
	s := loadSettings()
	p, err := application.Get().Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title: "Файл ключа Android", Directory: keyDialogDirectory(s.KeyStorePath),
		Filters: []application.FileFilter{{DisplayName: "Ключ Android (*.p12)", Pattern: "*.p12"}},
	}).PromptForSingleSelection()
	if dialogCancelled(err) {
		return a.State(), nil
	}
	if err != nil {
		return State{}, fmt.Errorf("Не удалось открыть окно выбора файла: %w", err)
	}
	if p != "" {
		s.KeyStorePath = p
		saveSettings(s)
	}
	return a.State(), nil
}

// ChooseKeystorePlace — куда сохранить новый ключ Android (кадр 5 плана 65).
func (a *App) ChooseKeystorePlace(current string) (string, error) {
	p, err := application.Get().Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title: "Где хранить ключ Android", Directory: keyDialogDirectory(current), Filename: filepath.Base(current),
	}).PromptForSingleSelection()
	if dialogCancelled(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("Не удалось открыть окно выбора файла: %w", err)
	}
	return p, nil
}

// CreateKeystore — создать ключ Android (кадр 5): keytool из среды Android,
// те же параметры, что в scripts\new-android-signing-key.ps1.
func (a *App) CreateKeystore(keyPath, pass string) Result {
	if keyPath == "" {
		return Result{Message: "Укажите, где хранить ключ"}
	}
	if _, err := os.Stat(keyPath); err == nil {
		return Result{Message: "Файл ключа уже есть — не перезаписываю: прежний ключ пропал бы, а с ним обновления уже установленных приложений"}
	}
	if err := signing.CreateAndroidKey(loadSettings().AndroidEnv, keyPath, pass); err != nil {
		return fail(err)
	}
	s := loadSettings()
	s.KeyStorePath = keyPath
	saveSettings(s)
	return Result{OK: true, Path: keyPath}
}

// PickExe — «Подписать файл на этом компьютере» (12б).
func (a *App) PickExe() string {
	p, _ := application.Get().Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title: "Что подписать", Filters: []application.FileFilter{{DisplayName: "Программа (*.exe)", Pattern: "*.exe"}},
	}).PromptForSingleSelection()
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
		_ = application.Get().Browser.OpenURL(url)
	}
}

// ShowInFolder — «Показать в папке» (12д).
func (a *App) ShowInFolder(path string) {
	_ = exec.Command("explorer", "/select,", path).Start()
}

func main() {
	backend := &App{gh: signing.GitHub{Repo: "mixeme/wynd", AndroidEnv: loadSettings().AndroidEnv}}
	app := application.New(application.Options{
		Name:        "wynd-signer",
		Description: "Подпись выпусков Wynd",
		Services:    []application.Service{application.NewService(backend)},
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Wynd Подпись",
		Width:            640,
		Height:           560,
		MinWidth:         560,
		MinHeight:        480,
		BackgroundColour: application.NewRGB(0xF4, 0xF0, 0xE9),
		Frameless:        true,
		URL:              "/",
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
