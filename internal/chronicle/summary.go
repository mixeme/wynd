package chronicle

import (
	"fmt"
	"time"
)

func summaryCircleCreated(name string) string {
	return fmt.Sprintf("Создан круг «%s»", name)
}

func summaryCircleRenamed(name string) string {
	return fmt.Sprintf("Круг переименован в «%s»", name)
}

func summaryIdentityRenamed(from, to string) string {
	return fmt.Sprintf("%s теперь %s", from, to)
}

func summaryEditWindowChanged(label string) string {
	return fmt.Sprintf("Окно правок изменено: %s", label)
}

func summaryAvatarSet(name string) string {
	return fmt.Sprintf("Новое фото: %s", name)
}

func summaryAvatarCleared(name string) string {
	return fmt.Sprintf("Фото убрано: %s", name)
}

// Строки с глаголом — по роду участника (план 46, A5): «Аня вступила в
// круг». Род не выбран — строка без глагола: «В круге: Аня».

func summaryMemberJoined(g Gender, name string) string {
	if g == GenderNone {
		return fmt.Sprintf("В круге: %s", name)
	}
	return fmt.Sprintf("%s %s в круг", name, past(g, "вступил", "вступила"))
}

func summaryMemberLeft(g Gender, name string) string {
	if g == GenderNone {
		return fmt.Sprintf("Больше не в круге: %s", name)
	}
	return fmt.Sprintf("%s %s круг", name, past(g, "покинул", "покинула"))
}

func summaryOwnerTransferred(from, to string) string {
	return fmt.Sprintf("Владелец передан: %s → %s", from, to)
}

// g — род того, кто дал или забрал право (владельца).
func summarySettingsGranted(g Gender, actor, target string) string {
	if g == GenderNone {
		return fmt.Sprintf("%s: право менять настройки — %s", actor, target)
	}
	return fmt.Sprintf("%s %s право менять настройки: %s", actor, past(g, "дал", "дала"), target)
}

func summarySettingsRevoked(g Gender, actor, target string) string {
	if g == GenderNone {
		return fmt.Sprintf("%s: без права менять настройки — %s", actor, target)
	}
	return fmt.Sprintf("%s %s право менять настройки: %s", actor, past(g, "забрал", "забрала"), target)
}

func summaryPostCreated(g Gender, name string) string {
	if g == GenderNone {
		return fmt.Sprintf("Новая запись: %s", name)
	}
	return fmt.Sprintf("%s %s запись", name, past(g, "опубликовал", "опубликовала"))
}

func summaryPostEdited(g Gender, name string) string {
	if g == GenderNone {
		return fmt.Sprintf("Запись изменена: %s", name)
	}
	return fmt.Sprintf("%s %s запись", name, past(g, "отредактировал", "отредактировала"))
}

func summaryCommentCreated(g Gender, name string) string {
	if g == GenderNone {
		return fmt.Sprintf("Комментарий: %s", name)
	}
	return fmt.Sprintf("%s %s комментарий", name, past(g, "оставил", "оставила"))
}

func summaryCommentEdited(g Gender, name string) string {
	if g == GenderNone {
		return fmt.Sprintf("Комментарий изменён: %s", name)
	}
	return fmt.Sprintf("%s %s комментарий", name, past(g, "отредактировал", "отредактировала"))
}

func summaryReactionSet(g Gender, name, emoji string) string {
	if g == GenderNone {
		return fmt.Sprintf("Реакция %s: %s", emoji, name)
	}
	return fmt.Sprintf("%s %s реакцию %s", name, past(g, "поставил", "поставила"), emoji)
}

// Название и обложка дня идут в ленту строкой, как служебные события
// (3.1), поэтому в строке — какой это день: «выбрал обложку дня» без даты
// в ленте не понять.
func summaryDayTitled(g Gender, name, title, entryDate string) string {
	if g == GenderNone {
		return fmt.Sprintf("Название для %s — «%s»: %s", dayLabel(entryDate), title, name)
	}
	return fmt.Sprintf("%s %s %s «%s»", name, past(g, "назвал", "назвала"), dayLabel(entryDate), title)
}

func summaryDayCoverSet(g Gender, name, entryDate string) string {
	if g == GenderNone {
		return fmt.Sprintf("Обложка для %s: %s", dayLabel(entryDate), name)
	}
	return fmt.Sprintf("%s %s обложку для %s", name, past(g, "выбрал", "выбрала"), dayLabel(entryDate))
}

func summaryDayTitleCleared(g Gender, name, entryDate string) string {
	if g == GenderNone {
		return fmt.Sprintf("Название для %s убрано: %s", dayLabel(entryDate), name)
	}
	return fmt.Sprintf("%s %s название для %s", name, past(g, "убрал", "убрала"), dayLabel(entryDate))
}

func summaryDayCoverCleared(g Gender, name, entryDate string) string {
	if g == GenderNone {
		return fmt.Sprintf("Обложка для %s убрана: %s", dayLabel(entryDate), name)
	}
	return fmt.Sprintf("%s %s обложку для %s", name, past(g, "убрал", "убрала"), dayLabel(entryDate))
}

// captionDayCard — подпись открытки дня (3.15): что сделали. Даты и названия
// в ней нет — они в самой открытке.
func captionDayCard(g Gender, name string, titled, cover bool) string {
	if g == GenderNone {
		switch {
		case titled && cover:
			return fmt.Sprintf("Название и обложка дня: %s", name)
		case titled:
			return fmt.Sprintf("Название дня: %s", name)
		default:
			return fmt.Sprintf("Обложка дня: %s", name)
		}
	}
	named := past(g, "назвал", "назвала") + " день"
	chose := past(g, "выбрал", "выбрала") + " обложку"
	switch {
	case titled && cover:
		return fmt.Sprintf("%s %s и %s", name, named, chose)
	case titled:
		return fmt.Sprintf("%s %s", name, named)
	default:
		return fmt.Sprintf("%s %s", name, chose)
	}
}

var monthsGenitive =[...]string{"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря"}

// dayLabel: "2026-08-06" → "6 августа"; нераспознанное — как есть.
func dayLabel(entryDate string) string {
	t, err := time.Parse("2006-01-02", entryDate)
	if err != nil {
		return entryDate
	}
	return fmt.Sprintf("%d %s", t.Day(), monthsGenitive[t.Month()-1])
}

func summaryCutoffSet(cutoff string) string {
	return fmt.Sprintf("Отсечка архивации: %s", cutoff)
}

func summaryCutoffMoved(from, to string) string {
	return fmt.Sprintf("Отсечка перенесена: %s → %s", from, to)
}

func summaryDeadlineSet(label string) string {
	return fmt.Sprintf("Срок скачивания архива: %s", label)
}

func summaryDeadlineMoved(from, to string) string {
	return fmt.Sprintf("Срок скачивания перенесён: %s → %s", from, to)
}

func formatDeadlineLabel(t time.Time) string {
	return t.UTC().Format("02.01.2006 15:04")
}
