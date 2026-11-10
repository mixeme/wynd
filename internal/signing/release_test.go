package signing

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/installer"
	"gitea.mixdep.ru/mix/wynd/internal/releasepages"
)

const releaseTestTag = "workshop-v0.3.0"

var releaseTestEXE = []byte("MZ test Workshop release")

type releaseFixture struct {
	Draft           bool
	Notes           bool
	PubKey          string
	KeyPresent      bool
	KeyFailure      bool
	ListFailure     bool
	BadSum          bool
	BadAttest       bool
	Tag             string
	Releases        string
	ReleasePages    []string
	DownloadFailure bool
	AttestFailure   bool
	UploadFailure   bool
	PublishFailure  bool
}

// Сам тестовый бинарник играет gh: одинаково на Windows и Unix, без сети,
// настоящего ключа владельца и изменений на GitHub.
func TestMain(m *testing.M) {
	if dir := os.Getenv("WYND_SIGNING_TEST_GH"); dir != "" {
		if err := releaseTestGH(dir, os.Args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func releaseTestGH(dir string, args []string) error {
	var f releaseFixture
	raw, err := os.ReadFile(filepath.Join(dir, "fixture.json"))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return err
	}
	log, err := os.OpenFile(filepath.Join(dir, "calls"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(log, strings.Join(args, " "))
	_ = log.Close()
	if err != nil {
		return err
	}
	switch {
	case args[0] == "api" && strings.Contains(args[1], "/releases?"):
		response := f.Releases
		if len(f.ReleasePages) > 0 {
			u, err := url.Parse(args[1])
			if err != nil {
				return err
			}
			page, _ := strconv.Atoi(u.Query().Get("page"))
			if page < 1 {
				page = 1
			}
			response = "[]"
			if page <= len(f.ReleasePages) {
				response = f.ReleasePages[page-1]
			}
		}
		if response == "error" {
			return fmt.Errorf("GitHub unavailable")
		}
		fmt.Print(response)
	case args[0] == "release" && args[1] == "view":
		fmt.Printf(`{"isDraft":%t}`, f.Draft)
	case args[0] == "release" && args[1] == "download":
		if f.DownloadFailure {
			return fmt.Errorf("download failed")
		}
		var dest string
		for i, arg := range args {
			if arg == "-D" {
				dest = args[i+1]
			}
		}
		prod := productOf(args[2])
		if prod == nil {
			return fmt.Errorf("download of unknown release %q", args[2])
		}
		if err := os.WriteFile(filepath.Join(dest, prod.Exe), releaseTestEXE, 0o600); err != nil {
			return err
		}
		sum := sha256.Sum256(releaseTestEXE)
		want := hex.EncodeToString(sum[:])
		if f.BadSum {
			want = "wrong sum"
		}
		return os.WriteFile(filepath.Join(dest, prod.Exe+".sha256"), []byte(want), 0o600)
	case args[0] == "attestation":
		if f.AttestFailure {
			return fmt.Errorf("attestation request failed")
		}
		tag := f.Tag
		if tag == "" {
			tag = releaseTestTag
		}
		ref := "refs/tags/" + tag
		if f.BadAttest {
			ref = "refs/tags/workshop-v0.2.0"
		}
		fmt.Printf(`[{"verificationResult":{"signature":{"certificate":{"sourceRepositoryRef":%q,"sourceRepositoryDigest":"test-commit"}}}}]`, ref)
	case args[0] == "api" && strings.Contains(args[1], "/signing-key.pub?"):
		if f.KeyFailure {
			return fmt.Errorf("key request failed")
		}
		fmt.Print(base64.StdEncoding.EncodeToString([]byte(f.PubKey)))
	case args[0] == "api" && strings.Contains(args[1], "/whatsnew.md?"):
		if f.Notes {
			tag := f.Tag
			if tag == "" {
				tag = releaseTestTag
			}
			version := strings.TrimPrefix(tag, productOf(tag).Prefix)
			fmt.Print(base64.StdEncoding.EncodeToString([]byte("## " + version + "\n\n- Test release notes\n")))
		}
	case args[0] == "api":
		if f.ListFailure {
			return fmt.Errorf("directory request failed")
		}
		if f.KeyPresent {
			fmt.Print(`[{"name":"signing-key.pub"}]`)
		} else {
			fmt.Print(`[{"name":"main.go"}]`)
		}
	case args[0] == "release" && args[1] == "upload":
		if f.UploadFailure {
			return fmt.Errorf("upload failed")
		}
		sig, err := os.ReadFile(args[3])
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, "uploaded.sig"), sig, 0o600)
	case args[0] == "release" && args[1] == "edit":
		if f.PublishFailure {
			return fmt.Errorf("publish failed")
		}
	default:
		return fmt.Errorf("unexpected gh arguments: %v", args)
	}
	return nil
}

// Статус публикации и подпись приходят одним снимком: нет отдельных
// запросов файлов, чья ошибка могла бы скрыть существующую подпись.
func TestReleasesSnapshot(t *testing.T) {
	g, dir := fixtureGitHub(t, releaseFixture{Releases: `[
		{"tag_name":"v0.45.0","draft":false,"assets":[]},
		{"tag_name":"workshop-v0.3.1","draft":false,"published_at":"2026-11-03T01:00:00Z","html_url":"https://example.com/0.3.1","assets":[{"name":"wynd-workshop.exe.sig"}]},
		{"tag_name":"workshop-v0.3.0","draft":true,"assets":[{"name":"wynd-workshop.exe.sig"}]},
		{"tag_name":"workshop-v0.2.0","draft":false,"published_at":"2026-10-26T20:18:08Z","assets":[{"name":"other.exe.sig"}]},
		{"tag_name":"desktop-v0.1.1","draft":true,"assets":[{"name":"wynd-desktop.exe.sig"}]},
		{"tag_name":"desktop-v0.1.0","draft":true,"assets":[{"name":"wynd-workshop.exe.sig"}]}
	]`})
	releases, err := g.Releases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 5 || !releases[0].Signed || releases[0].Draft || releases[0].PublishedAt != "2026-11-03T01:00:00Z" || releases[0].URL != "https://example.com/0.3.1" {
		t.Fatalf("published release: %+v", releases)
	}
	if !releases[1].Signed || !releases[1].Draft || releases[1].PublishedAt != "" || releases[2].Signed {
		t.Fatalf("draft and unsigned release: %+v", releases)
	}
	// Окно: чужая подпись (от Мастерской) не делает выпуск окна подписанным,
	// а префикс тега не путается с версией.
	if releases[3].Product != ProductDesktop || releases[3].Version != "0.1.1" || !releases[3].Signed || !releases[3].Draft {
		t.Fatalf("desktop draft: %+v", releases[3])
	}
	if releases[4].Product != ProductDesktop || releases[4].Signed {
		t.Fatalf("desktop release with a workshop signature: %+v", releases[4])
	}
	calls, err := os.ReadFile(filepath.Join(dir, "calls"))
	if err != nil || strings.Count(string(calls), "\n") != 1 {
		t.Fatalf("expected one GitHub request, got %q: %v", calls, err)
	}
}

// Ошибка запроса или повреждённый ответ не возвращают ложные статусы.
func TestReleasesFailure(t *testing.T) {
	for _, response := range []string{"error", "invalid JSON"} {
		t.Run(response, func(t *testing.T) {
			g, _ := fixtureGitHub(t, releaseFixture{Releases: response})
			if releases, err := g.Releases(context.Background()); err == nil || releases != nil {
				t.Fatalf("releases=%+v, err=%v", releases, err)
			}
		})
	}
}

func TestReleasesFollowBoundedPages(t *testing.T) {
	full := "[" + strings.Repeat(`{"tag_name":"v0.48.1"},`, releasepages.Size-1) + `{"tag_name":"v0.48.0"}]`
	workshop := `[{"tag_name":"workshop-v0.3.1","draft":false,"assets":[{"name":"wynd-workshop.exe.sig"}]}]`
	for _, mode := range []string{"second page", "failure", "limit"} {
		t.Run(mode, func(t *testing.T) {
			pages := []string{full, workshop}
			if mode == "failure" {
				pages[1] = "error"
			}
			if mode == "limit" {
				pages = make([]string, releasepages.Max)
				for i := range pages {
					pages[i] = full
				}
			}
			g, dir := fixtureGitHub(t, releaseFixture{ReleasePages: pages})
			releases, err := g.Releases(t.Context())
			if mode == "second page" {
				if err != nil || len(releases) != 1 || !releases[0].Signed {
					t.Fatalf("releases=%+v err=%v", releases, err)
				}
			} else if err == nil || releases != nil {
				t.Fatalf("partial snapshot: %+v %v", releases, err)
			}
			calls, err := os.ReadFile(filepath.Join(dir, "calls"))
			if err != nil {
				t.Fatal(err)
			}
			want := 2
			if mode == "limit" {
				want = releasepages.Max
			}
			if strings.Count(string(calls), "\n") != want {
				t.Fatalf("calls=%q", calls)
			}
		})
	}
}

func fixtureGitHub(t *testing.T, f releaseFixture) (GitHub, string) {
	t.Helper()
	dir := t.TempDir()
	raw, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fixture.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WYND_SIGNING_TEST_GH", dir)
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return GitHub{Repo: "test/wynd", Bin: bin}, dir
}

// Подпись опубликованного выпуска добавляет только .sig: даже при наличии
// новых заметок не редактирует описание и не публикует выпуск заново.
func TestSignReleasePublication(t *testing.T) {
	key := filepath.Join(t.TempDir(), "signing.key")
	const pass = "test-release-password"
	pub, err := Keygen(key, key+".pub", pass)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name                   string
		draft, notes, keyInTag bool
		wrongKey               bool
		failure                string
	}{
		{"published with notes", false, true, true, false, ""},
		{"published without notes", false, false, true, false, ""},
		{"published before signing was enabled", false, true, false, false, ""},
		{"draft with notes", true, true, true, false, ""},
		{"draft without notes", true, false, true, false, ""},
		{"key does not match release", false, true, true, true, "signature"},
		{"upload fails", true, true, true, false, "upload"},
		{"publication fails after upload", true, true, true, false, "publish"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			remotePub := pub
			if tt.wrongKey {
				remotePub = base64.StdEncoding.EncodeToString(make([]byte, 32))
			}
			g, dir := fixtureGitHub(t, releaseFixture{Draft: tt.draft, Notes: tt.notes, PubKey: remotePub, KeyPresent: tt.keyInTag, UploadFailure: tt.failure == "upload", PublishFailure: tt.failure == "publish"})
			var checks []Progress
			c, err := g.FetchWithProgress(context.Background(), releaseTestTag, func(p Progress) { checks = append(checks, p) })
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			if c.Problem != "" || c.Draft != tt.draft || c.KeyInRelease != tt.keyInTag {
				t.Fatalf("unexpected check: %+v", c)
			}
			assertProgress(t, checks, "check", "release,download,checksum,attestation,key,notes", "")
			if len(c.Steps) != 6 || c.Steps[5].Status != "done" {
				t.Fatalf("final check snapshot: %+v", c.Steps)
			}
			var signs []Progress
			published, err := g.SignAndPublishWithProgress(context.Background(), c, key, pass, func(p Progress) { signs = append(signs, p) })
			started := "release,signature"
			if tt.failure != "signature" {
				started += ",upload"
				if tt.draft && tt.notes && tt.failure != "upload" {
					started += ",publish"
				}
			}
			assertProgress(t, signs, "sign", started, tt.failure)
			encoded, _ := json.Marshal(signs)
			if strings.Contains(string(encoded), pass) {
				t.Fatal("password leaked into progress")
			}
			if tt.failure != "" {
				if err == nil {
					t.Fatal("operation must fail")
				}
				_, uploadErr := os.Stat(filepath.Join(dir, "uploaded.sig"))
				if tt.failure == "publish" && uploadErr != nil || tt.failure != "publish" && !os.IsNotExist(uploadErr) {
					t.Fatalf("unexpected upload after %s failure: %v", tt.failure, uploadErr)
				}
				return
			}
			if err != nil || published != (!tt.draft || tt.notes) {
				t.Fatalf("published=%v, err=%v", published, err)
			}
			sig, err := os.ReadFile(filepath.Join(dir, "uploaded.sig"))
			if err != nil || installer.VerifySignature(pub, releaseTestEXE, sig) != nil {
				t.Fatalf("uploaded signature does not verify: %v", err)
			}
			calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
			if edited := strings.Contains(string(calls), "release edit "); edited != (tt.draft && tt.notes) {
				t.Fatalf("unexpected release edit: %s", calls)
			}
		})
	}
}

