package mail

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// AdminEmail — куда писать администратору (2026-10-21). Пусто — писем нет.
func (s *Service) AdminEmail(ctx context.Context) (string, error) {
	var email string
	if err := s.db.QueryRowContext(ctx, `SELECT admin_email FROM instance_settings WHERE id = 1`).Scan(&email); err != nil {
		return "", fmt.Errorf("mail: load admin email: %w", err)
	}
	return strings.TrimSpace(email), nil
}

// SetAdminEmail сохраняет почту администратора; пустая строка выключает письма.
func (s *Service) SetAdminEmail(ctx context.Context, email string) error {
	email = strings.TrimSpace(email)
	if email != "" && !strings.Contains(email, "@") {
		return ErrInvalid
	}
	_, err := s.db.ExecContext(ctx, `UPDATE instance_settings SET admin_email = ? WHERE id = 1`, email)
	return err
}

// SendAdminQuotaRequest — письмо администратору: владелец круга просит место
// (6.18; для круга, которого здесь ещё нет, — 7.31: перенос или оживление).
// Без почты администратора — ничего: просьба видна в панели.
func (s *Service) SendAdminQuotaRequest(ctx context.Context, requester, circle string, requested int64, before sql.NullInt64, arriving bool, link string) error {
	to, err := s.AdminEmail(ctx)
	if err != nil || to == "" {
		return err
	}
	asked := formatBytes(requested)
	if before.Valid {
		asked += " вместо " + formatBytes(before.Int64)
	}
	what := fmt.Sprintf("для круга «%s»", circle)
	if arriving {
		what = fmt.Sprintf("для круга «%s», который переносят или оживляют на этом сервере", circle)
	}
	subject := "Wynd: просят место для круга"
	body := fmt.Sprintf("%s просит %s %s.\n\nОтветить можно в панели:\n%s\n", requester, asked, what, link)
	return s.notify(ctx, to, subject, body)
}

// SendAdminPayRequest — письмо администратору: пришла заявка об оплате
// подписки (10.11). Без почты администратора — ничего: заявка видна в панели.
func (s *Service) SendAdminPayRequest(ctx context.Context, requester, comment, link string) error {
	to, err := s.AdminEmail(ctx)
	if err != nil || to == "" {
		return err
	}
	subject := "Wynd: заявка об оплате"
	body := fmt.Sprintf("Заявка об оплате подписки от %s.", requester)
	if c := strings.TrimSpace(comment); c != "" {
		body += fmt.Sprintf(" Комментарий: «%s».", c)
	}
	body += fmt.Sprintf("\n\nПосмотреть снимок экрана и ответить можно в панели:\n%s\n", link)
	return s.notify(ctx, to, subject, body)
}
