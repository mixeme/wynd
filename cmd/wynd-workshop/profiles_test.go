//go:build desktop

package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/workshop"
)

func profileTestApp(t *testing.T) (*App, workshop.ServerProfile, workshop.ServerProfile) {
	t.Helper()
	w, err := workshop.OpenWorkshop(filepath.Join(t.TempDir(), "servers.json"), func() *workshop.Wizard { return &workshop.Wizard{} })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Close)
	a := &App{ctx: context.Background(), workshop: w}
	p, err := a.SaveProfile(workshop.ServerProfile{Name: "Один", Host: "one.example.org", User: "root"})
	if err != nil {
		t.Fatal(err)
	}
	q, err := a.SaveProfile(workshop.ServerProfile{Name: "Другой", Host: "two.example.org", User: "root"})
	if err != nil {
		t.Fatal(err)
	}
	return a, p, q
}

func TestProfileBridgeDoesNotGuessAServerOrUseUnrequestedCredentials(t *testing.T) {
	a, p, q := profileTestApp(t)
	if _, err := a.Connect("missing", "", false); err != workshop.ErrProfileMissing {
		t.Fatal(err)
	}
	res, err := a.Connect(p.ID, "", false)
	if err != nil || res.Status != "password_required" {
		t.Fatal(res, err)
	}
	if res.Host != p.Host {
		t.Fatal("wrong profile", res)
	}
	if p, err := a.ServerPage(q.ID); err != nil || p.OK {
		t.Fatal("other profile unexpectedly connected", p, err)
	}
	if _, err := a.InstallProgress(""); err != workshop.ErrProfileMissing {
		t.Fatal("implicit selected profile", err)
	}
}

func TestSelfUpdateBridgeRejectsWorkOnAnyProfile(t *testing.T) {
	a, p, q := profileTestApp(t)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		done <- a.workshop.WithProfile(p.ID, false, func(_ *workshop.Wizard, _ workshop.ServerProfile) { close(entered); <-release })
	}()
	<-entered
	if _, err := a.ServerPage(q.ID); err != nil {
		t.Fatal("independent profile blocked", err)
	}
	if got := a.ApplySelfUpdate(); got != workshop.ErrWorkshopBusy.Error() {
		t.Fatal("self-update failed to guard all profiles", got)
	}
	if err := a.Disconnect(p.ID); err != workshop.ErrProfileBusy {
		t.Fatal("disconnected occupied profile", err)
	}
	if err := a.SetAutoConnect(p.ID, false); err != nil {
		t.Fatal("local preference blocked", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestProfileLoadErrorBlocksWritesWithoutCrashingTheWindow(t *testing.T) {
	err := errors.New("Не удалось прочитать список серверов")
	a := &App{profileError: err}
	if _, got := a.Profiles(); got != err {
		t.Fatal(got)
	}
	if _, got := a.SaveProfile(workshop.ServerProfile{Name: "Новый"}); got != err {
		t.Fatal(got)
	}
	if _, got := a.Connect("a", "", false); got != err {
		t.Fatal(got)
	}
	if got := a.ApplySelfUpdate(); got != err.Error() {
		t.Fatal(got)
	}
	a.shutdown(context.Background())
}
