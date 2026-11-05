package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testWorkshop(t *testing.T) (*Workshop, memVault) {
	t.Helper()
	vault := memVault{}
	w, err := OpenWorkshop(filepath.Join(t.TempDir(), "servers.json"), func() *Wizard { return &Wizard{Vault: vault} })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Close)
	return w, vault
}

func addProfile(t *testing.T, w *Workshop, host string) ServerProfile {
	t.Helper()
	p, err := w.SaveProfile(ServerProfile{Name: "Семейный сервер", Host: host, User: "root", UseKeys: true})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestWorkshopProfilesSurviveRestartWithoutSecrets(t *testing.T) {
	w, _ := testWorkshop(t)
	p := addProfile(t, w, "family.example.org")
	if err := w.SetAutoConnect(p.ID, true); err != nil {
		t.Fatal(err)
	}
	check := ServerCheck{At: time.Now().UTC(), Domain: "site.example.org", Installed: true, Running: true}
	if err := w.RememberCheck(p.ID, check); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(w.path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "password") || strings.Contains(string(data), "raw") {
		t.Fatal("credential/output on disk", string(data))
	}
	loaded, err := OpenWorkshop(w.path, func() *Wizard { return &Wizard{} })
	if err != nil {
		t.Fatal(err)
	}
	defer loaded.Close()
	got := loaded.Profiles()
	if len(got) != 1 || got[0].ID != p.ID || !got[0].AutoConnect || got[0].LastCheck.Domain != check.Domain || got[0].Busy {
		t.Fatalf("restart: %+v", got)
	}
	got[0].LastCheck.Domain = "outside mutation"
	if loaded.Profiles()[0].LastCheck.Domain != check.Domain {
		t.Fatal("snapshot aliases storage")
	}
}

func TestWorkshopDisablingAutoConnectDoesNotWaitForSSH(t *testing.T) {
	w, _ := testWorkshop(t)
	p := addProfile(t, w, "one.example.org")
	q := addProfile(t, w, "two.example.org")
	w.SetAutoConnect(p.ID, true)
	w.SetAutoConnect(q.ID, true)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		done <- w.WithProfile(p.ID, false, func(_ *Wizard, _ ServerProfile) { close(entered); <-release })
	}()
	<-entered
	if err := w.SetAutoConnect(p.ID, false); err != nil {
		t.Fatal(err)
	}
	if w.Profiles()[0].AutoConnect || !w.Profiles()[1].AutoConnect {
		t.Fatal("flags not independent")
	}
	if err := w.RemoveProfile(p.ID); err != ErrProfileBusy {
		t.Fatal("removed occupied profile", err)
	}
	if err := w.BeginSelfUpdate(); err != ErrWorkshopBusy {
		t.Fatal("restart during SSH", err)
	}
	if err := w.WithProfile(q.ID, false, func(_ *Wizard, _ ServerProfile) {}); err != nil {
		t.Fatal("other profile blocked", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := w.BeginSelfUpdate(); err != nil {
		t.Fatal(err)
	}
	if err := w.WithProfile(q.ID, false, func(_ *Wizard, _ ServerProfile) {}); err != ErrWorkshopBusy {
		t.Fatal("new work during update", err)
	}
	w.EndSelfUpdate()
}

func TestWorkshopBackgroundOperationBelongsToProfile(t *testing.T) {
	w, _ := testWorkshop(t)
	p := addProfile(t, w, "one.example.org")
	q := addProfile(t, w, "two.example.org")
	if err := w.WithProfile(p.ID, false, func(wiz *Wizard, _ ServerProfile) { wiz.progress = Progress{Running: true} }); err != nil {
		t.Fatal(err)
	}
	p.Name = "changed"
	if _, err := w.SaveProfile(p); err != ErrProfileBusy {
		t.Fatal("edited occupied profile", err)
	}
	if err := w.RemoveProfile(p.ID); err != ErrProfileBusy {
		t.Fatal(err)
	}
	if err := w.WithProfile(p.ID, false, func(_ *Wizard, _ ServerProfile) { t.Fatal("conflicting operation") }); err != ErrProfileBusy {
		t.Fatal(err)
	}
	if err := w.WithProfile(q.ID, false, func(wiz *Wizard, _ ServerProfile) {
		if wiz.progress.Running {
			t.Fatal("shared progress")
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := w.BeginSelfUpdate(); err != ErrWorkshopBusy {
		t.Fatal(err)
	}
	w.WithProfile(p.ID, true, func(wiz *Wizard, _ ServerProfile) { wiz.progress = Progress{Done: true} })
	if err := w.BeginSelfUpdate(); err != nil {
		t.Fatal(err)
	}
}

func TestWorkshopPasswordAutoConnectAndSharedVault(t *testing.T) {
	w, vault := testWorkshop(t)
	p, err := w.SaveProfile(ServerProfile{Name: "Пароль", Host: "one.example.org", User: "root", Remember: true, AutoConnect: true})
	if err != nil {
		t.Fatal(err)
	}
	if p.AutoConnect {
		t.Fatal("enabled before storing password")
	}
	if err := w.SetAutoConnect(p.ID, true); err == nil {
		t.Fatal("enabled without password")
	}
	vault.Set(p.Host, p.User, "secret")
	if err := w.SetAutoConnect(p.ID, true); err != nil {
		t.Fatal(err)
	}
	q, err := w.SaveProfile(ServerProfile{Name: "Тот же доступ", Host: p.Host, User: p.User, Remember: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.RemoveProfile(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := vault.Get(p.Host, p.User); !ok {
		t.Fatal("deleted shared password")
	}
	if err := w.RemoveProfile(q.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := vault.Get(p.Host, p.User); ok {
		t.Fatal("forgotten password remains")
	}
}

func TestWorkshopRefusesCorruptFileAndRollsBackFailedWrites(t *testing.T) {
	w, vault := testWorkshop(t)
	p := addProfile(t, w, "one.example.org")
	file := w.path
	w.path = filepath.Dir(w.path) // a directory cannot replace a file
	if err := w.SetAutoConnect(p.ID, true); err == nil {
		t.Fatal("write failure ignored")
	}
	if w.Profiles()[0].AutoConnect {
		t.Fatal("failed preference survived")
	}
	vault.Set(p.Host, p.User, "restore-after-failed-write")
	if err := w.RemoveProfile(p.ID); err == nil {
		t.Fatal("remove write failure ignored")
	}
	if len(w.Profiles()) != 1 {
		t.Fatal("lost profile after write failure")
	}
	if value, ok := vault.Get(p.Host, p.User); !ok || value != "restore-after-failed-write" {
		t.Fatal("lost password after failed write")
	}
	if err := os.WriteFile(file, []byte(`{"version":2}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenWorkshop(file, func() *Wizard { return &Wizard{} }); err == nil {
		t.Fatal("unknown storage version accepted")
	}
}

type blockedDeleteVault struct {
	memVault
	entered, release chan struct{}
	err              error
}

func (v *blockedDeleteVault) Delete(host, user string) error {
	if v.entered != nil {
		close(v.entered)
		<-v.release
	}
	if v.err != nil {
		return v.err
	}
	return v.memVault.Delete(host, user)
}

func TestWorkshopVaultFailureKeepsProfile(t *testing.T) {
	vault := &blockedDeleteVault{memVault: memVault{}, err: errors.New("vault unavailable")}
	w, err := OpenWorkshop(filepath.Join(t.TempDir(), "servers.json"), func() *Wizard { return &Wizard{Vault: vault} })
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	p := addProfile(t, w, "one.example.org")
	if err := w.RemoveProfile(p.ID); err == nil {
		t.Fatal("vault error lost")
	}
	if len(w.Profiles()) != 1 {
		t.Fatal("profile removed despite failed credential cleanup")
	}
}

func TestWorkshopLocalPreferenceDoesNotWaitForVaultRemoval(t *testing.T) {
	vault := &blockedDeleteVault{memVault: memVault{}, entered: make(chan struct{}), release: make(chan struct{})}
	w, err := OpenWorkshop(filepath.Join(t.TempDir(), "servers.json"), func() *Wizard { return &Wizard{Vault: vault} })
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	p, q := addProfile(t, w, "one.example.org"), addProfile(t, w, "two.example.org")
	w.SetAutoConnect(q.ID, true)
	done := make(chan error, 1)
	go func() { done <- w.RemoveProfile(p.ID) }()
	<-vault.entered
	if err := w.SetAutoConnect(q.ID, false); err != nil {
		t.Fatal(err)
	}
	saved := make(chan error, 1)
	go func() {
		_, err := w.SaveProfile(ServerProfile{Name: "duplicate", Host: p.Host, User: p.User})
		saved <- err
	}()
	select {
	case <-saved:
		t.Fatal("profile registration raced credential deletion")
	default:
	}
	close(vault.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := <-saved; err != nil {
		t.Fatal(err)
	}
}

func TestWorkshopLoginDeletesOnlyAnUnsharedRememberedPassword(t *testing.T) {
	w, vault := testWorkshop(t)
	srv := startServer(t, "pw", nil, "")
	dial := Dialer{SSHDir: filepath.Join(t.TempDir(), "ssh")}
	trusted(t, dial, srv.access("pw"))
	p, err := w.SaveProfile(ServerProfile{Name: "Один", Host: srv.host, User: "root"})
	if err != nil {
		t.Fatal(err)
	}
	login := func() {
		t.Helper()
		if err := w.WithProfile(p.ID, false, func(wiz *Wizard, _ ServerProfile) {
			wiz.Dialer = dial
			if res := wiz.Connect(context.Background(), ConnectInput{Host: srv.host, Port: srv.port, User: "root", Password: "pw"}); res.Status != StatusConnected {
				t.Fatal(res)
			}
		}); err != nil {
			t.Fatal(err)
		}
	}
	vault.Set(p.Host, p.User, "pw")
	login()
	if _, ok := vault.Get(p.Host, p.User); ok {
		t.Fatal("unremembered password retained")
	}
	q, err := w.SaveProfile(ServerProfile{Name: "Другой профиль", Host: srv.host, User: "root", Remember: true})
	if err != nil {
		t.Fatal(err)
	}
	vault.Set(p.Host, p.User, "pw")
	login()
	if _, ok := vault.Get(p.Host, p.User); !ok {
		t.Fatal("shared password deleted")
	}
	if err := w.RemoveProfile(q.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := vault.Get(p.Host, p.User); ok {
		t.Fatal("last remembered reference removed but password retained")
	}
}

func TestWorkshopMoveRefusesBusyDestinationUnderAnotherSSHName(t *testing.T) {
	w, _ := testWorkshop(t)
	s, d, alias := addProfile(t, w, "old.example.org"), addProfile(t, w, "new.example.org"), addProfile(t, w, "alias.example.org")
	old, target := startServer(t, "pw", nil, ""), startServer(t, "pw", nil, "")
	dial := Dialer{SSHDir: filepath.Join(t.TempDir(), "ssh")}
	trusted(t, dial, old.access("pw"))
	trusted(t, dial, target.access("pw"))
	for _, item := range []struct {
		id  string
		srv *testServer
	}{{s.ID, old}, {alias.ID, target}} {
		if err := w.WithProfile(item.id, false, func(wiz *Wizard, _ ServerProfile) {
			wiz.Dialer = dial
			res := wiz.Connect(context.Background(), ConnectInput{Host: item.srv.host, Port: item.srv.port, User: "root", Password: "pw"})
			if res.Status != StatusConnected {
				t.Fatal(res)
			}
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.WithProfile(alias.ID, false, func(wiz *Wizard, _ ServerProfile) { wiz.progress = Progress{Running: true} }); err != nil {
		t.Fatal(err)
	}
	if _, err := w.ReserveMove(s.ID, d.ID); err != nil {
		t.Fatal(err)
	}
	if err := w.WithMoveProfile(s.ID, false, func(wiz *Wizard, _ ServerProfile) {
		res := wiz.MoveConnect(context.Background(), ConnectInput{Host: target.host, Port: target.port, User: "root", Password: "pw"})
		if res.Status != StatusConnected {
			t.Fatal(res)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := w.BindMoveEndpoint(s.ID); err != ErrProfileBusy {
		t.Fatal("busy alias accepted", err)
	}
}

func TestWorkshopMoveReservesBothProfiles(t *testing.T) {
	w, _ := testWorkshop(t)
	s := addProfile(t, w, "old.example.org")
	d := addProfile(t, w, "new.example.org")
	if _, err := w.ReserveMove(s.ID, s.ID); err == nil {
		t.Fatal("move to source")
	}
	if _, err := w.ReserveMove(s.ID, d.ID); err != nil {
		t.Fatal(err)
	}
	if err := w.WithProfile(d.ID, false, func(_ *Wizard, _ ServerProfile) { t.Fatal("used destination") }); err != ErrProfileBusy {
		t.Fatal(err)
	}
	if err := w.RemoveProfile(d.ID); err != ErrProfileBusy {
		t.Fatal(err)
	}
	if err := w.SetAutoConnect(d.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := w.BeginSelfUpdate(); err != ErrWorkshopBusy {
		t.Fatal(err)
	}
	if id, err := w.ReleaseMove(s.ID); err != nil || id != d.ID {
		t.Fatal(id, err)
	}
	if err := w.WithProfile(d.ID, false, func(_ *Wizard, _ ServerProfile) {}); err != nil {
		t.Fatal(err)
	}
}

func TestWorkshopMoveKeepsSourceAndTransfersDestinationSession(t *testing.T) {
	w, _ := testWorkshop(t)
	s, d := addProfile(t, w, "old.example.org"), addProfile(t, w, "new.example.org")
	old, newServer := startServer(t, "pw", nil, ""), startServer(t, "pw", nil, "")
	dial := Dialer{SSHDir: filepath.Join(t.TempDir(), "ssh")}
	trusted(t, dial, old.access("pw"))
	trusted(t, dial, newServer.access("pw"))
	ctx := context.Background()
	for _, item := range []struct {
		id  string
		srv *testServer
	}{{s.ID, old}, {d.ID, newServer}} {
		if err := w.WithProfile(item.id, false, func(wiz *Wizard, _ ServerProfile) {
			wiz.Dialer = dial
			in := ConnectInput{Host: item.srv.host, Port: item.srv.port, User: "root", Password: "pw"}
			if res := wiz.Connect(ctx, in); res.Status != StatusConnected {
				t.Fatalf("connect: %+v", res)
			}
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.ReserveMove(s.ID, d.ID); err != nil {
		t.Fatal(err)
	}
	in, err := w.MoveInput(s.ID, d.ID, "")
	if err != nil || in.Password != "pw" || in.Remember {
		t.Fatalf("ephemeral credential: %+v %v", in, err)
	}
	if err := w.WithProfile(s.ID, false, func(_ *Wizard, _ ServerProfile) { t.Fatal("source service during reserved move") }); err != ErrProfileBusy {
		t.Fatal(err)
	}
	if err := w.WithMoveProfile(s.ID, false, func(wiz *Wizard, _ ServerProfile) {
		if res := wiz.MoveConnect(ctx, ConnectInput{Host: newServer.host, Port: newServer.port, User: "root", Password: "pw"}); res.Status != StatusConnected {
			t.Fatal(res)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if id, err := w.ReleaseMove(s.ID); err != nil || id != d.ID {
		t.Fatal(id, err)
	}
	for _, item := range []struct{ id, addr string }{{s.ID, old.addr}, {d.ID, newServer.addr}} {
		if err := w.WithProfile(item.id, false, func(wiz *Wizard, _ ServerProfile) {
			if wiz.session.client.RemoteAddr().String() != item.addr {
				t.Fatal("profile identity changed")
			}
			if _, err := wiz.session.Exec(ctx, "probe", nil, shortTimeout); err != nil {
				t.Fatal("transferred session closed", err)
			}
		}); err != nil {
			t.Fatal(err)
		}
	}
}
