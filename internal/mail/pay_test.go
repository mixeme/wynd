package mail_test

import (
	"context"
	"mime"
	netmail "net/mail"
	"strings"
	"testing"
	"time"
)

func waitPayLetter(t *testing.T, received chan string) string {
	t.Helper()
	select {
	case msg := <-received:
		return msg
	case <-time.After(2 * time.Second):
		t.Fatal("письмо не ушло")
		return ""
	}
}

func paySubject(t *testing.T, raw string) string {
	t.Helper()
	m, err := netmail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("письмо не разбирается: %v", err)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
	if err != nil {
		t.Fatalf("тема не разбирается: %v", err)
	}
	return subject
}

// Утверждённая заявка: письмо с новым сроком; бессрочная — без даты.
func TestPayRequestApprovedLetter(t *testing.T) {
	svc, received := adminMailService(t)
	ctx := context.Background()
	expires := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	if err := svc.SendPayRequestApproved(ctx, "ana@example.com", "Дом Ани", expires, false); err != nil {
		t.Fatal(err)
	}
	msg := waitPayLetter(t, received)
	if !strings.Contains(msg, "To: ana@example.com") {
		t.Fatalf("не тому: %q", msg)
	}
	if subject := paySubject(t, msg); subject != "Заявка об оплате утверждена" {
		t.Fatalf("тема: %q", subject)
	}
	if body := plainText(t, msg); !strings.Contains(body, "Подписка на «Дом Ани» продлена до 30.09.2026.") {
		t.Fatalf("нет срока: %q", body)
	}
	if err := svc.SendPayRequestApproved(ctx, "ana@example.com", "Дом Ани", expires, true); err != nil {
		t.Fatal(err)
	}
	body := plainText(t, waitPayLetter(t, received))
	if strings.Contains(body, "продлена") || !strings.Contains(body, "теперь бессрочная") {
		t.Fatalf("бессрочное продление: %q", body)
	}
}

// Отклонённая заявка: срок не продлён.
func TestPayRequestRejectedLetter(t *testing.T) {
	svc, received := adminMailService(t)
	if err := svc.SendPayRequestRejected(context.Background(), "ana@example.com", "Дом Ани"); err != nil {
		t.Fatal(err)
	}
	msg := waitPayLetter(t, received)
	if subject := paySubject(t, msg); subject != "Заявка об оплате отклонена" {
		t.Fatalf("тема: %q", subject)
	}
	if body := plainText(t, msg); !strings.Contains(body, "срок не продлён") {
		t.Fatalf("текст: %q", body)
	}
}

// Выдача без заявки: то же продление, без упоминания заявки.
func TestPayGrantedLetter(t *testing.T) {
	svc, received := adminMailService(t)
	expires := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	if err := svc.SendPayGranted(context.Background(), "ana@example.com", "Дом Ани", expires, false); err != nil {
		t.Fatal(err)
	}
	msg := waitPayLetter(t, received)
	if subject := paySubject(t, msg); subject != "Подписка продлена" {
		t.Fatalf("тема: %q", subject)
	}
	body := plainText(t, msg)
	if !strings.Contains(body, "продлена до 31.10.2026") {
		t.Fatalf("нет срока: %q", body)
	}
	if strings.Contains(body, "заявк") {
		t.Fatalf("выдача без заявки пишет о заявке: %q", body)
	}
	if err := svc.SendPayGranted(context.Background(), "ana@example.com", "Дом Ани", expires, true); err != nil {
		t.Fatal(err)
	}
	if body := plainText(t, waitPayLetter(t, received)); !strings.Contains(body, "теперь бессрочная") {
		t.Fatalf("бессрочная выдача: %q", body)
	}
}
