package config

import "testing"

func TestIsLoopback(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"http://127.0.0.1:7676", true},
		{"https://127.0.0.1", true},
		{"http://localhost", true},
		{"https://localhost:443", true},
		{"http://[::1]:7676", true},
		{"https://[::1]", true},
		{"https://home.example.org", false},
		{"home.example.org", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := IsLoopback(tc.url); got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.url, got, tc.want)
		}
	}
}
