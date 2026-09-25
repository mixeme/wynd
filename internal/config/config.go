package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultListen binds to loopback: the documented deployments put a
	// reverse proxy on the same host, and a loopback dev instance must not be
	// reachable from the LAN over plain HTTP. Docker sets WYND_LISTEN=:7676.
	DefaultListen    = "127.0.0.1:7676"
	DefaultPublicURL = "http://127.0.0.1:7676"
	DefaultDataDir   = "dev/data"
)

type fileConfig struct {
	Listen         string   `json:"listen"`
	PublicURL      string   `json:"public_url"`
	TrustedProxies []string `json:"trusted_proxies,omitempty"`
}

type Config struct {
	DataDir   string
	Listen    string
	PublicURL string
	// TrustedProxies lists CIDRs (or bare addresses) whose X-Forwarded-For
	// header is believed. Empty means loopback only.
	TrustedProxies []string
}

// Load builds the config from defaults, then config.json, then WYND_* variables. It never writes config.json; it only creates the data directory, blobs/, keys/ and a 0600 wynd.db.
func Load() (*Config, error) {
	cfg, err := read()
	if err != nil {
		return nil, err
	}
	if err := ensureDataLayout(cfg.DataDir); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ErrNoDatabase — в каталоге данных нет непустого wynd.db.
var ErrNoDatabase = errors.New("no database in data dir")

// LoadExisting reads the config like Load but creates nothing and fails with
// ErrNoDatabase when the data directory holds no non-empty wynd.db.
//
// Для служебных команд (`wynd backup`, `wynd admin-password`): опечатка в
// WYND_DATA_DIR раньше оставляла после них каталог с blobs/, keys/ и пустым
// wynd.db — Load готовит каталог к первому запуску сервера, а команде нужен
// уже существующий.
func LoadExisting() (*Config, error) {
	cfg, err := read()
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(filepath.Join(cfg.DataDir, "wynd.db"))
	if err != nil || info.Size() == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoDatabase, cfg.DataDir)
	}
	return cfg, nil
}

// read собирает конфигурацию, ничего не создавая на диске.
func read() (*Config, error) {
	dataDir := os.Getenv("WYND_DATA_DIR")
	if dataDir == "" {
		dataDir = DefaultDataDir
	}

	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve data dir: %w", err)
	}

	cfg := &Config{
		DataDir:   absDataDir,
		Listen:    DefaultListen,
		PublicURL: DefaultPublicURL,
	}

	configPath := filepath.Join(absDataDir, "config.json")
	if data, err := os.ReadFile(configPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read config: %w", err)
		}
		// Чтение конфигурации ничего не пишет: файл появляется, когда его
		// действительно сохраняют (bootstrap, панель). Иначе `wynd backup`
		// при неверном WYND_DATA_DIR создавал пустой каталог и «успешно»
		// бэкапил пустоту (API-5, STB-4).
	} else {
		var fc fileConfig
		if err := json.Unmarshal(data, &fc); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
		if fc.Listen != "" {
			cfg.Listen = fc.Listen
		}
		if fc.PublicURL != "" {
			cfg.PublicURL = fc.PublicURL
		}
		cfg.TrustedProxies = fc.TrustedProxies
	}

	if v := os.Getenv("WYND_LISTEN"); v != "" {
		cfg.Listen = v
	}
	if v := os.Getenv("WYND_PUBLIC_URL"); v != "" {
		if cfg.PublicURL != "" && cfg.PublicURL != v && cfg.PublicURL != DefaultPublicURL {
			log.Printf("config: WYND_PUBLIC_URL=%q перекрывает сохранённый %q", v, cfg.PublicURL)
		}
		cfg.PublicURL = v
	}
	if v, ok := os.LookupEnv("WYND_TRUSTED_PROXIES"); ok {
		cfg.TrustedProxies = splitList(v)
	}

	normalized, err := ValidatePublicURL(cfg.PublicURL)
	if err != nil {
		return nil, err
	}
	cfg.PublicURL = normalized
	return cfg, nil
}

func splitList(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func ensureDataLayout(dataDir string) error {
	dirs := []string{
		dataDir,
		filepath.Join(dataDir, "blobs"),
		filepath.Join(dataDir, "keys"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}

	// БД хранит SMTP-пароль и ключ VAPID открытым текстом — принятый риск
	// при условии прав 0600 (план 42, DEC-1). Существующий файл выравнивается
	// при старте; -wal/-shm SQLite создаёт с режимом основного файла.
	dbPath := filepath.Join(dataDir, "wynd.db")
	f, err := os.OpenFile(dbPath, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create wynd.db: %w", err)
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Chmod(dbPath, 0o600)
}
