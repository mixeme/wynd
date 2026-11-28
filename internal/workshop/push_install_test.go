package workshop

import (
	"context"
	"strings"
	"testing"
)

func TestPushInstallationUsesSeparateComposeAndChecksHTTPS(t *testing.T) {
	fastPolls(t)
	host := newFakeHost()
	host.files[installDir+"/.env"] = "EXISTING=yes"
	sess := dialHost(t, host)
	spec := testSpec(t)
	baseCompose := ComposeFile(spec)
	baseCaddy := CaddyFile(spec)
	spec.PushDomain = "https://Example.NET/"
	originalProbe := spec.Probe
	var probed bool
	spec.Probe = func(ctx context.Context, address string) (string, error) {
		if address == "https://example.net/v1/health" {
			probed = true
			return `{"healthy":true}`, nil
		}
		return originalProbe(ctx, address)
	}
	result := Install(t.Context(), sess, spec, nil)
	if !result.Done || !probed {
		t.Fatalf("install: %+v, push health checked: %v, log: %+v", result.Failure, probed, result.Log)
	}
	if host.files[installDir+"/compose.yaml"] != baseCompose {
		t.Fatal("primary Compose changed")
	}
	if got := host.files[installDir+"/Caddyfile"]; !strings.HasPrefix(got, baseCaddy) ||
		!strings.Contains(got, "example.net {") || !strings.Contains(got, "reverse_proxy push:80") {
		t.Fatalf("Caddyfile: %s", got)
	}
	if got := host.files[installDir+"/.env"]; got != "EXISTING=yes\n"+pushComposeLine+"\n" {
		t.Fatalf("dotenv: %q", got)
	}
	if got := host.files[installDir+"/push.json"]; got != "{\"domain\":\"example.net\",\"cache\":\"12h\"}\n" {
		t.Fatalf("push settings: %s", got)
	}
	if got := host.files[installDir+"/push.yaml"]; !strings.Contains(got, pushImage) ||
		!strings.Contains(got, `WYND_PUSH_SERVER: "https://example.net"`) ||
		!strings.Contains(got, "ntfy.yml:/etc/ntfy/server.yml:ro") || strings.Contains(got, "ports:") {
		t.Fatalf("Push Compose: %s", got)
	}
	if got := host.files[installDir+"/ntfy.yml"]; !strings.Contains(got, `cache-duration: "12h"`) ||
		!strings.Contains(got, "auth-default-access: deny-all") || !strings.Contains(got, `auth-access: ["*:up*:rw"]`) {
		t.Fatalf("ntfy config: %s", got)
	}

	host.changes = nil
	if result := Install(t.Context(), sess, spec, nil); !result.Done {
		t.Fatalf("retry: %+v", result.Failure)
	}
	if len(host.changes) != 1 || !strings.Contains(host.changes[0], "docker compose up -d") {
		t.Fatalf("retry mutated configuration: %q", host.changes)
	}
}

func TestPushInstallationRejectsBadSettingsBeforeChanges(t *testing.T) {
	host := newFakeHost()
	spec := testSpec(t)
	spec.PushDomain = spec.Domain
	result := Install(t.Context(), dialHost(t, host), spec, nil)
	if result.Failure == nil || result.Failure.Step != "push" || len(host.changes) != 0 {
		t.Fatalf("failure: %+v, remote changes: %q", result.Failure, host.changes)
	}
}

func TestMergePushEnv(t *testing.T) {
	for _, value := range []string{"COMPOSE_FILE=other.yaml", "export COMPOSE_FILE : other.yaml", "COMPOSE_PATH_SEPARATOR=;", pushComposeLine + "\n" + pushComposeLine} {
		if _, err := mergePushEnv(value); err == nil {
			t.Errorf("accepted competing Compose setting %q", value)
		}
	}
	before := "# keep CRLF\r\nCUSTOM=yes\r\n" + pushComposeLine + "\r\n"
	if got, err := mergePushEnv(before); err != nil || got != before {
		t.Fatalf("existing override changed: %q, %v", got, err)
	}
}

func TestWizardPushPlanChecksDNSAndKeepsChoice(t *testing.T) {
	w, _ := newWizard(t)
	host := startServer(t, "pw", nil, cleanUbuntu)
	w.Connect(t.Context(), ConnectInput{Host: host.host, Port: host.port, User: "root", Password: "pw"})
	if result := w.ConfirmHost(t.Context()); result.Status != StatusConnected {
		t.Fatalf("connect: %+v", result)
	}
	if result := w.Inspect(t.Context()); !result.OK {
		t.Fatalf("inspect: %+v", result)
	}
	w.Resolver = fakeResolver{
		"family.example.ru":      {"127.0.0.1"},
		"push.family.example.ru": {"127.0.0.1"},
		"example.net":            {"127.0.0.1"},
		"wrong.example.ru":       {"192.0.2.5"},
	}
	w.Source = SourceImage{Repo: "https://example.org/wynd"}
	plan := w.MakePlanWithPush(t.Context(), "family.example.ru", "Push.Family.Example.RU")
	if !plan.OK || plan.PushDomain != "push.family.example.ru" ||
		!strings.Contains(strings.Join(plan.Install, "\n"), "push.family.example.ru") {
		t.Fatalf("plan: %+v", plan)
	}
	plan = w.ChooseBuild(true)
	if !plan.OK || !plan.Build || w.specLocked(plan).PushDomain != "push.family.example.ru" {
		t.Fatalf("replanned: %+v", plan)
	}
	if plan := w.MakePlanWithPush(t.Context(), "family.example.ru", "example.net"); !plan.OK || w.specLocked(plan).PushDomain != "example.net" {
		t.Fatalf("separate domain: %+v", plan)
	}
	if plan := w.MakePlanWithPush(t.Context(), "family.example.ru", "wrong.example.ru"); plan.OK {
		t.Fatalf("accepted domain pointing to a different server: %+v", plan)
	}
	if result := w.StartInstall(t.Context()); result.Failure == nil || result.Running {
		t.Fatalf("stale plan could be started: %+v", result)
	}
}
