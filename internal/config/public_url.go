package config

import (
	"encoding/json"
	"errors"
	"fmt"
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

// WritePublicURL updates public_url in dataDir/config.json, creating the file if needed.
func WritePublicURL(dataDir, publicURL string) error {
	publicURL = NormalizePublicURL(publicURL)
	if publicURL == "" {
		return fmt.Errorf("empty public_url")
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
	if err := os.WriteFile(configPath, payload, 0o640); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
