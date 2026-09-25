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
