//go:build windows

package desktop

import (
	"bytes"
	"errors"

	"github.com/danieljoos/wincred"
)

// CredentialPrefix — учётные записи окна в Credential Manager:
// <префикс><origin> — сессия сервера, <префикс>admin — панель.
const CredentialPrefix = "Wynd.Desktop/"

// WincredStore — SecretStore поверх Credential Manager: по учётной записи на
// origin, блоб — JSON сессии целиком. Пишет только процесс окна.
type WincredStore struct{}

func NewWincredStore() *WincredStore { return &WincredStore{} }

// NewSecretStore — платформенная точка сборки (вне Windows — NullSecretStore).
func NewSecretStore() SecretStore { return WincredStore{} }

func (WincredStore) Get(origin string) ([]byte, bool) {
	cred, err := wincred.GetGenericCredential(CredentialPrefix + origin)
	if err != nil {
		return nil, false
	}
	return append([]byte(nil), cred.CredentialBlob...), true
}

func (WincredStore) Put(origin string, blob []byte) error {
	if len(blob) > secretBlobLimit {
		return errors.New("secrets: сессия больше учётной записи Windows не влезает")
	}
	c := wincred.NewGenericCredential(CredentialPrefix + origin)
	c.UserName = "Wynd"
	c.CredentialBlob = blob
	return c.Write()
}

func (WincredStore) Delete(origin string) bool {
	cred, err := wincred.GetGenericCredential(CredentialPrefix + origin)
	if err != nil {
		return false
	}
	return cred.Delete() == nil
}

func (WincredStore) List() []string {
	creds, err := wincred.FilteredList(CredentialPrefix + "*")
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(creds))
	for _, cred := range creds {
		if origin, ok := bytes.CutPrefix([]byte(cred.TargetName), []byte(CredentialPrefix)); ok {
			out = append(out, string(origin))
		}
	}
	return out
}
