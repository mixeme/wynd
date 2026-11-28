package workshop

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type pushApplyHost struct {
	layout        Layout
	files         map[string]string
	commands      []string
	pushRunning   bool
	rejectProject bool
	failWynd      bool
	failRestore   bool
	interrupted   bool
}

func (h *pushApplyHost) exec(command string, input []byte) (string, int) {
	l := h.layout
	h.commands = append(h.commands, command)
	if strings.HasPrefix(command, "if [ -L ") {
		return h.files[strings.Fields(command)[3]], 0
	}
	if strings.HasSuffix(command, "echo WYND_PUSH_FILE") {
		_, rest, _ := strings.Cut(command, "\nfile=")
		path, _, _ := strings.Cut(rest, "\n")
		if strings.Contains(command, "sha256sum") && !strings.Contains(command, sum(h.files[path])) {
			return "unexpected digest", 2
		}
		if strings.Contains(command, `ln -T -- "$tmp" "$file"`) {
			if _, exists := h.files[path]; exists {
				return "already exists", 2
			}
		}
		if strings.Contains(command, `check && rm -- "$file"`) {
			delete(h.files, path)
		} else {
			h.files[path] = string(input)
		}
		return "WYND_PUSH_FILE\n", 0
	}
	switch {
	case command == "true":
		return "", 0
	case command == l.StateScript():
		out := "@image wynd:9.9.9\n@url https://family.example.ru\n@running wynd-id\n@container wynd-id\n"
		if _, exists := h.files[l.Root()+"/push.json"]; exists {
			out += "@push\n"
		}
		return out, 0
	case strings.HasSuffix(command, "echo WYND_PUSH_ROOT"):
		return "WYND_PUSH_ROOT\n", 0
	case strings.HasSuffix(command, "echo WYND_PUSH_PROJECT"):
		if h.rejectProject || (h.failRestore && h.interrupted) {
			return "", 2
		}
		return "WYND_PUSH_PROJECT\n", 0
	case strings.Contains(command, "config --format json"):
		return fmt.Sprintf(`{"name":"wynd", "services":{"wynd":{"image":"wynd:9.9.9","volumes":[{"type":"bind","source":%q,"target":"/data"}]},"caddy":{"image":%q,"volumes":[{"type":"bind","source":%q,"target":"/etc/caddy/Caddyfile"}]}}}`, l.Data(), caddyImage, l.Caddyfile()), 0
	case strings.Contains(command, "for f in push.json push.yaml ntfy.yml"):
		return "", 0
	case strings.Contains(command, "config --services"):
		return "wynd\ncaddy\n", 0
	case strings.Contains(command, "ps --quiet caddy"):
		return "caddy-id", 0
	case strings.HasPrefix(command, "test -f "):
		_, exists := h.files[strings.TrimPrefix(command, "test -f ")]
		return "", exit(!exists)
	case strings.Contains(command, "caddy validate --config /dev/stdin"):
		if !strings.Contains(string(input), "example.net") {
			return "missing Caddy site", 1
		}
		return "", 0
	case strings.Contains(command, "up -d --no-deps push"):
		h.pushRunning = true
		return "", 0
	case strings.Contains(command, "rm --stop --force push"):
		h.pushRunning = false
		return "", 0
	case strings.Contains(command, "up -d --no-deps --pull never wynd"):
		if h.failWynd && strings.Contains(command, "-f push.yaml") {
			h.interrupted = true
			return "failed to recreate wynd", 1
		}
		return "", 0
	case strings.Contains(command, "ps --all --quiet push"):
		return fmt.Sprintf("%t %s", h.pushRunning, pushImage), 0
	case strings.Contains(command, "config --quiet"), strings.Contains(command, "force-recreate caddy"),
		strings.Contains(command, "State.Health.Status"):
		return "", 0
	default:
		return "", 127
	}
}

func TestApplyPushAddCompletesOrRecoversWithoutLosingExistingConfiguration(t *testing.T) {
	for _, scenario := range []string{"success", "rollback", "resume", "conflict"} {
		t.Run(scenario, func(t *testing.T) {
			l, err := NewLayout("/srv/wynd-main")
			if err != nil {
				t.Fatal(err)
			}
			const env = "CUSTOM=secret-existing-value" // Last newline is intentionally absent.
			caddy := CaddyFile(Spec{Domain: "family.example.ru"}) + "\n# local settings\n"
			h := &pushApplyHost{layout: l, failWynd: scenario == "rollback" || scenario == "resume", failRestore: scenario == "resume", rejectProject: scenario == "conflict",
				files: map[string]string{l.Root() + "/.env": env, l.Caddyfile(): caddy, l.Compose(): ComposeFile(Spec{Domain: "family.example.ru", Version: "9.9.9"})}}
			w := &Wizard{session: dialTestHost(t, h.exec), report: &Report{}, Root: l.Root(),
				Resolver: fakeResolver{"example.net": {"127.0.0.1"}, "other.net": {"127.0.0.1"}},
				Probe: func(_ context.Context, url string) (string, error) {
					switch url {
					case "https://example.net/v1/health":
						return `{"healthy":true}`, nil
					case "https://family.example.ru/api/v1/instance":
						return `{"push_server":"https://example.net"}`, nil
					default:
						t.Errorf("unexpected probe %s", url)
						return "", nil
					}
				}}
			result := w.ApplyPushAdd(t.Context(), "example.net")
			if scenario == "resume" {
				if result.OK || result.Page.Push == nil || !result.Page.Push.Adding {
					t.Fatalf("interrupted operation was not visible: %+v", result)
				}
				if other := w.PlanPushAdd(t.Context(), "other.net"); other.OK || !strings.Contains(other.Advice, "example.net") {
					t.Fatalf("different domain was accepted: %+v", other)
				}
				h.failWynd, h.failRestore = false, false
				result = w.ApplyPushAdd(t.Context(), "example.net")
			}
			if scenario == "success" || scenario == "resume" {
				if !result.OK || result.Page.Push == nil || result.Page.Push.Adding || !result.Page.Push.Running ||
					h.files[l.Root()+"/.env"] != env+"\n"+pushComposeLine+"\n" ||
					h.files[l.Caddyfile()] != caddy+(pushSettings{Domain: "example.net"}).caddySite() {
					t.Fatalf("Push not active: %+v", result)
				}
			} else if result.OK || len(h.files) != 3 || h.files[l.Root()+"/.env"] != env || h.files[l.Caddyfile()] != caddy || h.pushRunning {
				t.Fatalf("original installation not intact: %+v", result)
			}
			if strings.Contains(strings.Join(h.commands, "\n"), "secret-existing-value") ||
				strings.Contains(h.files[l.Root()+"/push.json"], "secret-existing-value") {
				t.Fatal("existing .env value escaped into a command or journal")
			}
		})
	}
}