// Wynd Настольный подписывается и публикуется так же, как Мастерская:
// сумма, удостоверение, вшитый ключ самообновления и «Что нового» из
// whatsnew.md в теге. Набор этапов программа объявляет сама.
func TestDesktopReleaseCheckAndSign(t *testing.T) {
	const tag = "desktop-v0.1.1"
	key := filepath.Join(t.TempDir(), "signing.key")
	const pass = "test-release-password"
	pub, err := Keygen(key, key+".pub", pass)
	if err != nil {
		t.Fatal(err)
	}
	g, dir := fixtureGitHub(t, releaseFixture{Tag: tag, Draft: true, Notes: true, PubKey: pub, KeyPresent: true})
	var checks []Progress
	c, err := g.FetchWithProgress(context.Background(), tag, func(p Progress) { checks = append(checks, p) })
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.Problem != "" || c.Product != ProductDesktop || !c.SelfUpdate || !c.KeyInRelease || c.Notes == "" {
		t.Fatalf("unexpected check: %+v", c)
	}
	if c.Name != "Wynd Настольный" || c.Exe != "wynd-desktop.exe" {
		t.Fatalf("product description missing: %+v", c)
	}
	var ids []string
	for _, step := range c.Steps {
		ids = append(ids, step.ID)
		if step.Status != "done" {
			t.Fatalf("unfinished step: %+v", step)
		}
	}
	if strings.Join(ids, ",") != "release,download,checksum,attestation,key,notes" {
		t.Fatalf("desktop check steps: %v", ids)
	}
	calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
	if !strings.Contains(string(calls), "cmd/wynd-desktop/whatsnew.md") {
		t.Fatalf("desktop release did not read its whatsnew.md: %s", calls)
	}
	var signs []Progress
	published, err := g.SignAndPublishWithProgress(context.Background(), c, key, pass, func(p Progress) { signs = append(signs, p) })
	if err != nil || !published {
		t.Fatalf("published=%v, err=%v", published, err)
	}
	for _, events := range [][]Progress{checks, signs} {
		for _, event := range events {
			if event.Tag != tag {
				t.Fatalf("wrong tag in progress: %+v", event)
			}
		}
	}
	calls, _ = os.ReadFile(filepath.Join(dir, "calls"))
	if !strings.Contains(string(calls), "release edit ") {
		t.Fatalf("desktop draft was not published: %s", calls)
	}
	sig, err := os.ReadFile(filepath.Join(dir, "uploaded.sig"))
	if err != nil || installer.VerifySignature(pub, releaseTestEXE, sig) != nil {
		t.Fatalf("uploaded signature does not verify: %v", err)
	}
}

