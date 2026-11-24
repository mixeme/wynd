//go:build linux

package desktop

import (
	"encoding/json"
	"slices"

	"github.com/zalando/go-keyring"
)

// keyringService — имя приложения в Secret Service. Пользователь — origin
// сервера, значение — JSON сессии; служебный ключ __index__ хранит список
// origin’ов: у Secret Service нет перечисления записей.
const (
	keyringService = "Wynd.Desktop"
	keyringIndex   = "__index__"
)

// KeyringStore — SecretStore поверх Secret Service (GNOME Keyring, KWallet).
type KeyringStore struct{}

// NewSecretStore — платформенная точка сборки для Linux.
func NewSecretStore() SecretStore { return KeyringStore{} }

func (KeyringStore) Get(origin string) ([]byte, bool) {
	value, err := keyring.Get(keyringService, origin)
	if err != nil {
		return nil, false
	}
	return []byte(value), true
}

func (KeyringStore) Put(origin string, blob []byte) error {
	if err := keyring.Set(keyringService, origin, string(blob)); err != nil {
		return err
	}
	return updateKeyringIndex(origin, true)
}

func (KeyringStore) Delete(origin string) bool {
	if err := keyring.Delete(keyringService, origin); err != nil {
		return false
	}
	_ = updateKeyringIndex(origin, false)
	return true
}

func (KeyringStore) List() []string {
	value, err := keyring.Get(keyringService, keyringIndex)
	if err != nil {
		return nil
	}
	var origins []string
	if json.Unmarshal([]byte(value), &origins) != nil {
		return nil
	}
	return origins
}

func updateKeyringIndex(origin string, add bool) error {
	origins := KeyringStore{}.List()
	pos := slices.Index(origins, origin)
	if add && pos < 0 {
		origins = append(origins, origin)
	}
	if !add && pos >= 0 {
		origins = append(origins[:pos], origins[pos+1:]...)
	}
	encoded, err := json.Marshal(origins)
	if err != nil {
		return err
	}
	if len(origins) == 0 {
		return keyring.Delete(keyringService, keyringIndex)
	}
	return keyring.Set(keyringService, keyringIndex, string(encoded))
}
