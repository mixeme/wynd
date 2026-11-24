package signing

// Android-выпуск (план 65): проверка и подпись APK — в окне «Wynd Подпись».
// Инструменты —
// из среды Android владельца (план 50: всё в D:\android-env); пароль keystore
// передаётся apksigner переменной окружения, не аргументом вызова. Имя флага
// releaseReady на экране не пишется — решение владельца 19.11.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// Пакет приложения в APK, файлы тега с описанием, готовностью и эталонным
// сертификатом подписи.
const (
	androidAPKPackage   = "mixeme.wynd"
	androidVersionProps = "android/version.properties"
	// androidCertPath — открытая часть ключа подписи (экспорт keytool
	// -exportcert): по ней подписанный APK сверяется с эталоном.
	androidCertPath = "android/wynd-android.cert.pem"
)

// Тексты итога для окна (Result.Detail) и объяснение черновика без публикации.
const (
	androidDetailPublished = "Файл wynd-android.apk и его контрольная сумма выложены в выпуск на GitHub. Подпись APK сверена с эталонным сертификатом из тега. Временные файлы на этом компьютере удалены."
	androidDetailBlocked   = "Файл wynd-android.apk и его контрольная сумма выложены в черновик. Выпуск не опубликован: публикация заблокирована. Временные файлы на этом компьютере удалены."
)

// androidAssetNote — пояснение к файлам выпуска. Рядом с подписанным APK
// в выпуске остаётся исходный unsigned: подпись встраивается в файл, и по
// исходному проверяется удостоверение сборки. Без строки два APK на странице
// выпуска выглядят как выбор.
const androidAssetNote = "\n\nУстанавливать нужно **`wynd-android.apk`**. Файл `wynd-android-unsigned.apk` — тот же APK до подписи, для проверки удостоверения сборки; установить его нельзя."

// androidReleaseBody — описание выпуска: «Что нового» и пояснение про файлы.
func androidReleaseBody(notes string) string {
	return notes + androidAssetNote + "\n"
}

// androidKeyAlias — alias постоянного ключа в keystore (план 50).
const androidKeyAlias = "wynd-android"

// androidBlocked — почему подписанный выпуск останется черновиком.
const androidBlocked = "Публикация выпуска заблокирована. Подпишем и выложим подписанный APK в черновик — опубликовать его можно будет позже."

// androidVersionCode — код версии из тега, как в android/app/build.gradle и
// в скрипте подписи: X.Y.Z → X·1 000 000 + Y·1 000 + Z.
func androidVersionCode(version string) (int64, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return 0, fmt.Errorf("версия %q не из трёх чисел", version)
	}
	var nums [3]int64
	for i, p := range parts {
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil || n < 0 || i > 0 && p != strconv.FormatInt(n, 10) {
			return 0, fmt.Errorf("версия %q не разбирается на числа", version)
		}
		nums[i] = n
	}
	if nums[1] > 999 || nums[2] > 999 {
		return 0, fmt.Errorf("версия %q переполняет код версии", version)
	}
	code := nums[0]*1_000_000 + nums[1]*1_000 + nums[2]
	if code <= 0 || code > 2_100_000_000 {
		return 0, fmt.Errorf("код версии %s выходит за пределы Android", version)
	}
	return code, nil
}

var (
	badgingNameRe  = regexp.MustCompile(`(?m)^package: name='([^']*)'`)
	badgingCodeRe  = regexp.MustCompile(`versionCode='(\d+)'`)
	badgingNameVer = regexp.MustCompile(`versionName='([^']*)'`)
	badgingDebugRe = regexp.MustCompile(`(?m)^application-debuggable`)
)

// badging — что окно сверяет с выпуском: пакет, версии и признак отладки из
// aapt dump badging.
type badging struct {
	Name        string
	VersionCode string
	VersionName string
	Debuggable  bool
}

// parseBadging разбирает вывод aapt dump badging; отсутствие любой строки —
// нечитаемый APK.
func parseBadging(raw string) (badging, error) {
	var b badging
	read := func(re *regexp.Regexp, what string) string {
		m := re.FindStringSubmatch(raw)
		if m == nil {
			return "\x00"
		}
		return m[1]
	}
	b.Name = read(badgingNameRe, "пакет")
	b.VersionCode = read(badgingCodeRe, "versionCode")
	b.VersionName = read(badgingNameVer, "versionName")
	b.Debuggable = badgingDebugRe.MatchString(raw)
	for _, pair := range []struct{ v, what string }{
		{b.Name, "имя пакета"}, {b.VersionCode, "versionCode"}, {b.VersionName, "versionName"},
	} {
		if pair.v == "\x00" {
			return b, fmt.Errorf("в badging нет %s", pair.what)
		}
	}
	return b, nil
}

