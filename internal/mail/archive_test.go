package mail_test

import (
	"context"
	"fmt"
	"mime"
	netmail "net/mail"
	"strings"
	"testing"
	"time"
)

// Письма архивации ведут на экран «Архив круга», а не на адрес API (из почты
// браузер шёл без ключа и получал {"error":"forbidden"}); даты — словами.
func TestArchiveLettersLinkToAppScreen(t *testing.T) {
	svc, received := adminMailService(t)
	ctx := context.Background()
	y := time.Now().Year()
	cutoff := fmt.Sprintf("%d-09-30", y)
	deadline := time.Date(y, 10, 25, 20, 59, 0, 0, time.UTC)
	link := "https://wynd.example.ru/circles/c1/archive"

	for _, c := range []struct {
		send    func() error
		subject string
		lines   []string
	}{
		{
			func() error {
				return svc.SendArchiveCycleStart(ctx, "anya@example.com", "Семья", cutoff, deadline, link)
			},
			"Архивация круга «Семья»",
			[]string{"В круге «Семья» всё, что написано до 30 сентября, уходит в архив. 25 октября оно удалится с сервера.", "Скачайте свой архив до этого дня:", link},
		},
		{
			func() error {
				return svc.SendArchiveReminder(ctx, "anya@example.com", "Семья", cutoff, deadline, link)
			},
			"Скоро удаление в круге «Семья»",
			[]string{"25 октября в круге «Семья» удалится всё, что написано до 30 сентября.", "Скачайте свой архив до этого дня:", link},
		},
	} {
		if err := c.send(); err != nil {
			t.Fatal(err)
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
		if subject != c.subject {
			t.Fatalf("тема %q", subject)
		}
		body := plainText(t, msg)
		for _, want := range c.lines {
			if !strings.Contains(body, want) {
				t.Fatalf("нет %q в %q", want, body)
			}
		}
		for _, bad := range []string{"/api/", "Отсечка", "персональный", "UTC"} {
			if strings.Contains(body, bad) {
				t.Fatalf("%q в письме: %q", bad, body)
			}
		}
	}
}
