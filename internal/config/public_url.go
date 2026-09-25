package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// NormalizePublicURL trims space, adds https:// when scheme is missing, drops a trailing slash.
func NormalizePublicURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	return strings.TrimRight(raw, "/")
}

// ValidatePublicURL нормализует адрес и отвергает то, что не является
// http(s)-адресом с хостом. Прежняя NormalizePublicURL принимала любую строку
// с «://», и `ftp://x` или `https://` спокойно доезжали до писем и ссылок
// (API-5).
func ValidatePublicURL(raw string) (string, error) {
	normalized := NormalizePublicURL(raw)
	if normalized == "" {
		return "", fmt.Errorf("public_url: пустой адрес")
	}
	u, err := url.Parse(normalized)
	if err != nil {
		return "", fmt.Errorf("public_url %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("public_url %q: схема должна быть http или https", raw)
	}
	if u.Host == "" || u.Hostname() == "" {
		return "", fmt.Errorf("public_url %q: не указан хост", raw)
	}
	if u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", fmt.Errorf("public_url %q: адрес без пути, запроса и якоря", raw)
	}
	if u.User != nil {
		return "", fmt.Errorf("public_url %q: без учётных данных в адресе", raw)
	}
	return normalized, nil
}

// WritePublicURL updates public_url in dataDir/config.json, creating the file if needed.
func WritePublicURL(dataDir, publicURL string) error {
	publicURL, err := ValidatePublicURL(publicURL)
	if err != nil {
		return err
	}
	configPath := filepath.Join(dataDir, "config.json")
	var fc fileConfig
	if data, err := os.ReadFile(configPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read config: %w", err)
		}
		fc = fileConfig{Listen: DefaultListen}
	} else if err := json.Unmarshal(data, &fc); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	if fc.Listen == "" {
		fc.Listen = DefaultListen
	}
	fc.PublicURL = publicURL
	payload, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	payload = append(payload, '\n')
	if err := os.MkdirAll(filepath.Dir(configPath), 0o750); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	return writeFileAtomic(configPath, payload, 0o640)
}
