package desktop

import (
	"encoding/json"
	"os"
	"testing"
)

// Номер в свойствах exe (build/windows/info.json, его вшивает
// `wails3 generate syso`) — тот же, что в окне.
func TestDesktopVersionMatchesBuildInfo(t *testing.T) {
	raw, err := os.ReadFile("../../cmd/wynd-desktop/build/windows/info.json")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Fixed struct {
			FileVersion    string `json:"file_version"`
			ProductVersion string `json:"product_version"`
		} `json:"fixed"`
		Info map[string]struct {
			ProductVersion string `json:"ProductVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]string{
		"fixed.file_version":    cfg.Fixed.FileVersion,
		"fixed.product_version": cfg.Fixed.ProductVersion,
		"info.0419":             cfg.Info["0419"].ProductVersion,
	} {
		if got != Version {
			t.Fatalf("%s %q, Version %q", name, got, Version)
		}
	}
}
