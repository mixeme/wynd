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

func summaryMemberJoined(name string) string {
	return fmt.Sprintf("%s вступил в круг", name)
}

func summaryMemberLeft(name string) string {
	return fmt.Sprintf("%s покинул круг", name)
}

func summaryOwnerTransferred(from, to string) string {
	return fmt.Sprintf("Владелец передан: %s → %s", from, to)
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

func summaryDayTitled(name, title string) string {
	return fmt.Sprintf("%s назвал день «%s»", name, title)
}

func summaryDayCoverSet(name string) string {
	return fmt.Sprintf("%s выбрал обложку дня", name)
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
