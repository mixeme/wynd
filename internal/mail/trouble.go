package mail

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/check"
)

// SendAdminServerTrouble — письмо администратору о поломках сервера (план 46,
// A10, ч. 2): все беды одним письмом, в теме — они же по срочности. Без
// почты администратора или без бед — ничего; sent говорит, ушло ли письмо.
func (s *Service) SendAdminServerTrouble(ctx context.Context, address, link string, troubles []check.Trouble, now time.Time) (sent bool, err error) {
	if len(troubles) == 0 {
		return false, nil
	}
	to, err := s.AdminEmail(ctx)
	if err != nil || to == "" {
		return false, err
	}
	subject, body := serverTroubleLetter(serverLabel(s.instanceName(ctx)), address, link, troubles, now)
	if err := s.notify(ctx, to, subject, body); err != nil {
		return false, err
	}
	return true, nil
}

func serverTroubleLetter(server, address, link string, troubles []check.Trouble, now time.Time) (subject, body string) {
	names := make([]string, 0, len(troubles))
	var b strings.Builder
	switch {
	case server != "" && address != "":
		fmt.Fprintf(&b, "Сервер «%s» — %s\n\n", server, address)
	case server != "":
		fmt.Fprintf(&b, "Сервер «%s»\n\n", server)
	case address != "":
		fmt.Fprintf(&b, "Сервер %s\n\n", address)
	}
	for _, t := range troubles {
		name, line := troubleText(t, now)
		names = append(names, name)
		b.WriteString("• " + line + "\n")
	}
	fmt.Fprintf(&b, "\nПодробнее — в «Проверке»:\n%s\n\nПока что-то не так, письмо приходит раз в день.\n", link)
	subject = strings.Join(names, ", ")
	if r := []rune(subject); len(r) > 0 {
		subject = strings.ToUpper(string(r[0])) + string(r[1:])
	}
	return subject, b.String()
}

// troubleText — беда коротко для темы и строкой для письма.
func troubleText(t check.Trouble, now time.Time) (name, line string) {
	switch t.Kind {
	case check.TroubleCertExpired:
		return "сертификат HTTPS истёк",
			fmt.Sprintf("Сертификат HTTPS истёк %s — браузеры не открывают сайт.", letterDate(t.At, now))
	case check.TroubleServerFull:
		return "место кончилось",
			fmt.Sprintf("Занято %s из %s — сервер упёрся в предел. Новое в круги не добавить.", formatBytes(t.Used), formatBytes(t.Total))
	case check.TroubleDiskLow:
		return "мало места на диске",
			fmt.Sprintf("На диске свободно %s из %s.", formatBytes(t.Used), formatBytes(t.Total))
	case check.TroubleCertNotRenewed:
		return "сертификат HTTPS не продлился",
			fmt.Sprintf("Сертификат HTTPS истекает %s и до сих пор не продлился.", letterDate(t.At, now))
	case check.TroubleServerNearlyFull:
		return "место почти кончилось",
			fmt.Sprintf("Занято %s из %s — больше 90 %% предела сервера.", formatBytes(t.Used), formatBytes(t.Total))
	case check.TroubleBackupMissing:
		return "нет свежей резервной копии", "Резервную копию ещё не делали."
	case check.TroubleBackupOld:
		return "нет свежей резервной копии",
			fmt.Sprintf("Резервной копии нет с %s.", letterDate(t.At, now))
	case check.TroubleRoutineStale:
		return "суточная рутина не запускается",
			fmt.Sprintf("Суточная рутина не запускалась с %s.", letterDate(t.At, now))
	}
	return "", ""
}

var monthsGenitive = []string{
	"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря",
}

// letterDate — «3 ноября»; год — только если не текущий.
func letterDate(t, now time.Time) string {
	d := t.UTC()
	s := fmt.Sprintf("%d %s", d.Day(), monthsGenitive[d.Month()-1])
	if d.Year() != now.UTC().Year() {
		s += fmt.Sprintf(" %d", d.Year())
	}
	return s
}