// androidReleaseReady — тег помечен готовым к публикации.
var releaseReadyRe = regexp.MustCompile(`(?m)^releaseReady=true\s*$`)

func androidReleaseReady(raw string) bool {
	return releaseReadyRe.MatchString(raw)
}

// androidTools — вызываемые инструменты среды Android.
type androidTools struct {
	JavaHome  string // корень JDK: JAVA_HOME вызова
	JavaBin   string // каталог java: начало PATH вызова
	AAPT      string
	Zipalign  string
	Apksigner string
}

// toolEnv — окружение вызова: JAVA_HOME и bin JDK в начале PATH; extra
// добавляет переменные (пароль keystore передаётся только так).
func (t *androidTools) toolEnv(extra ...string) []string {
	env := append([]string{}, os.Environ()...)
	env = append(env, "JAVA_HOME="+t.JavaHome)
	env = append(env, "PATH="+t.JavaBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return append(env, extra...)
}

// toolSuffix — на компьютере владельца инструменты Windows (.exe); в тестах
// на Unix — без суффикса. apksigner на Windows — батник.
func toolSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func apksignerName() string {
	if runtime.GOOS == "windows" {
		return "apksigner.bat"
	}
	return "apksigner"
}

// newestToolDir — последняя по имени папка из base/pattern, где лежит tool;
// по убыванию имени, как в скриптах подготовки среды.
func newestToolDir(base, pattern, tool string) (string, error) {
	dirs, err := filepath.Glob(filepath.Join(base, pattern))
	if err != nil {
		return "", err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	for _, d := range dirs {
		p := filepath.Join(d, tool)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("в %s нет %s", base, tool)
}

// findAndroidTools — JDK 17 и build-tools из среды Android владельца.
func findAndroidTools(envRoot string) (*androidTools, error) {
	if envRoot == "" {
		// Среда Android живёт в одном месте (решение владельца, план 50).
		envRoot = `D:\android-env`
	}
	javaPath, err := newestToolDir(filepath.Join(envRoot, "jdk"), "jdk-17*", filepath.Join("bin", "java"+toolSuffix()))
	if err != nil {
		return nil, fmt.Errorf("среда Android не найдена: %w", err)
	}
	buildTools := filepath.Join(envRoot, "sdk", "build-tools")
	aaptPath, err := newestToolDir(buildTools, "*", "aapt"+toolSuffix())
	if err != nil {
		return nil, fmt.Errorf("среда Android не найдена: %w", err)
	}
	zipPath, err := newestToolDir(buildTools, "*", "zipalign"+toolSuffix())
	if err != nil {
		return nil, fmt.Errorf("среда Android не найдена: %w", err)
	}
	signPath, err := newestToolDir(buildTools, "*", apksignerName())
	if err != nil {
		return nil, fmt.Errorf("среда Android не найдена: %w", err)
	}
	return &androidTools{
		JavaHome:  filepath.Dir(filepath.Dir(javaPath)),
		JavaBin:   filepath.Dir(javaPath),
		AAPT:      aaptPath,
		Zipalign:  zipPath,
		Apksigner: signPath,
	}, nil
}

// runTool запускает инструмент среды, возвращая stdout; отказ — с текстом
// stderr. Пароль в аргументах вызова не появляется никогда.
func runTool(ctx context.Context, path string, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	hideWindow(cmd)
	cmd.Env = env
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return out.String(), errors.New(msg)
	}
	return out.String(), nil
}

// isWrongKeyStorePassword — apksigner не открыл keystore из-за пароля; окно
// покажет «Пароль не подошёл», как у exe.
func isWrongKeyStorePassword(err error) bool {
	if err == nil {
		return false
	}
	// Сообщение Java-keystore о неверном пароле — не локализуется.
	return strings.Contains(err.Error(), "password was incorrect")
}

var apkCertDigestRe = regexp.MustCompile(`(?mi)^Signer #1 certificate SHA-256 digest:\s*([0-9a-f]+)`)

// apkCertDigest — отпечаток сертификата подписи из вывода apksigner
// verify --print-certs.
func apkCertDigest(out string) (string, error) {
	m := apkCertDigestRe.FindStringSubmatch(out)
	if m == nil {
		return "", errors.New("отпечаток сертификата не найден")
	}
	return m[1], nil
}

// androidCertDigest — отпечаток эталонного сертификата из тега: PEM → DER →
// SHA-256. Без эталона подписывать нельзя: нечем сверить ключ.
func (g GitHub) androidCertDigest(ctx context.Context, tag string) (string, error) {
	raw, err := g.fileE(ctx, androidCertPath, tag)
	if err != nil {
		return "", fmt.Errorf("проверить ключ подписи не получилось: %w", err)
	}
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("в теге нет эталонного сертификата %s", androidCertPath)
	}
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return "", fmt.Errorf("эталонный сертификат %s не читается", androidCertPath)
	}
	sum := sha256.Sum256(block.Bytes)
	return hex.EncodeToString(sum[:]), nil
}

