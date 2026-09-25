package mail

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// SendSubscriptionReminder notifies a participant before subscription expiry.
func (s *Service) SendSubscriptionReminder(ctx context.Context, email, instanceName string, expiresAt time.Time, daysLeft int) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return ErrInvalid
	}
	subject := "Wynd: подписка скоро закончится"
	body := fmt.Sprintf(
		"Подписка на «%s» действует до %s — через %d %s круги на этом сервере закроются.\n\nОткройте приложение и продлите доступ.\n",
		instanceName,
		expiresAt.UTC().Format("02.01.2006"),
		daysLeft,
		pluralDays(daysLeft),
	)
	return s.notify(ctx, email, subject, body)
}

func pluralDays(n int) string {
	mod10 := n % 10
	mod100 := n % 100
	if mod10 == 1 && mod100 != 11 {
		return "день"
	}
	if mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20) {
		return "дня"
	}
	return "дней"
}
