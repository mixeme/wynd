package signing

// Проверки Android-потока (план 65): шаги проверки выпуска, подпись apksigner,
// сверка сертификата с эталоном тега и блокировка публикации. Инструменты и gh
// играет сам тестовый бинарник (см. TestMain в release_test.go) — без сети,
// среды Android и настоящего ключа владельца.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const androidTestTag = "android-v0.1.0"

func TestAndroidVersionCode(t *testing.T) {
	for _, tt := range []struct {
		version string
		code    int64
		ok      bool
	}{
		{"0.1.0", 1_000, true},
		{"0.0.1", 1, true},
		{"1.2.3", 1_002_003, true},
		{"2100.0.0", 2_100_000_000, true},
		{"0.1", 0, false},
		{"0.1.0.0", 0, false},
		{"0.01.0", 0, false},
		{"0.1.00", 0, false},
		{"0.0.0", 0, false},
		{"0.1000.0", 0, false},
		{"2147.0.0", 0, false},
		{"a.b.c", 0, false},
		{"0.1.-1", 0, false},
	} {
		code, err := androidVersionCode(tt.version)
		if tt.ok && (err != nil || code != tt.code) {
			t.Fatalf("%s: code=%d err=%v", tt.version, code, err)
		}
		if !tt.ok && err == nil {
			t.Fatalf("%s: принято без ошибки", tt.version)
		}
	}
}

func TestParseBadging(t *testing.T) {
	b, err := parseBadging(androidBadging("mixeme.wynd", "1000", "0.1.0", false))
	if err != nil || b.Name != "mixeme.wynd" || b.VersionCode != "1000" || b.VersionName != "0.1.0" || b.Debuggable {
		t.Fatalf("badging: %+v err=%v", b, err)
	}
	if b, err := parseBadging(androidBadging("mixeme.wynd", "1000", "0.1.0", true)); err != nil || !b.Debuggable {
		t.Fatalf("debuggable не распознан: %+v err=%v", b, err)
	}
	if _, err := parseBadging("package: name='mixeme.wynd'\n"); err == nil {
		t.Fatal("неполный badging принят")
	}
}

func TestAndroidReleaseReady(t *testing.T) {
	if androidReleaseReady("releaseReady=false\nversionName=0.1.0") {
		t.Fatal("releaseReady=false прочитан как готовность")
	}
	if !androidReleaseReady("versionName=0.1.0\r\nreleaseReady=true\r\n") {
		t.Fatal("releaseReady=true не распознан")
	}
	if androidReleaseReady("versionName=0.1.0") {
		t.Fatal("без строки флага прочитано как готовность")
	}
}

// androidBadging — строка «package:» из вывода aapt dump badging.
func androidBadging(name, versionCode, versionName string, debug bool) string {
	out := fmt.Sprintf("package: name='%s' versionCode='%s' versionName='%s' compileSdkVersion='36'\n", name, versionCode, versionName)
	if debug {
		out += "application-debuggable:true\n"
	}
	return out
}

// certFixture — эталонный сертификат и его отпечаток, согласованные заранее.
func certFixture() (string, string) {
	der := []byte("test-certificate-der-bytes")
	pemStr := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	sum := sha256.Sum256(der)
	return pemStr, hex.EncodeToString(sum[:])
}

// androidFixture — согласованное окружение проверки android-v0.1.0: gh,
// badging, выравнивание, keystore-пароль, эталонный сертификат, готовность.
func androidFixture() (releaseFixture, string, string) {
	pemStr, digest := certFixture()
	f := releaseFixture{
		Draft: true, Notes: true, Tag: androidTestTag,
		Badging:      androidBadging("mixeme.wynd", "1000", "0.1.0", false),
		KSPass:       "ks-password",
		CertPEM:      pemStr,
		CertDigest:   digest,
		VersionProps: "versionName=0.1.0\nreleaseReady=true\n",
	}
	return f, pemStr, digest
}

