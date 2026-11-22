//go:build windows

package desktop

import (
	"os"
	"strings"
	"testing"
)

// Реестр: команда схемы wynd: переписывается на путь этого exe и читается
// обратно (по образцу теста автозапуска).
func TestInviteSchemeRegistry(t *testing.T) {
	if err := EnsureInviteScheme(); err != nil {
		t.Fatalf("register: %v", err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	stored, err := InviteSchemeCommand()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stored, exe) || !strings.Contains(stored, "%1") {
		t.Fatalf("команда без exe или адреса: %q", stored)
	}
	// Повторная регистрация переписывает, а не дублирует.
	if err := EnsureInviteScheme(); err != nil {
		t.Fatalf("re-register: %v", err)
	}
	if stored, err = InviteSchemeCommand(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stored, exe) {
		t.Fatalf("повторная регистрация потеряла команду: %q", stored)
	}
}
