package mail

import (
	"context"
	"strings"
)

// SendEmailChangedNotice уходит на прежний адрес: почту для входа сменили.
// Без ссылок и кнопок, и нового адреса не называет — прежний ящик мог
// достаться другому.
func (s *Service) SendEmailChangedNotice(ctx context.Context, oldEmail string, byAdmin bool) error {
	oldEmail = strings.TrimSpace(oldEmail)
	if oldEmail == "" {
		return ErrInvalid
	}
	var b strings.Builder
	b.WriteString("Почта для входа" + s.onServer(ctx) + " изменена: по этому адресу войти больше нельзя.\n\n")
	if byAdmin {
		b.WriteString("Почту сменил администратор сервера.\n\n")
	}
	b.WriteString("Круги, имена и записи остались прежними.\n\n")
	b.WriteString("Если почту меняли не вы и не по вашей просьбе — напишите администратору сервера.\n")
	return s.notify(ctx, oldEmail, "Wynd: почта для входа изменена", b.String())
}

// SendEmailAssignedNotice уходит на новый адрес, когда почту сменил
// администратор: кода не было, и человек узнаёт, куда теперь придёт код.
func (s *Service) SendEmailAssignedNotice(ctx context.Context, newEmail string) error {
	newEmail = strings.TrimSpace(newEmail)
	if newEmail == "" {
		return ErrInvalid
	}
	body := "Администратор сервера назначил этот адрес почтой для входа" + s.onServer(ctx) + ".\n\n" +
		"Откройте Wynd и войдите с этим адресом — код придёт сюда. Круги, имена и записи остались прежними.\n\n" +
		"Если вы об этом не просили — напишите администратору сервера.\n"
	return s.notify(ctx, newEmail, "Wynd: новая почта для входа", body)
}

// onServer — « на сервере «Имя»» или пусто, если сервер назван как продукт.
func (s *Service) onServer(ctx context.Context) string {
	if name := s.instanceName(ctx); name != "" {
		return " на сервере «" + name + "»"
	}
	return ""
}
