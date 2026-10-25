package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func answer(lines ...string) {
	stdin = bufio.NewReader(strings.NewReader(strings.Join(lines, "\n") + "\n"))
}

// Ключ, подпись exe и проверка той же функцией, что у Мастерской; неверный
// пароль, изменённый exe и второй keygen поверх ключа — отказ.
func TestKeygenSignVerify(t *testing.T) {
	dir := t.TempDir()
	key, pub, exe := filepath.Join(dir, "k", "signing.key"), filepath.Join(dir, "signing-key.pub"), filepath.Join(dir, "wynd-workshop.exe")
	if err := os.WriteFile(exe, []byte("MZ пробный exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	const pass = "дача-самовар-17"
	answer(pass, pass)
	if err := run([]string{"keygen", key, pub}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(key)
	if strings.Contains(string(raw), "wynd-workshop-signing-key") {
		t.Fatal("ключ лежит в файле открытым")
	}
	answer(pass, pass)
	if err := run([]string{"keygen", key, pub}); err == nil {
		t.Fatal("второй keygen перезаписал ключ")
	}
	answer("не тот пароль")
	if err := run([]string{"sign", key, exe}); err == nil {
		t.Fatal("подписал с неверным паролем")
	}
	answer(pass)
	if err := run([]string{"sign", key, exe}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"verify", pub, exe}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("MZ подменённый exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"verify", pub, exe}); err == nil {
		t.Fatal("подпись сошлась с другим exe")
	}
}

func TestKeygenRefusesShortOrMismatchedPassword(t *testing.T) {
	dir := t.TempDir()
	key, pub := filepath.Join(dir, "signing.key"), filepath.Join(dir, "signing-key.pub")
	answer("короткий", "короткий")
	if err := run([]string{"keygen", key, pub}); err == nil {
		t.Fatal("принял пароль короче 12 знаков")
	}
	answer("длинный-пароль-1", "длинный-пароль-2")
	if err := run([]string{"keygen", key, pub}); err == nil {
		t.Fatal("принял несовпавшие пароли")
	}
	if _, err := os.Stat(key); err == nil {
		t.Fatal("после отказа остался файл ключа")
	}
}
