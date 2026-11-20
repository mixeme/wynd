//go:build dev || desktop || bindings

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"

	"github.com/wailsapp/wails/v3/pkg/application"

	"gitea.mixdep.ru/mix/wynd/internal/signing"
)

type controlledReleases struct {
	fetch func(string) (*signing.Check, error)
	sign  func(*signing.Check) (bool, error)
	// signKeyPath — путь файла ключа, который окно передало при подписи
	// (у Android это keystore, у остальных — общий ключ экосистемы).
	signKeyPath *string
}

func (*controlledReleases) Releases(context.Context) ([]signing.Release, error) { return nil, nil }
func (b *controlledReleases) ReleaseURL(tag string) string {
	return "https://github.com/test/releases/tag/" + tag
}
func (b *controlledReleases) FetchWithProgress(_ context.Context, tag string, report func(signing.Progress)) (*signing.Check, error) {
	report(signing.Progress{Tag: tag, Phase: "check"})
	return b.fetch(tag)
}
func (b *controlledReleases) SignAndPublishWithProgress(_ context.Context, c *signing.Check, keyPath, _ string, report func(signing.Progress)) (bool, string, error) {
	report(signing.Progress{Tag: c.Tag, Phase: "sign"})
	if b.signKeyPath != nil {
		*b.signKeyPath = keyPath
	}
	ok, err := b.sign(c)
	return ok, "", err
}

// Ключ Android подписывает keystore из настроек, а не общий ключ экосистемы.
func TestSignReleasePicksAndroidKeyStore(t *testing.T) {
	t.Setenv("WYND_SIGNER_SETTINGS", filepath.Join(t.TempDir(), "signer.json"))
	settingsPath := os.Getenv("WYND_SIGNER_SETTINGS")
	key := filepath.Join(t.TempDir(), "workshop-signing.key")
	store := filepath.Join(t.TempDir(), "wynd-android.p12")
	if err := os.WriteFile(settingsPath, []byte(fmt.Sprintf(`{"key_path":%q,"key_store_path":%q}`, key, store)), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &App{ctx: context.Background(), emit: func(signing.Progress) {}}
	var seen string
	a.gh = &controlledReleases{
		fetch: func(tag string) (*signing.Check, error) {
			return &signing.Check{Tag: tag, Product: signing.ProductAndroid}, nil
		},
		signKeyPath: &seen,
		sign:        func(*signing.Check) (bool, error) { return true, nil },
	}
	c, err := a.CheckRelease("android-v0.1.0", "req-android")
	if err != nil {
		t.Fatal(err)
	}
	if res := (a.SignRelease(c.CheckID, "android-v0.1.0", "req-android", "pass")); !res.OK {
		t.Fatalf("sign result: %+v", res)
	}
	if seen != store {
		t.Fatalf("android sign must use the keystore, got %q", seen)
	}

	// Общий ключ остаётся у exe-программ.
	a.gh = &controlledReleases{
		fetch: func(tag string) (*signing.Check, error) {
			return &signing.Check{Tag: tag, Product: signing.ProductWorkshop}, nil
		},
		signKeyPath: &seen,
		sign:        func(*signing.Check) (bool, error) { return true, nil },
	}
	c, err = a.CheckRelease("workshop-v0.4.6", "req-workshop")
	if err != nil {
		t.Fatal(err)
	}
	if res := (a.SignRelease(c.CheckID, "workshop-v0.4.6", "req-workshop", "pass")); !res.OK {
		t.Fatalf("sign result: %+v", res)
	}
	if seen != key {
		t.Fatalf("workshop sign must use the ecosystem key, got %q", seen)
	}
}

func TestReleaseChecksCannotReplaceEachOtherOutOfOrder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		releaseA := make(chan struct{})
		enteredA := make(chan struct{})
		bDone := false
		a := &App{ctx: context.Background(), emit: func(signing.Progress) {}}
		a.gh = &controlledReleases{fetch: func(tag string) (*signing.Check, error) {
			if tag == "A" {
				close(enteredA)
				<-releaseA
			}
			return &signing.Check{Tag: tag}, nil
		}}
		go func() { _, _ = a.CheckRelease("A", "check-A") }()
		<-enteredA
		go func() { _, _ = a.CheckRelease("B", "check-B"); bDone = true }()
		synctest.Wait()
		if bDone {
			t.Error("second fetch replaced the resource while first fetch was running")
		}
		close(releaseA)
		synctest.Wait()
		if a.check == nil || a.check.Tag != "B" {
			t.Errorf("stale fetch won: %+v", a.check)
		}
	})
}

