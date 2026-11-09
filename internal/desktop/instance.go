package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrNoServer — сервер не ответил или ответил не по-нашему. Причина человеку
// не различается: опечатка, нет сети, выключенный сервер и чужой сайт выглядят
// одинаково (ПК4).
var ErrNoServer = errors.New("server did not answer")

// Instance — та часть /api/v1/instance, которая нужна оболочке до входа.
type Instance struct {
	Name             string `json:"name"`
	Version          string `json:"version"`
	RegistrationMode string `json:"registration_mode"`
}

// CheckResult — проверенный сервер для плашки и списка.
type CheckResult struct {
	Origin           string `json:"origin"`
	Name             string `json:"name"`
	Version          string `json:"version"`
	RegistrationMode string `json:"registration_mode"`
}

// CheckServer проверяет адрес запросом к /api/v1/instance. Запрос идёт из Go,
// а не из страницы: до выбранного сервера у окна ещё нет ни origin, ни CORS.
func CheckServer(ctx context.Context, client *http.Client, raw string) (CheckResult, error) {
	origin, err := NormalizeAddress(raw)
	if err != nil {
		return CheckResult{}, err
	}
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/api/v1/instance", nil)
	if err != nil {
		return CheckResult{}, fmt.Errorf("%w: %v", ErrNoServer, err)
	}
	req.Header.Set("Accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return CheckResult{}, fmt.Errorf("%w: %v", ErrNoServer, err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return CheckResult{}, fmt.Errorf("%w: status %d", ErrNoServer, res.StatusCode)
	}
	var info Instance
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&info); err != nil {
		return CheckResult{}, fmt.Errorf("%w: %v", ErrNoServer, err)
	}
	if strings.TrimSpace(info.Name) == "" {
		info.Name = HostOnly(origin)
	}
	return CheckResult{
		Origin:           origin,
		Name:             info.Name,
		Version:          info.Version,
		RegistrationMode: info.RegistrationMode,
	}, nil
}
