// Command workshop-sign — ключ подписи выпусков Мастерской (план 45) у
// владельца: создать ключ, подписать exe, проверить подпись. Самообновление
// Мастерской ставит только exe с подписью этим ключом
// (installer.VerifySignature): Ed25519 над всем файлом, подпись — base64 в
// wynd-workshop.exe.sig рядом с exe в выпуске.
//
// Закрытый ключ лежит в файле только зашифрованным паролем владельца — тем же
// шифрованием, что бэкапы и архивы (scrypt + age, internal/archive). Открытый
// ключ — в cmd/wynd-installer/signing-key.pub: его вшивает в сборку
// scripts\build-installer.bat. Обычно запускается через
// scripts\sign-workshop.bat.
//
//	workshop-sign keygen <файл ключа> <файл открытого ключа>
//	workshop-sign sign   <файл ключа> <exe>     → <exe>.sig
//	workshop-sign verify <файл открытого ключа> <exe> [подпись]
package main

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gitea.mixdep.ru/mix/wynd/internal/archive"
	"gitea.mixdep.ru/mix/wynd/internal/installer"
)

// keyMagic — первая строка расшифрованного файла ключа.
const keyMagic = "wynd-workshop-signing-key/v1"

// keyLogN — стоимость вывода ключа из пароля (scrypt N = 2^17, 128 МБ):
// подписывают редко, перебор пароля пусть будет дорогим.
const keyLogN = 17

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("нужна команда: keygen, sign или verify")
	}
	switch {
	case args[0] == "keygen" && len(args) == 3:
		return keygen(args[1], args[2])
	case args[0] == "sign" && len(args) == 3:
		return sign(args[1], args[2])
	case args[0] == "verify" && (len(args) == 3 || len(args) == 4):
		sig := args[2] + ".sig"
		if len(args) == 4 {
			sig = args[3]
		}
		return verify(args[1], args[2], sig)
	}
	return errors.New("неверные аргументы: keygen <ключ> <открытый ключ> | sign <ключ> <exe> | verify <открытый ключ> <exe> [подпись]")
}

func keygen(keyPath, pubPath string) error {
	if _, err := os.Stat(keyPath); err == nil {
		return fmt.Errorf("%s уже есть — не перезаписываю: прежний ключ пропал бы, а с ним возможность подписать обновление для уже розданных Мастерских", keyPath)
	}
	pass, err := readPassword("Пароль для ключа: ")
	if err != nil {
		return err
	}
	if len([]rune(pass)) < 12 {
		return errors.New("пароль короче 12 знаков")
	}
	again, err := readPassword("Ещё раз: ")
	if err != nil {
		return err
	}
	if pass != again {
		return errors.New("пароли не совпали")
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	enc, err := archive.DeriveKey(pass, salt, keyLogN)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	w, err := archive.Encrypt(&buf, archive.KeyParams{PublicKey: enc.PublicKey().Bytes(), Salt: salt, LogN: keyLogN})
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w, keyMagic+"\n"+base64.StdEncoding.EncodeToString(priv.Seed())+"\n"); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return err
	}
	// O_EXCL: файл ключа мог появиться, пока вводили пароль.
	f, err := os.OpenFile(keyPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	line := base64.StdEncoding.EncodeToString(pub)
	if err := os.WriteFile(pubPath, []byte(line+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Println("Ключ создан:", keyPath)
	fmt.Println("Открытый ключ:", line, "→", pubPath)
	return nil
}

func sign(keyPath, exePath string) error {
	exe, err := os.ReadFile(exePath)
	if err != nil {
		return err
	}
	raw, err := os.Open(keyPath)
	if err != nil {
		return err
	}
	defer func() { _ = raw.Close() }()
	pass, err := readPassword("Пароль ключа: ")
	if err != nil {
		return err
	}
	r, err := archive.Decrypt(raw, pass)
	if err != nil {
		return errors.New("пароль не подошёл или файл ключа повреждён")
	}
	plain, err := io.ReadAll(r)
	if err != nil {
		// age проверяет ключ на первом куске: неверный пароль — здесь.
		return errors.New("пароль не подошёл или файл ключа повреждён")
	}
	lines := strings.Split(strings.TrimSpace(string(plain)), "\n")
	if len(lines) != 2 || lines[0] != keyMagic {
		return errors.New("это не файл ключа подписи Мастерской")
	}
	seed, err := base64.StdEncoding.DecodeString(lines[1])
	if err != nil || len(seed) != ed25519.SeedSize {
		return errors.New("файл ключа повреждён")
	}
	priv := ed25519.NewKeyFromSeed(seed)
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, exe))
	pub := base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey))
	// Проверка той же функцией, что у Мастерской при самообновлении.
	if err := installer.VerifySignature(pub, exe, []byte(sig)); err != nil {
		return fmt.Errorf("подпись не прошла проверку: %w", err)
	}
	if err := os.WriteFile(exePath+".sig", []byte(sig+"\n"), 0o644); err != nil {
		return err
	}
	sum := sha256.Sum256(exe)
	fmt.Println("Подписан:", exePath)
	fmt.Println("SHA-256:", hex.EncodeToString(sum[:]))
	fmt.Println("Подпись:", exePath+".sig")
	fmt.Println("Открытый ключ:", pub)
	return nil
}

func verify(pubPath, exePath, sigPath string) error {
	pub, err := os.ReadFile(pubPath)
	if err != nil {
		return err
	}
	exe, err := os.ReadFile(exePath)
	if err != nil {
		return err
	}
	sig, err := os.ReadFile(sigPath)
	if err != nil {
		return err
	}
	if err := installer.VerifySignature(strings.TrimSpace(string(pub)), exe, sig); err != nil {
		return fmt.Errorf("подпись не сходится: %w", err)
	}
	fmt.Println("Подпись верна:", exePath)
	return nil
}

// stdin — один на все вопросы: свой буфер на каждый съел бы следующую строку.
var stdin = bufio.NewReader(os.Stdin)

// readPassword читает пароль с консоли без эха; не из консоли (тесты,
// перенаправленный ввод) — строку как есть.
func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	restore := noEcho()
	line, err := stdin.ReadString('\n')
	restore()
	fmt.Fprintln(os.Stderr)
	if err != nil && line == "" {
		return "", errors.New("пароль не введён")
	}
	return strings.TrimRight(line, "\r\n"), nil
}
