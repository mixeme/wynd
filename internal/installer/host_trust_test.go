package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func changedHostFixture(t *testing.T, shared bool) (Dialer, *testServer, ssh.PublicKey, string) {
	t.Helper()
	old := startServer(t, "pw", nil, "")
	current := startServer(t, "pw", nil, "")
	d := Dialer{SSHDir: filepath.Join(t.TempDir(), "ssh")}
	_, err := d.Dial(context.Background(), old.access("pw"))
	var unknown *UnknownHostError
	if !errors.As(err, &unknown) {
		t.Fatal(err)
	}
	if err := d.Trust(unknown); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(d.SSHDir, "known_hosts")
	hosts := []string{knownhosts.Normalize(current.addr)}
	if shared {
		hosts = append(hosts, "neighbour.example.org")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(knownhosts.Line(hosts, unknown.key) + "\n")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	return d, current, unknown.key, path
}

func TestWizardRequiresExplicitConfirmationOfChangedKey(t *testing.T) {
	d, srv, old, path := changedHostFixture(t, false)
	w := &Wizard{Dialer: d}
	defer w.Close()
	before, _ := os.ReadFile(path)
	res := w.Connect(context.Background(), ConnectInput{Host: srv.host, Port: srv.port, User: "root", Password: "pw"})
	if res.Status != StatusChangedHost || res.PreviousFingerprint != ssh.FingerprintSHA256(old) || res.Fingerprint == res.PreviousFingerprint {
		t.Fatalf("changed: %+v", res)
	}
	if w.ConfirmHost(context.Background()).Status != StatusError {
		t.Fatal("ordinary first-host confirmation trusted replacement")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("changed without confirmation")
	}
	if res := w.ConfirmChangedHost(context.Background()); res.Status != StatusConnected {
		t.Fatalf("confirmation: %+v", res)
	}
	check, err := knownhosts.New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := check(srv.addr, w.session.client.RemoteAddr(), old); err == nil {
		t.Fatal("retired key still trusted")
	}
	after, _ = os.ReadFile(path)
	firstLine := string(before[:len(before)-len(knownhosts.Line([]string{knownhosts.Normalize(srv.addr)}, old))-1])
	if len(firstLine) > 0 && string(after[:len(firstLine)]) != firstLine {
		t.Fatal("neighbour entry changed")
	}
	if w.ConfirmChangedHost(context.Background()).Status != StatusError {
		t.Fatal("confirmation reused")
	}
}

func TestReplaceHostRefusesSharedEntry(t *testing.T) {
	d, srv, _, path := changedHostFixture(t, true)
	before, _ := os.ReadFile(path)
	_, err := d.Dial(context.Background(), srv.access("pw"))
	var changed *ChangedHostError
	if !errors.As(err, &changed) {
		t.Fatal(err)
	}
	if err := d.ReplaceHost(changed); err == nil {
		t.Fatal("rewrote shared host entry")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("shared entry changed")
	}
}

func TestReplaceHostRefusesStaleConfirmation(t *testing.T) {
	d, srv, _, path := changedHostFixture(t, false)
	_, err := d.Dial(context.Background(), srv.access("pw"))
	var changed *ChangedHostError
	if !errors.As(err, &changed) {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# edited outside Workshop\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := d.ReplaceHost(changed); err == nil {
		t.Fatal("stale confirmation accepted")
	}
	after, _ := os.ReadFile(path)
	if string(after) != "# edited outside Workshop\n" {
		t.Fatal("external change lost")
	}
}
