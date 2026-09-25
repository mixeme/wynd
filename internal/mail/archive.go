package mail

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// SendArchiveCycleStart notifies a member that an archive cycle has started.
func (s *Service) SendArchiveCycleStart(ctx context.Context, email, cutoffDate string, deadline time.Time, downloadURL string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return ErrInvalid
	}
	subject := "Wynd: началась архивация круга"
	body := fmt.Sprintf(
		"В круге начался цикл архивации.\n\nОтсечка: %s\nСкачать до: %s\n\nСкачать персональный архив:\n%s\n",
		cutoffDate,
		deadline.UTC().Format("02.01.2006 15:04"),
		downloadURL,
	)
	return s.notify(ctx, email, subject, body)
}

// SendArchiveReminder reminds a member before the archive deadline.
func (s *Service) SendArchiveReminder(ctx context.Context, email, cutoffDate string, deadline time.Time, downloadURL string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return ErrInvalid
	}
	subject := "Wynd: напоминание об архиве круга"
	body := fmt.Sprintf(
		"Скоро истечёт срок скачивания архива.\n\nОтсечка: %s\nСкачать до: %s\n\nСкачать персональный архив:\n%s\n",
		cutoffDate,
		deadline.UTC().Format("02.01.2006 15:04"),
		downloadURL,
	)
	return s.notify(ctx, email, subject, body)
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
