package auth_test

import (
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

func TestParseParticipantEmail(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"ana@example.com", "ana@example.com", true},
		{"  Ana@Example.COM  ", "ana@example.com", true},
		{"Ana <ana@example.com>", "ana@example.com", true},
		{"no-at", "", false},
		{"@example.com", "", false},
		{"ana@", "", false},
		{"ana@localhost", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, err := auth.ParseParticipantEmail(tc.in)
		if tc.ok {
			if err != nil || got != tc.want {
				t.Fatalf("%q: got %q err %v want %q", tc.in, got, err, tc.want)
			}
		} else if err == nil {
			t.Fatalf("%q: expected error, got %q", tc.in, got)
		}
	}
}