// androidEnv — fake-инструменты рядом с fixture gh: тот же fixture.json.
func androidEnv(t *testing.T, dir string) {
	t.Helper()
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("WYND_SIGNING_TEST_APK", dir)
	jdk := filepath.Join(dir, "jdk", "jdk-17.0.1", "bin")
	if err := os.MkdirAll(jdk, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jdk, "java"+toolSuffix()), []byte("java marker"), 0o700); err != nil {
		t.Fatal(err)
	}
	bt := filepath.Join(dir, "sdk", "build-tools", "35.0.0")
	if err := os.MkdirAll(bt, 0o700); err != nil {
		t.Fatal(err)
	}
	copyBin := func(name string) {
		raw, err := os.ReadFile(bin)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(bt, name), raw, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if runtime.GOOS == "windows" {
		copyBin("aapt.exe")
		copyBin("zipalign.exe")
		copyBin("apksigner-tool.exe")
		// apksigner.bat — батник, зовущий копию тестового бинарника рядом.
		bat := "@echo off\r\n\"%~dp0apksigner-tool.exe\" %*\r\n"
		if err := os.WriteFile(filepath.Join(bt, "apksigner.bat"), []byte(bat), 0o700); err != nil {
			t.Fatal(err)
		}
		return
	}
	copyBin("aapt")
	copyBin("zipalign")
	copyBin("apksigner")
}

func androidTestGitHub(t *testing.T, f releaseFixture) (GitHub, string) {
	t.Helper()
	g, dir := fixtureGitHub(t, f)
	androidEnv(t, dir)
	g.AndroidEnv = dir
	return g, dir
}

func checkIDs(steps []Step) []string {
	ids := make([]string, len(steps))
	for i, s := range steps {
		ids[i] = s.ID
	}
	return ids
}

func TestAndroidCheck(t *testing.T) {
	base, _, _ := androidFixture()
	for _, tt := range []struct {
		name string
		edit func(f *releaseFixture)
		want struct {
			problem     string
			publishes   bool
			publishNote string
			readiness   string
			notes       bool
		}
	}{
		{
			name: "готов к публикации",
			edit: func(f *releaseFixture) {},
			want: struct {
				problem     string
				publishes   bool
				publishNote string
				readiness   string
				notes       bool
			}{publishes: true, notes: true},
		},
		{
			name: "публикация заблокирована",
			edit: func(f *releaseFixture) { f.VersionProps = "versionName=0.1.0\nreleaseReady=false\n" },
			want: struct {
				problem     string
				publishes   bool
				publishNote string
				readiness   string
				notes       bool
			}{publishNote: androidBlocked, readiness: "публикация заблокирована", notes: true},
		},
		{
			name: "чужой пакет",
			edit: func(f *releaseFixture) { f.Badging = androidBadging("com.other.app", "1000", "0.1.0", false) },
			want: struct {
				problem     string
				publishes   bool
				publishNote string
				readiness   string
				notes       bool
			}{problem: "не совпадают"},
		},
		{
			name: "чужая версия",
			edit: func(f *releaseFixture) { f.Badging = androidBadging("mixeme.wynd", "1001", "0.1.0", false) },
			want: struct {
				problem     string
				publishes   bool
				publishNote string
				readiness   string
				notes       bool
			}{problem: "не совпадают"},
		},
		{
			name: "отладочная сборка",
			edit: func(f *releaseFixture) { f.Badging = androidBadging("mixeme.wynd", "1000", "0.1.0", true) },
			want: struct {
				problem     string
				publishes   bool
				publishNote string
				readiness   string
				notes       bool
			}{problem: "не совпадают"},
		},
		{
			name: "не выровнен",
			edit: func(f *releaseFixture) { f.BadAlign = true },
			want: struct {
				problem     string
				publishes   bool
				publishNote string
				readiness   string
				notes       bool
			}{problem: "не выровнен"},
		},
		{
			name: "нет раздела версии",
			edit: func(f *releaseFixture) { f.Notes = false },
			want: struct {
				problem     string
				publishes   bool
				publishNote string
				readiness   string
				notes       bool
			}{problem: "нет раздела"},
		},
		{
			name: "готовность не прочиталась",
			edit: func(f *releaseFixture) { f.VersionProps = "" },
			want: struct {
				problem     string
				publishes   bool
				publishNote string
				readiness   string
				notes       bool
			}{problem: "готовность к публикации не получилось", notes: true},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := base
			tt.edit(&f)
			g, dir := androidTestGitHub(t, f)
			var checks []Progress
			c, err := g.FetchWithProgress(context.Background(), androidTestTag, func(p Progress) { checks = append(checks, p) })
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			if tt.want.problem != "" && !strings.Contains(c.Problem, tt.want.problem) {
				t.Fatalf("problem: %q", c.Problem)
			}
			if tt.want.problem == "" && c.Problem != "" {
				t.Fatalf("unexpected problem: %q", c.Problem)
			}
			if c.Publishes != tt.want.publishes || c.PublishNote != tt.want.publishNote {
				t.Fatalf("publishes=%v note=%q", c.Publishes, c.PublishNote)
			}
			if tt.want.notes != strings.Contains(c.Notes, "Test release notes") {
				t.Fatalf("notes: %q", c.Notes)
			}
			var readiness *Step
			for i := range c.Steps {
				if c.Steps[i].ID == "readiness" {
					readiness = &c.Steps[i]
				}
			}
			if tt.want.readiness != "" && (readiness == nil || readiness.Note != tt.want.readiness) {
				t.Fatalf("readiness step: %+v", readiness)
			}
			if len(checks) == 0 || checks[len(checks)-1].Tag != androidTestTag {
				t.Fatalf("progress: %+v", checks)
			}
			// Журнал вызовов gh не содержит пароля keystore.
			if calls, err := os.ReadFile(filepath.Join(dir, "calls")); err != nil || strings.Contains(string(calls), f.KSPass) {
				t.Fatalf("calls leak or read: %v", err)
			}
		})
	}
}

