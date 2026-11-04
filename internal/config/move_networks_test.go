package config

import "testing"

func TestMoveAllowedNetworks(t *testing.T) {
	t.Setenv("WYND_DATA_DIR", t.TempDir())
	t.Setenv("WYND_PUBLIC_URL", DefaultPublicURL)
	t.Setenv("WYND_MOVE_ALLOWED_CIDRS", "10.1.2.3/32, fd00::/64")
	cfg, err := read()
	if err != nil || len(cfg.MoveAllowedNetworks) != 2 {
		t.Fatalf("config: %v %v", cfg, err)
	}
	t.Setenv("WYND_MOVE_ALLOWED_CIDRS", "10.1.2.3/garbage")
	if _, err := read(); err == nil {
		t.Fatal("invalid operator network accepted")
	}
	t.Setenv("WYND_MOVE_ALLOWED_CIDRS", "")
	cfg, err = read()
	if err != nil || len(cfg.MoveAllowedNetworks) != 0 {
		t.Fatalf("default: %v %v", cfg, err)
	}
}

func TestMoveMaxBytes(t *testing.T) {
	t.Setenv("WYND_DATA_DIR", t.TempDir())
	t.Setenv("WYND_PUBLIC_URL", DefaultPublicURL)
	for _, bad := range []string{"0", "-1", "9223372036854775808", "junk"} {
		t.Setenv("WYND_MOVE_MAX_BYTES", bad)
		if _, err := read(); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	t.Setenv("WYND_MOVE_MAX_BYTES", "1024")
	cfg, err := read()
	if err != nil || cfg.MoveMaxArchiveBytes != 1024 {
		t.Fatalf("config: %+v %v", cfg, err)
	}
}
