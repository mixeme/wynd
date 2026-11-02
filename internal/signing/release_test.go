package signing

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/installer"
)

const releaseTestTag = "workshop-v0.3.0"

var releaseTestEXE = []byte("MZ test Workshop release")

type releaseFixture struct {
	Draft       bool
	Notes       bool
	PubKey      string
	KeyPresent  bool
	KeyFailure  bool
	ListFailure bool
	BadSum      bool
	BadAttest   bool
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
	case args[0] == "release" && args[1] == "view":
		fmt.Printf(`{"isDraft":%t}`, f.Draft)
	case args[0] == "release" && args[1] == "download":
		var dest string
		for i, arg := range args {
			if arg == "-D" {
				dest = args[i+1]
			}
		}
		if err := os.WriteFile(filepath.Join(dest, exeName), releaseTestEXE, 0o600); err != nil {
			return err
		}
		sum := sha256.Sum256(releaseTestEXE)
		want := hex.EncodeToString(sum[:])
		if f.BadSum {
			want = "wrong sum"
		}
		return os.WriteFile(filepath.Join(dest, exeName+".sha256"), []byte(want), 0o600)
	case args[0] == "attestation":
		ref := "refs/tags/" + releaseTestTag
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
			fmt.Print(base64.StdEncoding.EncodeToString([]byte("## 0.3.0\n\n- Test release notes\n")))
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
		sig, err := os.ReadFile(args[3])
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, "uploaded.sig"), sig, 0o600)
	case args[0] == "release" && args[1] == "edit":
	default:
		return fmt.Errorf("unexpected gh arguments: %v", args)
	}
	return nil
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
	}{
		{"published with notes", false, true, true, false},
		{"published without notes", false, false, true, false},
		{"published before signing was enabled", false, true, false, false},
		{"draft with notes", true, true, true, false},
		{"draft without notes", true, false, true, false},
		{"key does not match release", false, true, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			remotePub := pub
			if tt.wrongKey {
				remotePub = base64.StdEncoding.EncodeToString(make([]byte, 32))
			}
			g, dir := fixtureGitHub(t, releaseFixture{Draft: tt.draft, Notes: tt.notes, PubKey: remotePub, KeyPresent: tt.keyInTag})
			c, err := g.Fetch(context.Background(), releaseTestTag)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			if c.Problem != "" || c.Draft != tt.draft || c.KeyInRelease != tt.keyInTag {
				t.Fatalf("unexpected check: %+v", c)
			}
			published, err := g.SignAndPublish(context.Background(), c, key, pass)
			if tt.wrongKey {
				if err == nil {
					t.Fatal("signature with a key that differs from the release key must fail")
				}
				if _, err := os.Stat(filepath.Join(dir, "uploaded.sig")); !os.IsNotExist(err) {
					t.Fatalf("wrong-key signature must not be uploaded: %v", err)
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

// Ошибка чтения ключа не считается старой сборкой без ключа; сумма и
// удостоверение по-прежнему обязательны, и отказ не выкладывает подпись.
func TestReleaseChecksBlockSigning(t *testing.T) {
	for _, tt := range []struct {
		name string
		f    releaseFixture
	}{
		{"directory request failed", releaseFixture{ListFailure: true}},
		{"key request failed", releaseFixture{KeyPresent: true, KeyFailure: true}},
		{"empty key", releaseFixture{KeyPresent: true}},
		{"checksum mismatch", releaseFixture{BadSum: true}},
		{"attestation for another tag", releaseFixture{BadAttest: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g, dir := fixtureGitHub(t, tt.f)
			c, err := g.Fetch(context.Background(), releaseTestTag)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			if c.Problem == "" {
				t.Fatal("unsafe release passed checks")
			}
			if _, err := g.SignAndPublish(context.Background(), c, "unused", "unused"); err == nil {
				t.Fatal("unsafe release was signed")
			}
			if _, err := os.Stat(filepath.Join(dir, "uploaded.sig")); !os.IsNotExist(err) {
				t.Fatalf("signature must not be uploaded: %v", err)
			}
		})
	}
}
