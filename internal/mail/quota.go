package mail

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"
)

// SendQuotaAnswer — письмо владельцу круга об ответе администратора на
// просьбу о месте (решение владельца 2026-10-13; кадры 6.44, 6.45).
func (s *Service) SendQuotaAnswer(ctx context.Context, email, circle string, requested int64, before sql.NullInt64, approved bool, link string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return ErrInvalid
	}
	asked := formatBytes(requested)
	if before.Valid {
		asked += " вместо " + formatBytes(before.Int64)
	}
	var subject, body string
	if approved {
		subject = "Wynd: кругу дали место"
		body = fmt.Sprintf("Администратор дал кругу «%s» %s.\n\n%s\n", circle, asked, link)
	} else {
		subject = "Wynd: в месте отказали"
		body = fmt.Sprintf("Администратор отказал кругу «%s» в %s. Можно попросить ещё раз или освободить место:\n%s\n", circle, formatBytes(requested), link)
	}
	return s.notify(ctx, email, subject, body)
}

// formatBytes — как formatBytes в клиенте (web/src/lib/format/bytes.ts):
// «20 ГБ», «1,5 ГБ», «512 МБ».
func formatBytes(n int64) string {
	units := []string{"Б", "КБ", "МБ", "ГБ", "ТБ"}
	if n <= 0 {
		return "0 Б"
	}
	i := int(math.Floor(math.Log(float64(n)) / math.Log(1024)))
	if i >= len(units) {
		i = len(units) - 1
	}
	v := float64(n) / math.Pow(1024, float64(i))
	if v >= 10 || i == 0 {
		return fmt.Sprintf("%d %s", int64(math.Round(v)), units[i])
	}
	return strings.Replace(fmt.Sprintf("%.1f %s", v, units[i]), ".", ",", 1)
}

// SendFullArchiveReady — письмо владельцу: полный архив круга собран и ждёт
// скачивания до срока (6.40, 6.33).
func (s *Service) SendFullArchiveReady(ctx context.Context, email, circle string, expires time.Time, link string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return ErrInvalid
	}
	subject := "Wynd: архив круга готов"
	body := fmt.Sprintf("Архив круга «%s» собран. Скачать его можно до %s — потом сервер удалит файл.\n\n%s\n",
		circle, expires.UTC().Format("02.01.2006 15:04")+" UTC", link)
	return s.notify(ctx, email, subject, body)
}

// SendCircleLeaving — единственное письмо участникам, когда круг замер под
// полный архив с удалением (6.36): что происходит и что можно успеть.
func (s *Service) SendCircleLeaving(ctx context.Context, email, circle, owner, link string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return ErrInvalid
	}
	who := "Владелец"
	if strings.TrimSpace(owner) != "" {
		who = owner
	}
	subject := "Wynd: круг «" + circle + "» уходит с сервера"
	body := fmt.Sprintf("%s убирает «%s» с сервера и сохраняет круг в файл. Читать можно, писать нельзя.\n\nСвой архив — всё, что вы видели в круге, — можно скачать, пока круг здесь:\n%s\n", who, circle, link)
	return s.notify(ctx, email, subject, body)
}