func TestReleaseSigningOwnsCheckUntilItFinishes(t *testing.T) {
	t.Setenv("WYND_SIGNER_SETTINGS", filepath.Join(t.TempDir(), "no-owner-settings"))
	synctest.Test(t, func(t *testing.T) {
		releaseSign := make(chan struct{})
		enteredSign := make(chan struct{})
		bDone := false
		a := &App{ctx: context.Background(), emit: func(signing.Progress) {}}
		a.gh = &controlledReleases{
			fetch: func(tag string) (*signing.Check, error) { return &signing.Check{Tag: tag}, nil },
			sign: func(c *signing.Check) (bool, error) {
				close(enteredSign)
				<-releaseSign
				if a.check != c {
					t.Error("in-use Check was replaced during signature")
				}
				return true, nil
			},
		}
		checked, _ := a.CheckRelease("A", "check-A")
		go func() { _ = a.SignRelease(checked.CheckID, checked.Tag, "sign-A", "test-only-password") }()
		<-enteredSign
		go func() { _, _ = a.CheckRelease("B", "check-B"); bDone = true }()
		synctest.Wait()
		if bDone {
			t.Error("fetch ran while another operation owned the signing resource")
		}
		close(releaseSign)
		synctest.Wait()
		if a.check == nil || a.check.Tag != "B" {
			t.Errorf("signature cleared another operation's Check: %+v", a.check)
		}
	})
}

func TestReleaseFailedCheckInvalidatesPreviousSuccess(t *testing.T) {
	a := &App{ctx: t.Context(), emit: func(signing.Progress) {}}
	a.gh = &controlledReleases{fetch: func(tag string) (*signing.Check, error) {
		if tag == "B" {
			return nil, errors.New("fetch failed")
		}
		return &signing.Check{Tag: tag}, nil
	}}
	_, _ = a.CheckRelease("A", "check-A")
	if _, err := a.CheckRelease("B", "check-B"); err == nil {
		t.Fatal("fixture did not fail")
	}
	if a.check != nil {
		t.Fatal("failed new check left a previously successful resource signable")
	}
}

func TestReleaseIdentityRejectsOldCheckEvenForSameTag(t *testing.T) {
	t.Setenv("WYND_SIGNER_SETTINGS", filepath.Join(t.TempDir(), "no-owner-settings"))
	calls := 0
	a := &App{ctx: t.Context(), emit: func(signing.Progress) {}}
	a.gh = &controlledReleases{
		fetch: func(tag string) (*signing.Check, error) { return &signing.Check{Tag: tag}, nil },
		sign:  func(*signing.Check) (bool, error) { calls++; return true, nil },
	}
	old, _ := a.CheckRelease("same-tag", "request-1")
	current, _ := a.CheckRelease("same-tag", "request-2")
	if old.CheckID == "" || current.CheckID == "" || old.CheckID == current.CheckID {
		t.Fatal("check identity was reused")
	}
	for _, input := range [][2]string{{old.CheckID, old.Tag}, {current.CheckID, "wrong-tag"}} {
		if res := a.SignRelease(input[0], input[1], "sign-stale", "test-only-password"); res.OK {
			t.Fatal("stale/mismatched check signed")
		}
		if calls != 0 || a.check != current {
			t.Fatal("rejected request consumed the current check")
		}
	}
	if res := a.SignRelease(current.CheckID, current.Tag, "sign-current", "test-only-password"); !res.OK || calls != 1 || a.check != nil {
		t.Fatalf("current check not consumed correctly: %+v", res)
	}
}

func TestReleaseSignRetriesHaveDistinctProgressIdentities(t *testing.T) {
	t.Setenv("WYND_SIGNER_SETTINGS", filepath.Join(t.TempDir(), "no-owner-settings"))
	var progress []signing.Progress
	attempts := 0
	a := &App{ctx: t.Context(), emit: func(p signing.Progress) { progress = append(progress, p) }}
	a.gh = &controlledReleases{
		fetch: func(tag string) (*signing.Check, error) { return &signing.Check{Tag: tag}, nil },
		sign: func(*signing.Check) (bool, error) {
			attempts++
			if attempts == 1 {
				return false, signing.ErrWrongPassword
			}
			return true, nil
		},
	}
	c, _ := a.CheckRelease("same-tag", "check-request")
	if res := a.SignRelease(c.CheckID, c.Tag, "sign-first", "test-only-password"); !res.WrongPassword || a.check != c {
		t.Fatalf("retry resource lost: %+v", res)
	}
	if res := a.SignRelease(c.CheckID, c.Tag, "sign-second", "test-only-password"); !res.OK {
		t.Fatalf("retry failed: %+v", res)
	}
	if len(progress) != 3 {
		t.Fatalf("progress=%+v", progress)
	}
	for i, want := range []string{"check-request", "sign-first", "sign-second"} {
		if progress[i].RequestID != want || progress[i].CheckID != c.CheckID {
			t.Fatalf("progress identity: %+v", progress[i])
		}
	}
}

func TestReleaseShutdownCancelsFetchBeforeClosingResources(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		entered := make(chan struct{})
		a := &App{emit: func(signing.Progress) {}}
		if err := a.ServiceStartup(context.Background(), application.ServiceOptions{}); err != nil {
			t.Fatal(err)
		}
		a.gh = &controlledReleases{fetch: func(string) (*signing.Check, error) { close(entered); <-a.ctx.Done(); return nil, a.ctx.Err() }}
		go func() { _, _ = a.CheckRelease("tag", "request") }()
		<-entered
		_ = a.ServiceShutdown()
		if _, err := a.CheckRelease("tag", "after-shutdown"); !errors.Is(err, context.Canceled) {
			t.Fatalf("shutdown did not cancel queued work: %v", err)
		}
		if a.check != nil {
			t.Fatal("resource survived shutdown")
		}
	})
}
