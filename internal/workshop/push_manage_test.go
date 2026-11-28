package workshop

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
)

type pushTestHost struct {
	layout       Layout
	settings     pushSettings
	files        map[string]string
	commands     []string
	running      bool
	present      bool
	wyndRunning  bool
	caddyRunning bool
	fail         string
	otherEnv     string
	otherSite    string
}

func newPushTestHost(t *testing.T, running bool) (*pushTestHost, *Run) {
	t.Helper()
	l, err := NewLayout("/srv/wynd-push-test")
	if err != nil {
		t.Fatal(err)
	}
	p := pushSettings{Domain: "push.family.example.ru", Cache: defaultPushCache}
	spec := Spec{Domain: "family.example.ru", Version: "9.9.9"}
	h := &pushTestHost{
		layout: l, settings: p, present: true, running: running, caddyRunning: true,
		otherEnv:  "# keep CRLF\r\nCUSTOM=my-value\r\nCOMPOSE_REMOVE_ORPHANS=true\r\n",
		otherSite: CaddyFile(spec) + "\n# manually added comment\n",
	}
	h.files = map[string]string{
		l.Compose():        ComposeFile(spec),
		l.Root() + "/.env": h.otherEnv + pushComposeLine + "\n",
		l.Caddyfile():      h.otherSite + p.caddySite(),
	}
	for _, file := range p.installFiles(l) {
		h.files[file.path] = file.content
	}
	r := &Run{
		sess: dialTestHost(t, h.handle), layout: l, steps: []StepState{{ID: "push"}}, report: func() {},
		spec: Spec{Probe: func(_ context.Context, url string) (string, error) {
			if url != p.origin()+"/v1/health" || !h.running {
				t.Fatalf("unexpected health probe %q (running=%v)", url, h.running)
			}
			return `{"healthy":true}`, nil
		}},
	}
	return h, r
}

func (h *pushTestHost) handle(command string, input []byte) (string, int) {
	l := h.layout
	if strings.HasPrefix(command, "if [ -L ") {
		return h.files[strings.Fields(command)[3]], 0
	}
	switch command {
	case "true":
		return "", 0
	case l.StateScript():
		return "@image wynd:9.9.9\n@url https://family.example.ru\n@container wynd-id\n@push\n", 0
	}
	if strings.Contains(command, "echo WYND_PUSH_ROOT") {
		return "WYND_PUSH_ROOT\n", 0
	}
	if strings.HasPrefix(command, "cat > ") {
		path := strings.TrimSuffix(strings.Fields(command)[2], ".new")
		h.files[path] = string(input)
		return "", 0
	}
	if strings.HasPrefix(command, "umask 077; tmp=$(mktemp ") {
		h.files[l.Root()+"/.env"] = string(input)
		return "", 0
	}
	if strings.Contains(command, "ps --all --quiet push") {
		if !h.present {
			return "", 0
		}
		return fmt.Sprintf("%t %s\n", h.running, pushImage), 0
	}
	h.commands = append(h.commands, command)
	if h.fail != "" && strings.Contains(command, h.fail) {
		return "compose failed", 1
	}
	switch {
	case strings.Contains(command, "config --quiet"):
		return "", 0
	case strings.Contains(command, "ps --quiet wynd"):
		if h.wyndRunning {
			return "wynd-id\n", 0
		}
		return "", 0
	case strings.Contains(command, "ps --quiet caddy"):
		if h.caddyRunning {
			return "caddy-id\n", 0
		}
		return "", 0
	case strings.Contains(command, "up -d --no-deps") && strings.HasSuffix(command, " wynd 2>&1"):
		h.wyndRunning = !strings.Contains(command, "--no-start")
		return "", 0
	case strings.Contains(command, "up -d --no-deps --force-recreate") && strings.HasSuffix(command, " caddy 2>&1"):
		h.caddyRunning = !strings.Contains(command, "--no-start")
		return "", 0
	case strings.Contains(command, "State.Health.Status"):
		return "", exit(!h.wyndRunning)
	case command == l.startWynd(" --quiet-pull")+" 2>&1 && rm -f "+l.StoppedMark():
		h.wyndRunning = true
		return "", 0
	case strings.Contains(command, "stop push"):
		h.running = false
		return "", 0
	case strings.Contains(command, "rm --stop --force push"):
		h.running, h.present = false, false
		return "", 0
	case strings.Contains(command, "--no-deps") && strings.HasSuffix(command, " push 2>&1"):
		h.running, h.present = true, true
		return "", 0
	case strings.Contains(command, "rm -f -- push.yaml ntfy.yml && rm -f -- push.json"):
		for _, file := range h.settings.installFiles(l) {
			delete(h.files, file.path)
		}
		return "", 0
	}
	return "unexpected: " + command, 127
}

func TestPushServiceControlsOnlyPush(t *testing.T) {
	h, r := newPushTestHost(t, false)
	for _, action := range []string{ServiceStart, ServiceRestart, ServiceStop} {
		if err := managePush(t.Context(), r, action, ""); err != nil {
			t.Fatal(err)
		}
	}
	project := h.layout.pushCompose()
	want := []string{
		project + "up -d --no-deps push 2>&1",
		project + "restart --no-deps push 2>&1",
		project + "stop push 2>&1",
	}
	if !slices.Equal(h.commands, want) {
		t.Fatalf("commands: %q", h.commands)
	}
	w := &Wizard{session: r.sess, Root: h.layout.Root()}
	page := w.ServerPage(t.Context())
	if !page.OK || page.PushError != "" || page.Push == nil || page.Push.Running ||
		page.Push.Domain != h.settings.Domain || page.Push.Cache != defaultPushCache ||
		page.Push.Version != strings.TrimPrefix(strings.Split(pushImage, ":")[1], "v") {
		t.Fatalf("page: %+v, Push: %+v", page, page.Push)
	}
	if err := managePush(t.Context(), r, ServiceStart, ""); err != nil {
		t.Fatal(err)
	}
	state, err := readPushState(t.Context(), r)
	if err != nil || !state.Running || !state.Healthy {
		t.Fatalf("running Push: %+v, %v", state, err)
	}
}