// Без раздела версии в whatsnew.md окно, как и Мастерская, подписывается,
// но черновик остаётся черновиком.
func TestDesktopDraftWithoutNotesStaysDraft(t *testing.T) {
	const tag = "desktop-v0.1.1"
	key := filepath.Join(t.TempDir(), "signing.key")
	const pass = "test-release-password"
	if _, err := Keygen(key, key+".pub", pass); err != nil {
		t.Fatal(err)
	}
	g, dir := fixtureGitHub(t, releaseFixture{Tag: tag, Draft: true})
	var checks []Progress
	c, err := g.FetchWithProgress(context.Background(), tag, func(p Progress) { checks = append(checks, p) })
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.Problem != "" || c.Notes != "" {
		t.Fatalf("unexpected check: %+v", c)
	}
	published, err := g.SignAndPublish(context.Background(), c, key, pass)
	if err != nil || published {
		t.Fatalf("published=%v, err=%v", published, err)
	}
	calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
	if strings.Contains(string(calls), "release edit ") {
		t.Fatalf("desktop draft without notes was published: %s", calls)
	}
	if _, err := os.Stat(filepath.Join(dir, "uploaded.sig")); err != nil {
		t.Fatalf("signature was not uploaded: %v", err)
	}
}


// Ошибка чтения ключа не считается старой сборкой без ключа; сумма и
// удостоверение по-прежнему обязательны, и отказ не выкладывает подпись.
func TestReleaseChecksBlockSigning(t *testing.T) {
	for _, tt := range []struct {
		name   string
		f      releaseFixture
		failed string
	}{
		{"directory request failed", releaseFixture{ListFailure: true}, "key"},
		{"key request failed", releaseFixture{KeyPresent: true, KeyFailure: true}, "key"},
		{"empty key", releaseFixture{KeyPresent: true}, "key"},
		{"checksum mismatch", releaseFixture{BadSum: true}, "checksum"},
		{"attestation for another tag", releaseFixture{BadAttest: true}, "attestation"},
		{"attestation network failure", releaseFixture{AttestFailure: true}, "attestation"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g, dir := fixtureGitHub(t, tt.f)
			var events []Progress
			c, err := g.FetchWithProgress(context.Background(), releaseTestTag, func(p Progress) { events = append(events, p) })
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			if c.Problem == "" {
				t.Fatal("unsafe release passed checks")
			}
			started := "release,download,checksum"
			if tt.failed != "checksum" {
				started += ",attestation"
				if tt.failed != "attestation" {
					started += ",key"
				}
			}
			assertProgress(t, events, "check", started, tt.failed)
			if _, err := g.SignAndPublish(context.Background(), c, "unused", "unused"); err == nil {
				t.Fatal("unsafe release was signed")
			}
			if _, err := os.Stat(filepath.Join(dir, "uploaded.sig")); !os.IsNotExist(err) {
				t.Fatalf("signature must not be uploaded: %v", err)
			}
		})
	}
}

