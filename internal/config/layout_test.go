package config

import (
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
	for _, name := range []string{"wynd.db", "config.json", "blobs", "keys"} {
		if _, err := os.Stat(filepath.Join(cfg.DataDir, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
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
