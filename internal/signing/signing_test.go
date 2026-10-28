package signing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/installer"
)

// Ключ, открытие паролем, подпись и проверка той же функцией, что у
// Мастерской; неверный пароль и второй ключ поверх первого — отказ.
func TestKeygenUnlockSign(t *testing.T) {
	dir := t.TempDir()
	key, pub := filepath.Join(dir, "k", "signing.key"), filepath.Join(dir, "signing-key.pub")
	if _, err := Keygen(key, pub, "короткий"); err != ErrShortPassword {
		t.Fatalf("короткий пароль: %v", err)
	}
	line, err := Keygen(key, pub, "дача-самовар-17")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Keygen(key, pub, "дача-самовар-17"); err != ErrKeyExists {
		t.Fatalf("второй ключ: %v", err)
	}
	if _, err := Unlock(key, "не тот пароль"); err != ErrWrongPassword {
		t.Fatalf("неверный пароль: %v", err)
	}
	priv, err := Unlock(key, "дача-самовар-17")
	if err != nil {
		t.Fatal(err)
	}
	exe := []byte("MZ пробный exe")
	sig, err := Sign(priv, exe, line)
	if err != nil {
		t.Fatal(err)
	}
	if installer.VerifySignature(line, exe, []byte(sig)) != nil {
		t.Fatal("подпись не сходится")
	}
	if _, err := Sign(priv, exe, "AAAA"); err == nil {
		t.Fatal("подпись прошла проверку чужим открытым ключом")
	}
	saved, _ := os.ReadFile(pub)
	if strings.TrimSpace(string(saved)) != line {
		t.Fatalf("открытый ключ в файле: %q", saved)
	}
}

func TestWhatsNew(t *testing.T) {
	md := "# Что нового\n\nвступление\n\n## 0.3.0\n\n- первое\n- второе\n\n## 0.2.0\n\n- старое\n"
	if got := WhatsNew(md, "0.3.0"); got != "- первое\n- второе" {
		t.Fatalf("0.3.0: %q", got)
	}
	if got := WhatsNew(md, "0.2.0"); got != "- старое" {
		t.Fatalf("0.2.0: %q", got)
	}
	if WhatsNew(md, "0.4.0") != "" {
		t.Fatal("нет раздела — пусто")
	}
	raw, err := os.ReadFile("../../cmd/wynd-installer/whatsnew.md")
	if err != nil {
		t.Fatal(err)
	}
	if WhatsNew(string(raw), installer.WorkshopVersion) == "" {
		t.Fatalf("в whatsnew.md нет раздела текущей версии Мастерской %s", installer.WorkshopVersion)
	}
}
