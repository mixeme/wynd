package installer

import (
	"encoding/json"
	"os"
	"testing"
)

// Номер в свойствах exe (wails.json) — тот же, что в окне.
func TestWorkshopVersionMatchesWails(t *testing.T) {
	raw, err := os.ReadFile("../../cmd/wynd-installer/wails.json")
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
	if cfg.Info.ProductVersion != WorkshopVersion {
		t.Fatalf("wails.json productVersion %q, WorkshopVersion %q", cfg.Info.ProductVersion, WorkshopVersion)
	}
}
