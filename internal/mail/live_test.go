//go:build integration

package mail_test

import (
	"context"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/mail"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

func TestLiveSMTPProbe(t *testing.T) {
	for _, fx := range loadLiveSMTPFixtures(t) {
		t.Run(fx.Name, func(t *testing.T) {
			st, err := store.OpenMemory()
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()

			svc, err := mail.New(st, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()

			if err := svc.Probe(ctx, fx.Config); err != nil {
				t.Fatalf("probe: %v", err)
			}
		})
	}
}

func TestLiveSMTPSendTest(t *testing.T) {
	for _, fx := range loadLiveSMTPFixtures(t) {
		t.Run(fx.Name, func(t *testing.T) {
			st, err := store.OpenMemory()
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()

			svc, err := mail.New(st, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()

			if err := svc.SaveConfig(ctx, fx.Config); err != nil {
				t.Fatal(err)
			}
			if err := svc.SendTest(ctx, fx.SendTo); err != nil {
				t.Fatalf("send test to %q: %v", fx.SendTo, err)
			}
			cfg, err := svc.LoadConfig(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.TestSentAt == nil {
				t.Fatal("want smtp_test_sent_at after SendTest")
			}
			if cfg.LastError != "" {
				t.Fatalf("last error after success: %q", cfg.LastError)
			}
		})
	}
}

func TestLiveSMTPAuthCode(t *testing.T) {
	for _, fx := range loadLiveSMTPFixtures(t) {
		t.Run(fx.Name, func(t *testing.T) {
			st, err := store.OpenMemory()
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()

			svc, err := mail.New(st, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()

			if err := svc.SaveConfig(ctx, fx.Config); err != nil {
				t.Fatal(err)
			}
			if err := svc.SendCode(ctx, fx.SendTo, "123456"); err != nil {
				t.Fatalf("send code to %q: %v", fx.SendTo, err)
			}
		})
	}
}
