package main

import "testing"

func TestListensOnLoopback(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:7676": true,
		"localhost:7676": true,
		"[::1]:7676":     true,
		"0.0.0.0:7676":   false,
		":7676":          false,
		"[::]:7676":      false,
		"192.168.1.5:80": false,
		"garbage":        false,
	}
	for listen, want := range cases {
		if got := listensOnLoopback(listen); got != want {
			t.Errorf("listensOnLoopback(%q) = %v, want %v", listen, got, want)
		}
	}
}