// androidCheck — после удостоверения сборки: пакет и версия по badging,
// выравнивание, описание и готовность к публикации. Отказ ставит c.Problem
// и останавливает проверку, как у exe.
func (g GitHub) androidCheck(ctx context.Context, c *Check, prod *product, p *progress, draft bool) {
	version := strings.TrimPrefix(c.Tag, prod.Prefix)
	code, err := androidVersionCode(version)
	if err != nil {
		c.Problem = err.Error()
		return
	}
	tools, err := findAndroidTools(g.AndroidEnv)
	if err != nil {
		c.Problem = err.Error()
		return
	}
	p.start("package")
	out, err := runTool(ctx, tools.AAPT, tools.toolEnv(), "dump", "badging", c.exe)
	if err != nil {
		c.Problem = "Проверить пакет и версию не получилось: " + err.Error()
		return
	}
	b, err := parseBadging(out)
	if err != nil {
		c.Problem = "Проверить пакет и версию не получилось: " + err.Error()
		return
	}
	if b.Name != androidAPKPackage || b.VersionName != version || b.VersionCode != strconv.FormatInt(code, 10) || b.Debuggable {
		c.Problem = "Пакет или версия не совпадают с выпуском " + c.Tag
		return
	}
	p.done("package", "")
	p.start("align")
	if _, err := runTool(ctx, tools.Zipalign, tools.toolEnv(), "-c", "4", c.exe); err != nil {
		c.Problem = "APK не выровнен: " + err.Error()
		return
	}
	p.done("align", "")
	p.start("notes")
	raw, err := g.fileE(ctx, prod.Notes, c.Tag)
	if err != nil {
		c.Problem = "Прочитать описание выпуска не получилось: " + err.Error()
		return
	}
	c.Notes = WhatsNew(raw, version)
	if c.Notes == "" {
		c.Problem = "В " + prod.Notes + " тега нет раздела " + version
		return
	}
	p.done("notes", "")
	p.start("readiness")
	raw, err = g.fileE(ctx, androidVersionProps, c.Tag)
	if err != nil {
		c.Problem = "Проверить готовность к публикации не получилось: " + err.Error()
		return
	}
	ready := androidReleaseReady(raw)
	if ready {
		p.done("readiness", "")
	} else {
		p.done("readiness", "публикация заблокирована")
	}
	if draft && !ready {
		c.PublishNote = androidBlocked
	}
	c.Publishes = draft && ready
}

