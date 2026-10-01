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

func summaryMemberJoined(name string) string {
	return fmt.Sprintf("%s вступил в круг", name)
}

func summaryMemberLeft(name string) string {
	return fmt.Sprintf("%s покинул круг", name)
}

func summaryOwnerTransferred(from, to string) string {
	return fmt.Sprintf("Владелец передан: %s → %s", from, to)
}

func summarySettingsGranted(actor, target string) string {
	return fmt.Sprintf("%s дал право менять настройки: %s", actor, target)
}

func summarySettingsRevoked(actor, target string) string {
	return fmt.Sprintf("%s забрал право менять настройки: %s", actor, target)
}

func summaryPostCreated(name string) string {
	return fmt.Sprintf("%s опубликовал запись", name)
}

func summaryPostEdited(name string) string {
	return fmt.Sprintf("%s отредактировал запись", name)
}

func summaryCommentCreated(name string) string {
	return fmt.Sprintf("%s оставил комментарий", name)
}

func summaryCommentEdited(name string) string {
	return fmt.Sprintf("%s отредактировал комментарий", name)
}

func summaryReactionSet(name, emoji string) string {
	return fmt.Sprintf("%s поставил реакцию %s", name, emoji)
}

// Название и обложка дня идут в ленту строкой, как служебные события
// (3.1), поэтому в строке — какой это день: «выбрал обложку дня» без даты
// в ленте не понять.
func summaryDayTitled(name, title, entryDate string) string {
	return fmt.Sprintf("%s назвал %s «%s»", name, dayLabel(entryDate), title)
}

func summaryDayCoverSet(name, entryDate string) string {
	return fmt.Sprintf("%s выбрал обложку для %s", name, dayLabel(entryDate))
}

var monthsGenitive = [...]string{"января", "февраля", "марта", "апреля", "мая", "июня",
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