// Порядок шагов проверки Android-выпуска: как в принятых кадрах.
func TestAndroidCheckStepsOrder(t *testing.T) {
	f, _, _ := androidFixture()
	g, _ := androidTestGitHub(t, f)
	c, err := g.FetchWithProgress(context.Background(), androidTestTag, func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if got := strings.Join(checkIDs(c.Steps), ","); got != "release,download,checksum,attestation,package,align,notes,readiness" {
		t.Fatalf("steps: %s", got)
	}
	for _, s := range c.Steps {
		if s.Status != "done" {
			t.Fatalf("step %s: %s", s.ID, s.Status)
		}
	}
	if !c.Publishes || c.PublishNote != "" {
		t.Fatalf("publishes=%v note=%q", c.Publishes, c.PublishNote)
	}
}

// Подпись готового черновика: apksigner, сверка с эталоном, загрузка и публикация;
// пароль не появляется ни в аргументах, ни в журналах.
func TestAndroidSignAndPublish(t *testing.T) {
	f, _, _ := androidFixture()
	g, dir := androidTestGitHub(t, f)
	c, err := g.FetchWithProgress(context.Background(), androidTestTag, func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var signs []Progress
	published, detail, err := g.SignAndPublishWithProgress(context.Background(), c, "keystore.p12", f.KSPass, func(p Progress) { signs = append(signs, p) })
	if err != nil || !published {
		t.Fatalf("published=%v err=%v", published, err)
	}
	if detail != androidDetailPublished {
		t.Fatalf("detail: %q", detail)
	}
	if got := strings.Join(checkIDs(signs[len(signs)-1].Steps), ","); got != "release,signature,upload,publish" {
		t.Fatalf("sign steps: %s", got)
	}
	calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
	toolCalls, _ := os.ReadFile(filepath.Join(dir, "tool-calls"))
	joined := string(calls) + string(toolCalls)
	if !strings.Contains(joined, "release upload") || !strings.Contains(joined, "release edit") {
		t.Fatalf("upload/publish calls missing: %s", joined)
	}
	if strings.Contains(joined, f.KSPass) {
		t.Fatal("keystore password leaked into call logs")
	}
}

// Заблокированная публикация: подпись и загрузка есть, публикации и этапа нет.
func TestAndroidSignBlocked(t *testing.T) {
	f, _, _ := androidFixture()
	f.VersionProps = "versionName=0.1.0\nreleaseReady=false\n"
	g, dir := androidTestGitHub(t, f)
	c, err := g.FetchWithProgress(context.Background(), androidTestTag, func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.Publishes || c.PublishNote == "" {
		t.Fatalf("check state: publishes=%v note=%q", c.Publishes, c.PublishNote)
	}
	published, detail, err := g.SignAndPublishWithProgress(context.Background(), c, "keystore.p12", f.KSPass, func(Progress) {})
	if err != nil || published {
		t.Fatalf("published=%v err=%v", published, err)
	}
	if detail != androidDetailBlocked {
		t.Fatalf("detail: %q", detail)
	}
	calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
	if strings.Contains(string(calls), "release edit") {
		t.Fatalf("blocked release was published: %s", calls)
	}
}

// Неверный пароль keystore узнаваем окном: «Пароль не подошёл».
func TestAndroidSignWrongPassword(t *testing.T) {
	f, _, _ := androidFixture()
	g, _ := androidTestGitHub(t, f)
	c, err := g.FetchWithProgress(context.Background(), androidTestTag, func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, _, err := g.SignAndPublishWithProgress(context.Background(), c, "keystore.p12", "не тот пароль", func(Progress) {}); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("wrong password not recognized: %v", err)
	}
}

// Подпись не совпавшим с эталоном ключом запрещена.
func TestAndroidSignCertMismatch(t *testing.T) {
	f, _, _ := androidFixture()
	f.CertDigest = strings.Repeat("ab", 32)
	g, _ := androidTestGitHub(t, f)
	c, err := g.FetchWithProgress(context.Background(), androidTestTag, func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_, _, err = g.SignAndPublishWithProgress(context.Background(), c, "keystore.p12", f.KSPass, func(Progress) {})
	if err == nil || !strings.Contains(err.Error(), "эталонным") {
		t.Fatalf("mismatch not rejected: %v", err)
	}
}

// Отладочный сертификат отвергается до сверки с эталоном.
func TestAndroidSignDebugCert(t *testing.T) {
	f, _, _ := androidFixture()
	f.BadDebug = true
	g, _ := androidTestGitHub(t, f)
	c, err := g.FetchWithProgress(context.Background(), androidTestTag, func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_, _, err = g.SignAndPublishWithProgress(context.Background(), c, "keystore.p12", f.KSPass, func(Progress) {})
	if err == nil || !strings.Contains(err.Error(), "отладочным") {
		t.Fatalf("debug cert not rejected: %v", err)
	}
}

// В теге без эталонного сертификата подписывать нельзя: нечем сверить ключ.
func TestAndroidSignWithoutReference(t *testing.T) {
	f, _, _ := androidFixture()
	f.CertPEM = ""
	g, _ := androidTestGitHub(t, f)
	c, err := g.FetchWithProgress(context.Background(), androidTestTag, func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_, _, err = g.SignAndPublishWithProgress(context.Background(), c, "keystore.p12", f.KSPass, func(Progress) {})
	if err == nil || !strings.Contains(err.Error(), "проверить ключ подписи не получилось") {
		t.Fatalf("missing reference not rejected: %v", err)
	}
}

// Списание сил: fixture gh пишет файл по downloadName, а не по Exe.
func TestAndroidDownloadName(t *testing.T) {
	prod := productOf(androidTestTag)
	if prod.downloadName() != "wynd-android-unsigned.apk" || prod.signedAsset() != "wynd-android.apk" {
		t.Fatalf("android product names: %+v", prod)
	}
}
