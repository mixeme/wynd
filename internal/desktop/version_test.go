package desktop

import (
	"encoding/json"
	"os"
	"testing"
)

// Номер в свойствах exe (wails.json) — тот же, что в окне.
func TestDesktopVersionMatchesWails(t *testing.T) {
	raw, err := os.ReadFile("../../cmd/wynd-desktop/wails.json")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Info.ProductVersion != Version {
		t.Fatalf("wails.json productVersion %q, Version %q", cfg.Info.ProductVersion, Version)
	}
}