// signAndPublishAndroid — подпись apksigner, сверка сертификата с эталоном
// из тега, загрузка; публикация, только когда выпуск готов к ней.
func (g GitHub) signAndPublishAndroid(ctx context.Context, c *Check, keyPath, pass string, p *progress) (published bool, detail string, err error) {
	prod := productOf(c.Tag)
	p.start("release")
	if c.Problem != "" {
		return false, "", errors.New(c.Problem)
	}
	// За время проверок выпуск мог поменяться другим способом; подписанный
	// APK не перезаписываем — прежняя подпись остаётся (правило скрипта и CI).
	draft, alreadySigned, err := g.androidReleaseState(ctx, c.Tag)
	if err != nil {
		return false, "", err
	}
	if alreadySigned {
		return false, "", errors.New("В выпуске уже есть подписанный APK; не перезаписываю: прежняя подпись остаётся")
	}
	if draft && c.Publishes {
		p.value.Steps = append(p.value.Steps, Step{ID: "publish", Title: "Публикуем выпуск", Status: "pending"})
	}
	p.done("release", "")

	p.start("signature")
	tools, err := findAndroidTools(g.AndroidEnv)
	if err != nil {
		return false, "", err
	}
	signed := filepath.Join(c.dir, prod.Exe)
	// Пароль keystore — переменная окружения процесса apksigner: в аргументах
	// вызова он был бы виден другим процессам того же пользователя.
	env := tools.toolEnv("WYND_KS_PASS=" + pass)
	if _, err := runTool(ctx, tools.Apksigner, env,
		"sign", "--ks", keyPath, "--ks-key-alias", prod.KeyAlias,
		"--ks-pass", "env:WYND_KS_PASS", "--out", signed, c.exe); err != nil {
		if isWrongKeyStorePassword(err) {
			return false, "", ErrWrongPassword
		}
		return false, "", fmt.Errorf("подпись APK не получилась: %w", err)
	}
	out, err := runTool(ctx, tools.Apksigner, tools.toolEnv(), "verify", "--print-certs", signed)
	if err != nil {
		return false, "", fmt.Errorf("проверить подпись APK не получилось: %w", err)
	}
	if strings.Contains(out, "CN=Android Debug") {
		return false, "", errors.New("подпись отладочным ключом запрещена: публичный выпуск подписывается постоянным ключом")
	}
	digest, err := apkCertDigest(out)
	if err != nil {
		return false, "", fmt.Errorf("в выводе apksigner нет отпечатка сертификата: %w", err)
	}
	want, err := g.androidCertDigest(ctx, c.Tag)
	if err != nil {
		return false, "", err
	}
	if !strings.EqualFold(digest, want) {
		return false, "", errors.New("сертификат подписи не совпал с эталонным из тега — подписывать этим ключом нельзя")
	}
	p.done("signature", "")

	p.start("upload")
	apk, err := os.ReadFile(signed)
	if err != nil {
		return false, "", err
	}
	sum := sha256.Sum256(apk)
	sumPath := signed + ".sha256"
	if err := os.WriteFile(sumPath, []byte(hex.EncodeToString(sum[:])+"\n"), 0o644); err != nil {
		return false, "", err
	}
	if _, err := g.gh(ctx, "release", "upload", c.Tag, signed, sumPath, "-R", g.Repo, "--clobber"); err != nil {
		return false, "", fmt.Errorf("выложить подписанный APK не получилось: %w", err)
	}
	p.done("upload", "")

	if !draft {
		return true, androidDetailPublished, nil
	}
	if !c.Publishes {
		return false, androidDetailBlocked, nil
	}
	p.start("publish")
	notes := filepath.Join(c.dir, "notes.md")
	if err := os.WriteFile(notes, []byte(androidReleaseBody(c.Notes)), 0o644); err != nil {
		return false, "", err
	}
	if _, err := g.gh(ctx, "release", "edit", c.Tag, "-R", g.Repo, "--draft=false",
		"--title", "Wynd Android "+strings.TrimPrefix(c.Tag, prod.Prefix), "--notes-file", notes); err != nil {
		return false, "", fmt.Errorf("подпись выложена, а опубликовать не получилось: %w", err)
	}
	p.done("publish", "")
	return true, androidDetailPublished, nil
}

// androidReleaseState — состояние выпуска одним запросом: черновик ли и есть
// ли уже подписанный APK (который окно не перезаписывает).
func (g GitHub) androidReleaseState(ctx context.Context, tag string) (draft, signed bool, err error) {
	out, err := g.gh(ctx, "release", "view", tag, "-R", g.Repo, "--json", "isDraft,assets")
	if err != nil {
		return false, false, err
	}
	var view struct {
		Draft  *bool `json:"isDraft"`
		Assets []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(out, &view); err != nil {
		return false, false, err
	}
	if view.Draft == nil {
		return false, false, errors.New("GitHub не сообщил, опубликован ли выпуск")
	}
	want := productOf(tag).signedAsset()
	for _, a := range view.Assets {
		if a.Name == want {
			signed = true
		}
	}
	return *view.Draft, signed, nil
}

// CreateAndroidKey — новый keystore владельца: те же параметры, что в
// scripts\new-android-signing-key.ps1 (PKCS12, RSA 3072, SHA256withRSA,
// срок 10000 дней, CN=Wynd Android). Пароль keytool читает из stdin,
// не из аргументов вызова.
func CreateAndroidKey(envRoot, keyPath, pass string) error {
	if _, err := os.Stat(keyPath); err == nil {
		return ErrKeyExists
	}
	if len([]rune(pass)) < MinPassword {
		return ErrShortPassword
	}
	tools, err := findAndroidTools(envRoot)
	if err != nil {
		return err
	}
	keytool := filepath.Join(tools.JavaBin, "keytool"+toolSuffix())
	cmd := exec.Command(keytool,
		"-genkeypair", "-storetype", "PKCS12", "-keystore", keyPath,
		"-alias", androidKeyAlias,
		"-keyalg", "RSA", "-keysize", "3072", "-sigalg", "SHA256withRSA",
		"-validity", "10000", "-dname", "CN=Wynd Android")
	hideWindow(cmd)
	cmd.Env = tools.toolEnv()
	// PKCS12 хранит ключ под паролем хранилища: keytool спрашивает его дважды.
	cmd.Stdin = strings.NewReader(pass + "\n" + pass + "\n")
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("keytool не создал ключ: %s", msg)
	}
	return nil
}
