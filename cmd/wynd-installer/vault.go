//go:build dev || desktop || bindings

package main

import "github.com/zalando/go-keyring"

// keyringService — имя, под которым пароли серверов видны в связке ключей
// системы (Диспетчер учётных данных Windows, Secret Service в Linux).
const keyringService = "Wynd Installer"

// keyringVault хранит пароль от сервера в системной связке ключей. В файлы
// приложения пароль не попадает.
type keyringVault struct{}

func vaultKey(host, user string) string { return user + "@" + host }

func (keyringVault) Get(host, user string) (string, bool) {
	password, err := keyring.Get(keyringService, vaultKey(host, user))
	return password, err == nil && password != ""
}

func (keyringVault) Set(host, user, password string) error {
	return keyring.Set(keyringService, vaultKey(host, user), password)
}

func (keyringVault) Delete(host, user string) error {
	err := keyring.Delete(keyringService, vaultKey(host, user))
	if err == keyring.ErrNotFound {
		return nil
	}
	return err
}