// Сбой скачивания оставляет поздние этапы ожидающими и не теряет failed,
// даже когда Check не возвращён; ранние снимки событий не меняются задним числом.
func TestDownloadProgressFailure(t *testing.T) {
	g, _ := fixtureGitHub(t, releaseFixture{DownloadFailure: true})
	var events []Progress
	c, err := g.FetchWithProgress(context.Background(), releaseTestTag, func(p Progress) { events = append(events, p) })
	if err == nil || c != nil {
		t.Fatalf("check=%+v, err=%v", c, err)
	}
	assertProgress(t, events, "check", "release,download", "download")
}

func assertProgress(t *testing.T, events []Progress, phase, started, failed string) {
	t.Helper()
	var running []string
	for _, event := range events {
		if event.Tag != releaseTestTag || event.Phase != phase {
			t.Fatalf("wrong operation: %+v", event)
		}
		for _, step := range event.Steps {
			if step.Status == "running" {
				running = append(running, step.ID)
			}
		}
	}
	if strings.Join(running, ",") != started {
		t.Fatalf("stages=%v, want %s", running, started)
	}
	if len(events) == 0 {
		t.Fatal("no progress events")
	}
	failedSeen := false
	for _, step := range events[len(events)-1].Steps {
		if step.Status == "failed" {
			if step.ID != failed {
				t.Fatalf("wrong failed stage: %+v", step)
			}
			failedSeen = true
		}
		if step.Status == "running" || failed == "" && step.Status != "done" {
			t.Fatalf("unfinished stage: %+v", step)
		}
	}
	if failedSeen != (failed != "") {
		t.Fatalf("failed stage missing: %+v", events[len(events)-1])
	}
}
