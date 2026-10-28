// Command workshop-sign — ключ подписи выпусков Мастерской (план 45) у
// владельца: создать ключ, подписать exe, проверить подпись (сами ключ и
// подпись — internal/signing; то же умеет окно cmd/wynd-signer). Самообновление
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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"gitea.mixdep.ru/mix/wynd/internal/installer"
	"gitea.mixdep.ru/mix/wynd/internal/signing"
)

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
		return fmt.Errorf("%s: %w", keyPath, signing.ErrKeyExists)
	}
	pass, err := readPassword("Пароль для ключа: ")
	if err != nil {
		return err
	}
	if len([]rune(pass)) < signing.MinPassword {
		return signing.ErrShortPassword
	}
	again, err := readPassword("Ещё раз: ")
	if err != nil {
		return err
	}
	if pass != again {
		return errors.New("пароли не совпали")
	}
	line, err := signing.Keygen(keyPath, pubPath, pass)
	if err != nil {
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
	if _, err := os.Stat(keyPath); err != nil {
		return err
	}
	pass, err := readPassword("Пароль ключа: ")
	if err != nil {
		return err
	}
	priv, err := signing.Unlock(keyPath, pass)
	if err != nil {
		return err
	}
	sig, err := signing.Sign(priv, exe, "")
	if err != nil {
		return err
	}
	if err := os.WriteFile(exePath+".sig", []byte(sig+"\n"), 0o644); err != nil {
		return err
	}
	sum := sha256.Sum256(exe)
	fmt.Println("Подписан:", exePath)
	fmt.Println("SHA-256:", hex.EncodeToString(sum[:]))
	fmt.Println("Подпись:", exePath+".sig")
	fmt.Println("Открытый ключ:", signing.Public(priv))
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
