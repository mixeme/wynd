package installer

import (
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
	q := addProfile(t, w, p.Host)
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
	w, _ := testWorkshop(t)
	p := addProfile(t, w, "one.example.org")
	file := w.path
	w.path = filepath.Dir(w.path) // a directory cannot replace a file
	if err := w.SetAutoConnect(p.ID, true); err == nil {
		t.Fatal("write failure ignored")
	}
	if w.Profiles()[0].AutoConnect {
		t.Fatal("failed preference survived")
	}
	if err := w.RemoveProfile(p.ID); err == nil {
		t.Fatal("remove write failure ignored")
	}
	if len(w.Profiles()) != 1 {
		t.Fatal("lost profile after write failure")
	}
	if err := os.WriteFile(file, []byte(`{"version":2}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenWorkshop(file, func() *Wizard { return &Wizard{} }); err == nil {
		t.Fatal("unknown storage version accepted")
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
