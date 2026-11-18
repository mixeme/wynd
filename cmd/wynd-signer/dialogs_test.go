//go:build dev || desktop || bindings

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Отмена диалога в Wails v3 приходит ошибкой внутренней библиотеки
// (internal/go-common-file-dialog: «cancelled by user»). Если её не отличить,
// окно показывает «Не удалось открыть окно выбора файла: cancelled by user»
// после обычного «Отмена» (найдено живой проверкой 18.11).
func TestDialogCancelled(t *testing.T) {
	if !dialogCancelled(errors.New("cancelled by user")) {
		t.Fatal("v3 cancellation error must be treated as no choice")
	}
	for _, err := range []error{nil, errors.New("access denied"), errors.New("cancelled")} {
		if dialogCancelled(err) {
			t.Fatalf("%v must stay an error", err)
		}
	}
}

func TestKeyDialogDirectory(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "keys")
	if err := os.Mkdir(existing, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(file, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ name, path, want string }{
		{"existing key directory", filepath.Join(existing, "signing.key"), existing},
		{"first launch without .wynd", filepath.Join(root, ".wynd", "workshop-signing.key"), root},
		{"removed nested directory", filepath.Join(root, "removed", "keys", "signing.key"), root},
		{"parent is a file", filepath.Join(file, "signing.key"), root},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := keyDialogDirectory(tt.path); got != tt.want {
				t.Fatalf("directory = %q, want %q", got, tt.want)
			}
		})
	}
	if _, err := os.Stat(filepath.Join(root, ".wynd")); !os.IsNotExist(err) {
		t.Fatalf("choosing a directory must not create .wynd: %v", err)
	}
}
