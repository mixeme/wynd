package selfupdate

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
)

// signedRelease отдаёт файл и его подпись; возвращает адреса и открытый ключ.
func signedRelease(t *testing.T, payload []byte) (fileURL, sigURL, key string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, payload))
	mux := http.NewServeMux()
	mux.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	})
	mux.HandleFunc("/file.sig", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sig + "\n"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL + "/file", srv.URL + "/file.sig", base64.StdEncoding.EncodeToString(pub)
}

// Подпись проверяется до подмены: с чужим ключом файл не принимается.
func TestFetchVerifiedRejectsBadSignature(t *testing.T) {
	fileURL, sigURL, _ := signedRelease(t, []byte("new"))
	wrongPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	wrongKey := base64.StdEncoding.EncodeToString(wrongPub)
	if _, err := FetchVerified(context.Background(), http.DefaultClient, wrongKey, fileURL, sigURL, nil); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("ждали ErrBadSignature, получили %v", err)
	}
}

// ApplyTo подменяет заданный файл (AppImage), оставляя прежний рядом (.old).
func TestApplyToReplacesTarget(t *testing.T) {
	payload := []byte("new-appimage")
	fileURL, sigURL, key := signedRelease(t, payload)
	target := filepath.Join(t.TempDir(), "Wynd.AppImage")
	if err := os.WriteFile(target, []byte("old-appimage"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ApplyTo(context.Background(), http.DefaultClient, key, fileURL, sigURL, target, nil)
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
	old, err := os.ReadFile(target + ".old")
	if err != nil {
		t.Fatal(err)
	}
	if string(old) != "old-appimage" {
		t.Fatalf("прежний файл: %q", old)
	}
	CleanOldSelfAt(target)
	if _, err := os.Stat(target + ".old"); !os.IsNotExist(err) {
		t.Fatalf(".old остался: %v", err)
	}
}
