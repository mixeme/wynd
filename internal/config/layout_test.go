package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesSingleDataRoot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WYND_DATA_DIR", dir)
	t.Setenv("WYND_LISTEN", "")
	t.Setenv("WYND_PUBLIC_URL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir != abs {
		t.Fatalf("data dir: %q want %q", cfg.DataDir, abs)
	}
	if cfg.Listen != DefaultListen {
		t.Fatalf("listen: %q", cfg.Listen)
	}
	for _, name := range []string{"wynd.db", "blobs", "keys"} {
		if _, err := os.Stat(filepath.Join(cfg.DataDir, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	// config.json при чтении не создаётся: он появляется только когда его
	// сохраняют — bootstrap или панель (API-5).
	if _, err := os.Stat(filepath.Join(cfg.DataDir, "config.json")); !os.IsNotExist(err) {
		t.Fatalf("config.json создан чтением: %v", err)
	}
	token, err := BootstrapToken(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("empty bootstrap token")
	}
	if _, err := os.Stat(filepath.Join(cfg.DataDir, "keys", "bootstrap")); err != nil {
		t.Fatal(err)
	}
}

// Инвариант: LoadExisting — для служебных команд — ничего не создаёт и
// отказывает без базы; с базой читает ту же конфигурацию, что Load.
func TestLoadExistingCreatesNothing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "typo")
	t.Setenv("WYND_DATA_DIR", missing)
	t.Setenv("WYND_LISTEN", "")
	t.Setenv("WYND_PUBLIC_URL", "")
	if _, err := LoadExisting(); !errors.Is(err, ErrNoDatabase) {
		t.Fatalf("missing dir: %v, want ErrNoDatabase", err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("каталог создан: %v", err)
	}

	dir := t.TempDir()
	t.Setenv("WYND_DATA_DIR", dir)
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	// Load создал пустой wynd.db — для команды это всё ещё «нет базы».
	if _, err := LoadExisting(); !errors.Is(err, ErrNoDatabase) {
		t.Fatalf("empty db: %v, want ErrNoDatabase", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "wynd.db"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadExisting()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != DefaultListen {
		t.Fatalf("listen: %q", cfg.Listen)
	}
}
