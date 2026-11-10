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
	subject := "Подписка скоро закончится"
	body := fmt.Sprintf(
		"Подписка на «%s» действует до %s — через %d %s круги на этом сервере закроются.\n\nОткройте приложение и продлите доступ.\n",
		instanceName,
		expiresAt.UTC().Format("02.01.2006"),
		daysLeft,
		pluralDays(daysLeft),
	)
	return s.notify(ctx, email, subject, body)
}

// SendPayRequestApproved tells a participant their payment request was approved
// and the subscription term changed.
func (s *Service) SendPayRequestApproved(ctx context.Context, email, instanceName string, expiresAt time.Time, unlimited bool) error {
	return s.payExtended(ctx, email, instanceName, expiresAt, unlimited, "Заявка об оплате утверждена", "Заявка об оплате утверждена.")
}

// SendPayRequestRejected tells a participant their payment request was declined.
func (s *Service) SendPayRequestRejected(ctx context.Context, email, instanceName string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return ErrInvalid
	}
	subject := "Заявка об оплате отклонена"
	body := fmt.Sprintf(
		"Заявка об оплате подписки на «%s» отклонена; срок не продлён.\n\nЕсли это ошибка, свяжитесь с администратором.\n",
		instanceName,
	)
	return s.notify(ctx, email, subject, body)
}

// SendPayGranted tells a participant the admin extended their subscription without a request.
func (s *Service) SendPayGranted(ctx context.Context, email, instanceName string, expiresAt time.Time, unlimited bool) error {
	return s.payExtended(ctx, email, instanceName, expiresAt, unlimited, "Подписка продлена", "")
}

// payExtended — общий текст продления: у утверждённой заявки причина своя,
// у выдачи без заявки причины нет; срок и подпись одинаковые.
func (s *Service) payExtended(ctx context.Context, email, instanceName string, expiresAt time.Time, unlimited bool, subject, cause string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return ErrInvalid
	}
	var term string
	if unlimited {
		term = "Подписка на «" + instanceName + "» теперь бессрочная."
	} else {
		term = fmt.Sprintf("Подписка на «%s» продлена до %s.", instanceName, expiresAt.UTC().Format("02.01.2006"))
	}
	if cause != "" {
		term = cause + " " + term
	}
	body := term + "\n\nОткройте приложение — доступ уже работает.\n"
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
