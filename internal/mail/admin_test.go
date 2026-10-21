package mail_test

import (
	"context"
	"database/sql"
	"net"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/mail"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

func adminMailService(t *testing.T) (*mail.Service, chan string) {
	t.Helper()
	addr, received := startFakeSMTP(t)
	host, port, _ := net.SplitHostPort(addr)
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc, err := mail.New(st, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveConfig(context.Background(), mail.Config{Host: host, Port: atoi(port), From: "wynd@example.com"}); err != nil {
		t.Fatal(err)
	}
	return svc, received
}

// Без почты администратора писем нет: просьба видна только в панели.
func TestAdminLettersNeedAdminEmail(t *testing.T) {
	svc, received := adminMailService(t)
	ctx := context.Background()
	if err := svc.SendAdminPayRequest(ctx, "anya@example.com", "", "http://x/admin/pay/requests/1"); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-received:
		t.Fatalf("письмо без почты администратора: %q", msg)
	case <-time.After(300 * time.Millisecond):
	}
	if err := svc.SetAdminEmail(ctx, "не адрес"); err == nil {
		t.Fatal("адрес без @ принят")
	}
}

func TestAdminQuotaAndPayLetters(t *testing.T) {
	svc, received := adminMailService(t)
	ctx := context.Background()
	if err := svc.SetAdminEmail(ctx, " admin@example.com "); err != nil {
		t.Fatal(err)
	}
	got, err := svc.AdminEmail(ctx)
	if err != nil || got != "admin@example.com" {
		t.Fatalf("AdminEmail = %q, %v", got, err)
	}

	const gb = int64(1) << 30
	before := sql.NullInt64{Int64: gb, Valid: true}
	if err := svc.SendAdminQuotaRequest(ctx, "anya@example.com", "Семья", 2*gb, before, false, "http://x/admin"); err != nil {
		t.Fatal(err)
	}
	msg := <-received
	if !strings.Contains(msg, "To: admin@example.com") {
		t.Fatalf("не тому: %q", msg)
	}
	if body := plainText(t, msg); !strings.Contains(body, "anya@example.com просит 2,0 ГБ вместо 1,0 ГБ для круга «Семья».") &&
		!strings.Contains(body, "anya@example.com просит 2 ГБ вместо 1 ГБ для круга «Семья».") {
		t.Fatalf("текст просьбы: %q", body)
	}

	if err := svc.SendAdminPayRequest(ctx, "bob@example.com", "перевёл 300 ₽", "http://x/admin/pay/requests/7"); err != nil {
		t.Fatal(err)
	}
	body := plainText(t, <-received)
	for _, want := range []string{"Заявка об оплате подписки от bob@example.com.", "«перевёл 300 ₽»", "http://x/admin/pay/requests/7"} {
		if !strings.Contains(body, want) {
			t.Fatalf("нет %q в %q", want, body)
		}
	}
}
