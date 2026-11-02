package mail_test

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/mail"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

// Почта в теле нужна для узнавания человека на новом сервере, а не для уведомлений.
func TestCircleNotificationLoginEmail(t *testing.T) {
	const recipient = "anya@example.org"
	for _, tc := range []struct {
		name       string
		needsEmail bool
		send       func(context.Context, *mail.Service) error
	}{
		{"archive ready", false, func(ctx context.Context, s *mail.Service) error {
			return s.SendFullArchiveReady(ctx, recipient, "Семья", time.Date(2026, 11, 5, 11, 2, 0, 0, time.UTC), "https://old.example/archive")
		}},
		{"leaving", false, func(ctx context.Context, s *mail.Service) error {
			return s.SendCircleLeaving(ctx, recipient, "Семья", "Мышь", "https://old.example/circle")
		}},
		{"waiting", true, func(ctx context.Context, s *mail.Service) error {
			return s.SendCircleWaiting(ctx, recipient, "Семья", "Мышь", "https://new.example/join")
		}},
		{"waiting without owner", true, func(ctx context.Context, s *mail.Service) error {
			return s.SendCircleWaiting(ctx, recipient, "Семья", "", "https://new.example/join")
		}},
		{"moving", false, func(ctx context.Context, s *mail.Service) error {
			return s.SendCircleMoving(ctx, recipient, "Семья", "Мышь", "https://old.example/circle")
		}},
		{"moved", true, func(ctx context.Context, s *mail.Service) error {
			return s.SendCircleMoved(ctx, recipient, "Семья", "https://new.example")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addr, received := startFakeSMTP(t)
			host, portText, err := net.SplitHostPort(addr)
			if err != nil {
				t.Fatal(err)
			}
			port, err := strconv.Atoi(portText)
			if err != nil {
				t.Fatal(err)
			}
			st, err := store.OpenMemory()
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			s, err := mail.New(st, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if err := s.SaveConfig(ctx, mail.Config{Host: host, Port: port, From: "wynd@example.org"}); err != nil {
				t.Fatal(err)
			}
			if err := tc.send(ctx, s); err != nil {
				t.Fatal(err)
			}
			select {
			case raw := <-received:
				body := plainText(t, raw)
				wantCount := 0
				if tc.needsEmail {
					wantCount = 1
				}
				if got := strings.Count(body, recipient); got != wantCount {
					t.Fatalf("login email count = %d, want %d: %s", got, wantCount, body)
				}
				if tc.needsEmail && !strings.Contains(body, "Войдите с почтой "+recipient+" — попадёте в своё имя и свои записи.") {
					t.Fatalf("missing identity explanation: %s", body)
				}
				if strings.Contains(body, "писать пока нельзя") || strings.Contains(raw, "mailto:") {
					t.Fatalf("misleading writing promise or email action: %s", body)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("local SMTP did not receive message")
			}
		})
	}
}
