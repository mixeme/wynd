package auth_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

func TestLogCodesWritesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dev-auth-codes.log")
	lc := auth.LogCodes{File: path}
	if err := lc.SendCode(context.Background(), "user@example.com", "123456"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if !strings.Contains(body, "auth code for user@example.com: 123456") {
		t.Fatalf("log body: %q", body)
	}
}
