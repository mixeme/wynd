package config

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestLoadPushServer(t *testing.T) {
	t.Setenv("WYND_DATA_DIR", t.TempDir())
	t.Setenv("WYND_PUBLIC_URL", DefaultPublicURL)
	t.Setenv("WYND_MOVE_ALLOWED_CIDRS", "")
	t.Setenv("WYND_MOVE_MAX_BYTES", "")

	var warnings bytes.Buffer
	writer := log.Writer()
	log.SetOutput(&warnings)
	t.Cleanup(func() { log.SetOutput(writer) })

	for _, tc := range []struct {
		name, raw, want string
		warn            bool
	}{
		{name: "unset"},
		{name: "origin", raw: "https://push.example.org", want: "https://push.example.org"},
		{name: "normalized", raw: " HTTPS://Push.Example.ORG.:443/ ", want: "https://push.example.org"},
		{name: "alternate port", raw: "https://push.example.org:8443/", want: "https://push.example.org:8443"},
		{name: "public IPv4", raw: "https://1.1.1.1/", want: "https://1.1.1.1"},
		{name: "public IPv6", raw: "https://[2606:4700:4700::1111]:8443/", want: "https://[2606:4700:4700::1111]:8443"},
		{name: "missing scheme", raw: "push.example.org", warn: true},
		{name: "HTTP", raw: "http://push.example.org", warn: true},
		{name: "path", raw: "https://push.example.org/up-private-topic", warn: true},
		{name: "escaped slash", raw: "https://push.example.org/%2F", warn: true},
		{name: "query", raw: "https://push.example.org?token=secret", warn: true},
		{name: "empty query", raw: "https://push.example.org?", warn: true},
		{name: "fragment", raw: "https://push.example.org#", warn: true},
		{name: "credentials", raw: "https://username:password@push.example.org", warn: true},
		{name: "empty host", raw: "https://:443", warn: true},
		{name: "invalid host", raw: "https://push..example.org", warn: true},
		{name: "bracketed name", raw: "https://[push.example.org]", warn: true},
		{name: "empty port", raw: "https://push.example.org:", warn: true},
		{name: "port overflow", raw: "https://push.example.org:65536", warn: true},
		{name: "unqualified host", raw: "https://push", warn: true},
		{name: "local domain", raw: "https://push.localhost", warn: true},
		{name: "private IPv4", raw: "https://192.168.1.20", warn: true},
		{name: "CGNAT", raw: "https://100.64.0.20", warn: true},
		{name: "link local IPv6", raw: "https://[fe80::1]", warn: true},
		{name: "mapped loopback", raw: "https://[::ffff:127.0.0.1]", warn: true},
		{name: "reserved IPv4", raw: "https://192.0.2.10", warn: true},
		{name: "alternate IP spelling", raw: "https://127.0.0.0x1", warn: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("WYND_PUSH_SERVER", tc.raw)
			warnings.Reset()
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.PushServer != tc.want {
				t.Fatalf("PushServer = %q, want %q", cfg.PushServer, tc.want)
			}
			if tc.warn {
				// Default log flags may prefix a timestamp.
				if !strings.HasSuffix(warnings.String(), "config: ignoring WYND_PUSH_SERVER: expected a public HTTPS origin\n") ||
					strings.Contains(warnings.String(), tc.raw) {
					t.Fatalf("unexpected log: %q", warnings.String())
				}
			} else if warnings.Len() != 0 {
				t.Fatalf("unexpected log: %q", warnings.String())
			}
		})
	}
}
