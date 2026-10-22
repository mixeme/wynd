package mail_test

import (
	"context"
	"mime"
	netmail "net/mail"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/check"
)

// Письмо о поломках (A10, ч. 2): тема — беды по срочности без «Wynd:»,
// в тексте — строка на беду и ссылка на «Проверку».
func TestAdminServerTroubleLetter(t *testing.T) {
	svc, received := adminMailService(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 22, 6, 0, 0, 0, time.UTC)
	troubles := []check.Trouble{
		{Kind: check.TroubleDiskLow, Used: 800 << 20, Total: 50 << 30},
		{Kind: check.TroubleCertNotRenewed, At: time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)},
		{Kind: check.TroubleServerNearlyFull, Used: 37 << 30, Total: 40 << 30},
		{Kind: check.TroubleBackupOld, At: time.Date(2026, 10, 10, 3, 0, 0, 0, time.UTC)},
		{Kind: check.TroubleRoutineStale, At: time.Date(2025, 12, 20, 3, 0, 0, 0, time.UTC)},
	}

	// Без почты администратора — ничего.
	sent, err := svc.SendAdminServerTrouble(ctx, "wynd.example.ru", "https://wynd.example.ru/admin/check", troubles, now)
	if err != nil || sent {
		t.Fatalf("без почты администратора: sent %v, %v", sent, err)
	}
	if err := svc.SetAdminEmail(ctx, "admin@example.com"); err != nil {
		t.Fatal(err)
	}
	// Без бед — тоже ничего.
	if sent, err := svc.SendAdminServerTrouble(ctx, "wynd.example.ru", "https://wynd.example.ru/admin/check", nil, now); err != nil || sent {
		t.Fatalf("без бед: sent %v, %v", sent, err)
	}
	select {
	case msg := <-received:
		t.Fatalf("лишнее письмо: %q", msg)
	case <-time.After(300 * time.Millisecond):
	}

	sent, err = svc.SendAdminServerTrouble(ctx, "wynd.example.ru", "https://wynd.example.ru/admin/check", troubles, now)
	if err != nil || !sent {
		t.Fatalf("sent %v, %v", sent, err)
	}
	msg := <-received
	m, err := netmail.ReadMessage(strings.NewReader(msg))
	if err != nil {
		t.Fatal(err)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
	if err != nil {
		t.Fatal(err)
	}
	wantSubject := "Мало места на диске, сертификат HTTPS не продлился, место почти кончилось, нет свежей резервной копии, суточная рутина не запускается"
	if subject != wantSubject {
		t.Fatalf("тема %q", subject)
	}
	body := plainText(t, msg)
	for _, want := range []string{
		"• На диске свободно 800 МБ из 50 ГБ.",
		"• Сертификат HTTPS истекает 3 ноября и до сих пор не продлился.",
		"• Занято 37 ГБ из 40 ГБ — больше 90 % предела сервера.",
		"• Резервной копии нет с 10 октября.",
		"• Суточная рутина не запускалась с 20 декабря 2025.",
		"Подробнее — в «Проверке»:",
		"https://wynd.example.ru/admin/check",
		"Пока что-то не так, письмо приходит раз в день.",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("нет %q в %q", want, body)
		}
	}
}

func TestAdminServerTroubleWorstLines(t *testing.T) {
	svc, received := adminMailService(t)
	ctx := context.Background()
	if err := svc.SetAdminEmail(ctx, "admin@example.com"); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 22, 6, 0, 0, 0, time.UTC)
	troubles := []check.Trouble{
		{Kind: check.TroubleCertExpired, At: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)},
		{Kind: check.TroubleServerFull, Used: 40 << 30, Total: 40 << 30},
		{Kind: check.TroubleBackupMissing},
	}
	if _, err := svc.SendAdminServerTrouble(ctx, "wynd.example.ru", "https://wynd.example.ru/admin/check", troubles, now); err != nil {
		t.Fatal(err)
	}
	msg := <-received
	m, _ := netmail.ReadMessage(strings.NewReader(msg))
	subject, _ := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
	if subject != "Сертификат HTTPS истёк, место кончилось, нет свежей резервной копии" {
		t.Fatalf("тема %q", subject)
	}
	body := plainText(t, msg)
	for _, want := range []string{
		"• Сертификат HTTPS истёк 20 октября — браузеры не открывают сайт.",
		"• Занято 40 ГБ из 40 ГБ — сервер упёрся в предел. Новое в круги не добавить.",
		"• Резервную копию ещё не делали.",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("нет %q в %q", want, body)
		}
	}
}
