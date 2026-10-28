// Package signing — ключ подписи выпусков Мастерской (план 45) у владельца:
// создать ключ, открыть его паролем, подписать exe. Самообновление Мастерской
// ставит только exe с подписью этим ключом (installer.VerifySignature):
// Ed25519 над всем файлом, подпись — base64 в wynd-workshop.exe.sig рядом с
// exe. Пользуются им cmd/workshop-sign (командная строка) и cmd/wynd-signer
// (окно «Wynd Подпись», кадры 12а–12д).
//
// Закрытый ключ лежит в файле только зашифрованным паролем владельца — тем же
// шифрованием, что бэкапы и архивы (scrypt + age, internal/archive).
package signing

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
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

// MinPassword — пароль ключа не короче стольких знаков.
const MinPassword = 12

var (
	ErrShortPassword = fmt.Errorf("пароль короче %d знаков", MinPassword)
	ErrKeyExists     = errors.New("файл ключа уже есть — не перезаписываю: прежний ключ пропал бы, а с ним возможность подписать обновление для уже розданных Мастерских")
	ErrWrongPassword = errors.New("пароль не подошёл или файл ключа повреждён")
	ErrNotKey        = errors.New("это не файл ключа подписи Мастерской")
)

// Keygen создаёт ключ: закрытый — в keyPath зашифрованным паролем, открытый —
// строкой base64 в pubPath. Возвращает открытый ключ.
func Keygen(keyPath, pubPath, pass string) (string, error) {
	if _, err := os.Stat(keyPath); err == nil {
		return "", ErrKeyExists
	}
	if len([]rune(pass)) < MinPassword {
		return "", ErrShortPassword
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	enc, err := archive.DeriveKey(pass, salt, keyLogN)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	w, err := archive.Encrypt(&buf, archive.KeyParams{PublicKey: enc.PublicKey().Bytes(), Salt: salt, LogN: keyLogN})
	if err != nil {
		return "", err
	}
	if _, err := io.WriteString(w, keyMagic+"\n"+base64.StdEncoding.EncodeToString(priv.Seed())+"\n"); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return "", err
	}
	// O_EXCL: файл ключа мог появиться, пока вводили пароль.
	f, err := os.OpenFile(keyPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	line := base64.StdEncoding.EncodeToString(pub)
	if err := os.WriteFile(pubPath, []byte(line+"\n"), 0o644); err != nil {
		return "", err
	}
	return line, nil
}

// Unlock открывает ключ паролем.
func Unlock(keyPath, pass string) (ed25519.PrivateKey, error) {
	raw, err := os.Open(keyPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = raw.Close() }()
	r, err := archive.Decrypt(raw, pass)
	if err != nil {
		return nil, ErrWrongPassword
	}
	plain, err := io.ReadAll(r)
	if err != nil {
		// age проверяет ключ на первом куске: неверный пароль — здесь.
		return nil, ErrWrongPassword
	}
	lines := strings.Split(strings.TrimSpace(string(plain)), "\n")
	if len(lines) != 2 || lines[0] != keyMagic {
		return nil, ErrNotKey
	}
	seed, err := base64.StdEncoding.DecodeString(lines[1])
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("файл ключа повреждён")
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// Public — открытый ключ строкой base64, как в signing-key.pub.
func Public(priv ed25519.PrivateKey) string {
	return base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey))
}

// Sign подписывает exe и проверяет подпись той же функцией, что у Мастерской
// при самообновлении, — открытым ключом pub (пусто — своим же).
func Sign(priv ed25519.PrivateKey, exe []byte, pub string) (string, error) {
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, exe))
	if pub == "" {
		pub = Public(priv)
	}
	if err := installer.VerifySignature(strings.TrimSpace(pub), exe, []byte(sig)); err != nil {
		return "", fmt.Errorf("подпись не прошла проверку открытым ключом: %w", err)
	}
	return sig, nil
}
