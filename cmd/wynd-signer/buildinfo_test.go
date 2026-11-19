//go:build dev || desktop || bindings

package main

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"
)

// У «Подписи» нет Go-константы версии: номер живёт только в ресурсах exe.
// Сторож — равенство полей info.json и версии манифеста (0.2.0 → 0.2.0.0),
// чтобы расхождение не уехало в выпуск.
func TestSignerBuildInfoVersion(t *testing.T) {
	raw, err := os.ReadFile("build/windows/info.json")
	if err != nil {
		t.Fatal(err)
	}
	var info struct {
		Fixed struct {
			FileVersion    string `json:"file_version"`
			ProductVersion string `json:"product_version"`
		} `json:"fixed"`
		Info map[string]struct {
			ProductVersion string `json:"ProductVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(raw, &info); err != nil {
		t.Fatal(err)
	}
	version := info.Fixed.ProductVersion
	if version == "" {
		t.Fatal("info.json: пустой product_version")
	}
	for name, got := range map[string]string{
		"fixed.file_version":    info.Fixed.FileVersion,
		"fixed.product_version": version,
		"info.0409":             info.Info["0409"].ProductVersion,
	} {
		if got != version {
			t.Fatalf("%s %q, product_version %q", name, got, version)
		}
	}

	manifest, err := os.ReadFile("build/windows/wails.exe.manifest")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`name="com\.wynd\.signer" version="([0-9.]+)"`).FindSubmatch(manifest)
	if m == nil {
		t.Fatal("в манифесте нет com.wynd.signer с версией")
	}
	if got, want := string(m[1]), version+".0"; got != want {
		t.Fatalf("манифест %q, ожидалось %q", got, want)
	}
}
