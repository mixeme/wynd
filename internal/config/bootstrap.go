package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BootstrapToken reads keys/bootstrap, creating it with a new random token (0600) on first start.
func BootstrapToken(dataDir string) (string, error) {
	path := filepath.Join(dataDir, "keys", "bootstrap")
	if data, err := os.ReadFile(path); err == nil {
		token := strings.TrimSpace(string(data))
		if token != "" {
			return token, nil
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read bootstrap token: %w", err)
	}

	token, err := newBootstrapToken()
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("write bootstrap token: %w", err)
	}
	return token, nil
}

func newBootstrapToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate bootstrap token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
