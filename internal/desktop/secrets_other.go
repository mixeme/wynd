//go:build !windows

package desktop

// Окно собирается только под Windows (desktop-reference, «Границы»);
// заглушка нужна серверным тестам на Linux. С системным хранилищем вне
// Windows сессии остаются в IndexedDB — как в браузере.

type NullSecretStore struct{}

func NewNullSecretStore() *NullSecretStore { return &NullSecretStore{} }

// NewSecretStore — платформенная точка сборки.
func NewSecretStore() SecretStore { return NullSecretStore{} }

func (NullSecretStore) Get(origin string) ([]byte, bool) { return nil, false }

func (NullSecretStore) Put(origin string, blob []byte) error { return nil }

func (NullSecretStore) Delete(origin string) bool { return false }

func (NullSecretStore) List() []string { return nil }
