package mail

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// Письма архивации ведут на экран «Архив круга» в приложении, а не на
// /api/…/archive/download: браузер из почты ключа входа не несёт и получал
// голое {"error":"forbidden"} (владелец, 2026-10-23, снимок с телефона).
// Даты — словами, как на экранах: «30 сентября», без «Отсечки» и UTC.

// SendArchiveCycleStart notifies a member that an archive cycle has started.
func (s *Service) SendArchiveCycleStart(ctx context.Context, email, circle, cutoffDate string, deadline time.Time, link string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return ErrInvalid
	}
	now := time.Now()
	subject := fmt.Sprintf("Архивация круга «%s»", circle)
	body := fmt.Sprintf(
		"В круге «%s» всё, что написано до %s, уходит в архив. %s оно удалится с сервера.\n\nСкачайте свой архив до этого дня:\n%s\n",
		circle, cutoffWords(cutoffDate, now), letterDate(deadline, now), link,
	)
	return s.notify(ctx, email, subject, body)
}

// SendArchiveReminder reminds a member before the archive deadline.
func (s *Service) SendArchiveReminder(ctx context.Context, email, circle, cutoffDate string, deadline time.Time, link string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return ErrInvalid
	}
	now := time.Now()
	subject := fmt.Sprintf("Скоро удаление в круге «%s»", circle)
	body := fmt.Sprintf(
		"%s в круге «%s» удалится всё, что написано до %s.\n\nСкачайте свой архив до этого дня:\n%s\n",
		letterDate(deadline, now), circle, cutoffWords(cutoffDate, now), link,
	)
	return s.notify(ctx, email, subject, body)
}

// cutoffWords — «30 сентября» из «2026-09-30»; непонятную строку отдаёт как есть.
func cutoffWords(cutoffDate string, now time.Time) string {
	t, err := time.Parse("2006-01-02", cutoffDate)
	if err != nil {
		return cutoffDate
	}
	return letterDate(t, now)
}

func (s *Service) notify(ctx context.Context, email, subject, body string) error {
	cfg, err := s.LoadConfig(ctx)
	if err != nil {
		return err
	}
	if !configured(cfg) {
		if s.loopback.Load() {
			log.Printf("wynd mail (loopback): to=%s subject=%q\n%s", email, subject, body)
			return nil
		}
		return ErrNotConfigured
	}
	return s.sendMessage(ctx, cfg, email, subject, body)
}
