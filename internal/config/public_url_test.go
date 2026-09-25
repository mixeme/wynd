package config

import "testing"

func TestNormalizePublicURL(t *testing.T) {
	if got := NormalizePublicURL("home.example.org"); got != "https://home.example.org" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizePublicURL("https://a.test/"); got != "https://a.test" {
		t.Fatalf("got %q", got)
	}
}

// Инвариант (API-5): строгая проверка адреса. Прежняя нормализация принимала
// любую строку с «://», и ftp://x доезжал до писем и ссылок.
func TestValidatePublicURL(t *testing.T) {
	good := map[string]string{
		"home.example.org":         "https://home.example.org",
		"https://home.example.org": "https://home.example.org",
		"http://127.0.0.1:7676":    "http://127.0.0.1:7676",
		" https://x.example/ ":     "https://x.example",
	}
	for raw, want := range good {
		got, err := ValidatePublicURL(raw)
		if err != nil {
			t.Fatalf("ValidatePublicURL(%q): %v", raw, err)
		}
		if got != want {
			t.Fatalf("ValidatePublicURL(%q) = %q, want %q", raw, got, want)
		}
	}
	bad := []string{"", "   ", "ftp://x.example", "https://", "://x", "https://x.example/path", "https://u:p@x.example"}
	for _, raw := range bad {
		if got, err := ValidatePublicURL(raw); err == nil {
			t.Fatalf("ValidatePublicURL(%q) = %q, want error", raw, got)
		}
	}
}
