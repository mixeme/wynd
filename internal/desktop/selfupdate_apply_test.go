package desktop

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// signedAsset отдаёт файл выпуска и его подпись; возвращает адреса и ключ.
func signedAsset(t *testing.T, payload []byte) (fileURL, sigURL, key string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, payload))
	mux := http.NewServeMux()
	mux.HandleFunc("/asset", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	})
	mux.HandleFunc("/asset.sig", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sig + "\n"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL + "/asset", srv.URL + "/asset.sig", base64.StdEncoding.EncodeToString(pub)
}

// AppImage обновляется подменой самого файла: os.Executable() внутри образа
// указывает на временный бинарник и для этого не годится.
func TestApplyUpdateAppImage(t *testing.T) {
	payload := []byte("new-appimage")
	fileURL, sigURL, key := signedAsset(t, payload)
	oldKey := SigningKey
	t.Cleanup(func() { SigningKey = oldKey })
	SigningKey = key

	target := filepath.Join(t.TempDir(), "Wynd-0.5.8-x86_64.AppImage")
	if err := os.WriteFile(target, []byte("old-appimage"), 0o755); err != nil {
		t.Fatal(err)
	}
	up := Update{Available: true, kind: kindAppImage, target: target, fileURL: fileURL, sigURL: sigURL}
	got, err := ApplyUpdate(context.Background(), http.DefaultClient, up, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != target {
		t.Fatalf("путь установки: %q, ждали %q", got, target)
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, payload) {
		t.Fatalf("в файле %q, ждали %q", body, payload)
	}
}

func TestApplyUpdateWithoutRelease(t *testing.T) {
	if _, err := ApplyUpdate(context.Background(), http.DefaultClient, Update{}, nil); !errors.Is(err, ErrNoUpdate) {
		t.Fatalf("ждали ErrNoUpdate, получили %v", err)
	}
}

// Что подменять: AppImage, пакет .deb или exe; локальная Linux-сборка без
// установки обновлений не предлагает.
func TestDetectUpdateKind(t *testing.T) {
	owns := func(string) bool { return true }
	noOwns := func(string) bool { return false }
	cases := []struct {
		name, goos, appImage, exe string
		owns                      func(string) bool
		kind                      UpdateKind
		target                    string
	}{
		{"windows", "windows", "", `C:\Wynd\wynd-desktop.exe`, nil, kindExe, `C:\Wynd\wynd-desktop.exe`},
		{"appimage", "linux", "/tmp/Wynd.AppImage", "/tmp/.mount_w/usr/bin/wynd-desktop", owns, kindAppImage, "/tmp/Wynd.AppImage"},
		{"deb", "linux", "", "/usr/bin/wynd-desktop", owns, kindDeb, "/usr/bin/wynd-desktop"},
		{"local build", "linux", "", "/home/me/wynd-desktop", noOwns, kindNone, ""},
		{"not ours", "linux", "", "/usr/bin/wynd-desktop", noOwns, kindNone, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			kind, target := detectUpdateKind(c.goos, c.appImage, c.exe, c.owns)
			if kind != c.kind || target != c.target {
				t.Fatalf("получили %v %q, ждали %v %q", kind, target, c.kind, c.target)
			}
		})
	}
}

// Linux берёт из выпуска свой файл; выпуск без него не предлагается, даже
// если он новее.
func TestPickUpdateLinux(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	old := now.Add(-10 * 24 * time.Hour)
	assets := []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	}{
		{"wynd-desktop.exe", "https://x/exe"},
		{"wynd-desktop.exe.sig", "https://x/exe.sig"},
		{"Wynd-0.2.0-x86_64.AppImage", "https://x/app"},
		{"Wynd-0.2.0-x86_64.AppImage.sig", "https://x/app.sig"},
		{"wynd-desktop_0.2.0_amd64.deb", "https://x/deb"},
		{"wynd-desktop_0.2.0_amd64.deb.sig", "https://x/deb.sig"},
	}
	rels := []ghRelease{
		{TagName: "desktop-v0.2.0", PublishedAt: now.Add(-time.Hour), Assets: assets, Body: "- Новое\n"},
		{TagName: "desktop-v0.3.0", PublishedAt: now.Add(-time.Minute), Assets: assets[:2]},
	}
	app := pickUpdateFor(rels, "0.1.1", now, kindAppImage, "/tmp/Wynd.AppImage")
	if !app.Available || app.Version != "0.2.0" || app.fileURL != "https://x/app" || app.sigURL != "https://x/app.sig" {
		t.Fatalf("AppImage: %+v", app)
	}
	if app.kind != kindAppImage || app.target != "/tmp/Wynd.AppImage" {
		t.Fatalf("вид и путь AppImage: %+v", app)
	}
	deb := pickUpdateFor(rels, "0.1.1", now, kindDeb, "/usr/bin/wynd-desktop")
	if !deb.Available || deb.fileURL != "https://x/deb" || deb.sigURL != "https://x/deb.sig" {
		t.Fatalf(".deb: %+v", deb)
	}
	// Выпуск только с exe: Linux его пропускает — файла для этой сборки нет.
	onlyExe := []ghRelease{{TagName: "desktop-v0.2.0", PublishedAt: old, Assets: assets[:2]}}
	if up := pickUpdateFor(onlyExe, "0.1.1", now, kindAppImage, ""); up.Available {
		t.Fatalf("выпуск без AppImage предложен: %+v", up)
	}
	withoutSig := []ghRelease{{TagName: "desktop-v0.2.0", PublishedAt: old, Assets: assets[:5]}}
	if up := pickUpdateFor(withoutSig, "0.1.1", now, kindDeb, ""); up.Available {
		t.Fatalf("выпуск без подписи .deb предложен: %+v", up)
	}
}