func TestPushCacheSaveRetriesAndKeepsStopState(t *testing.T) {
	for _, running := range []bool{true, false} {
		t.Run(fmt.Sprint(running), func(t *testing.T) {
			h, r := newPushTestHost(t, running)
			if running {
				h.fail = "--force-recreate push"
				if err := managePush(t.Context(), r, PushSaveCache, "6h"); err == nil {
					t.Fatal("expected failure after config write")
				}
				pending, err := readPushState(t.Context(), r)
				if err != nil || pending.Cache != "12h" || pending.PendingCache != "6h" {
					t.Fatalf("pending: %+v, %v", pending, err)
				}
				h.fail = ""
			}
			if _, err := pushActionAt(t.Context(), r.sess, r.layout, PushSaveCache, "invalid"); err == nil {
				t.Fatal("accepted invalid cache")
			}
			if err := managePush(t.Context(), r, PushSaveCache, "6h"); err != nil {
				t.Fatal(err)
			}
			actual, err := readPushSettings(t.Context(), r)
			if err != nil || actual.Cache != "6h" || actual.Change != nil ||
				h.files[actual.ntfyFile(r.layout).path] != actual.ntfyFile(r.layout).content || h.running != running {
				t.Fatalf("cache was not applied: %+v, %v", actual, err)
			}
			if (running && len(h.commands) != 2) || (!running && len(h.commands) != 0) {
				t.Fatalf("unexpected commands: %q", h.commands)
			}
			for _, cmd := range h.commands {
				if cmd != r.layout.pushCompose()+"up -d --no-deps --force-recreate push 2>&1" {
					t.Fatalf("cache affected other services: %q", cmd)
				}
			}
		})
	}
}

func TestPushRemovalResumesAndPreservesOtherConfiguration(t *testing.T) {
	h, r := newPushTestHost(t, true)
	base := h.files[r.layout.Compose()]
	h.fail = "--force-recreate caddy"
	if _, err := pushActionAt(t.Context(), r.sess, r.layout, PushRemove, ""); err == nil {
		t.Fatal("expected failure applying Caddy configuration")
	}
	st, err := readPushState(t.Context(), r)
	if err != nil || !st.Removing || h.running {
		t.Fatalf("removal was not recorded: %+v, %v", st, err)
	}
	h.fail = ""
	log, err := pushActionAt(t.Context(), r.sess, r.layout, PushRemove, "")
	if err != nil {
		t.Fatalf("retry: %v, %+v", err, log)
	}
	for _, file := range h.settings.installFiles(r.layout) {
		if _, ok := h.files[file.path]; ok {
			t.Fatalf("file left behind: %s", file.path)
		}
	}
	if env := h.files[r.layout.Root()+"/.env"]; env != h.otherEnv {
		t.Fatalf("other env altered: %q", env)
	}
	if h.files[r.layout.Compose()] != base || h.files[r.layout.Caddyfile()] != h.otherSite {
		t.Fatal("the base project or other Caddy entries changed")
	}
	if h.wyndRunning || !h.caddyRunning {
		t.Fatal("removal changed which base services were running")
	}
	for _, cmd := range h.commands {
		if strings.Contains(cmd, "--remove-orphans") || strings.Contains(cmd, " down ") ||
			strings.Contains(cmd, "volume rm") || strings.Contains(cmd, "rm -rf") {
			t.Fatalf("unexpected deletion: %q", cmd)
		}
		if strings.Contains(cmd, "docker compose ") && !strings.Contains(cmd, composeKeepOrphans) {
			t.Fatalf("command inherited orphan removal from .env: %q", cmd)
		}
	}
}

func TestPushRemovalChecksRunningWyndAndKeepsCaddyStopped(t *testing.T) {
	h, r := newPushTestHost(t, true)
	h.wyndRunning, h.caddyRunning = true, false
	if err := managePush(t.Context(), r, PushRemove, ""); err != nil {
		t.Fatal(err)
	}
	if !h.wyndRunning || h.caddyRunning || !slices.ContainsFunc(h.commands, func(command string) bool {
		return strings.Contains(command, "State.Health.Status")
	}) {
		t.Fatalf("incorrect base service state after removal: %q", h.commands)
	}
}

func TestMainServiceStartKeepsPushStopped(t *testing.T) {
	h, r := newPushTestHost(t, false)
	if _, err := ServiceActionAt(t.Context(), r.sess, r.layout, ServiceStart); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.commands[0], "up -d --quiet-pull wynd caddy; else") || !h.wyndRunning || h.running {
		t.Fatalf("main start affected Push: %q", h.commands)
	}
}

func TestPushRemovalRejectsModifiedCaddyBeforeMutation(t *testing.T) {
	h, r := newPushTestHost(t, true)
	path := r.layout.Caddyfile()
	h.files[path] = strings.Replace(h.files[path], "push:80", "custom:1234", 1)
	original := h.files[r.layout.Root()+"/push.json"]
	if err := managePush(t.Context(), r, PushRemove, ""); err == nil || !h.running ||
		len(h.commands) != 0 || h.files[r.layout.Root()+"/push.json"] != original {
		t.Fatalf("removal modified an operator-managed project: %v", err)
	}
}
