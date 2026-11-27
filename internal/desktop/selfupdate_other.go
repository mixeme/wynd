//go:build !linux

package desktop

import (
	"context"
	"net/http"
	"os"
	"runtime"
)

// DetectUpdateKind — вне Linux обновляется сам exe программы.
func DetectUpdateKind() (UpdateKind, string) {
	exe, _ := os.Executable()
	return detectUpdateKind(runtime.GOOS, "", exe, nil)
}

// applyDeb вне Linux не вызывается: пакетов .deb здесь нет.
func applyDeb(
	_ context.Context,
	_ *http.Client,
	_ Update,
	_ func(done, total int64),
) (string, error) {
	return "", ErrNoUpdate
}
