package chronicle

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

// Стоимость текста — те же формулы, что у триггеров quota_text_* (C42):
// служебная часть строки + байты текста и имён. Оценка делается до записи и
// должна совпадать с фактическим учётом, иначе квота пропускает лишнее (R6).
// Проверяется контрактными тестами против триггеров.

// WriterText — имя и род участника, которыми подпишутся строка и событие.
func (c *Chronicle) WriterText(ctx context.Context, circleID, accountID string) (string, Gender, error) {
	mem, err := c.MembershipForAccount(ctx, circleID, accountID)
	if err != nil {
		return "", GenderNone, err
	}
	name, err := c.ResolveIdentityName(ctx, mem.IdentityID)
	if err != nil {
		return "", GenderNone, err
	}
	return name, c.IdentityGender(ctx, mem.IdentityID), nil
}

// EventTextBytes — служебная часть строки журнала (триггер quota_text_event).
const EventTextBytes int64 = 400

// eventRowCost — строка журнала: служебная часть, JSON-данные, строка ленты и
// имя участника.
func eventRowCost(payload any, summary, actorName string) int64 {
	return EventTextBytes + jsonBytes(payload) + int64(len(summary)) + int64(len(actorName))
}

// jsonBytes — байты JSON-представления данных события: та же запись, что
// encoding/json кладёт в events.payload. Пустые данные appendEvent пишет как
// «{}», а не как пустую строку.
func jsonBytes(v any) int64 {
	if v == nil {
		return 2
	}
	b, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	return int64(len(b))
}

func textBytes(s string) int64 {
	return int64(len(s))
}

// PostTextCost — новая запись и её событие: строка posts, поисковый индекс
// (текст считается трижды) и строка журнала с именем автора.
func PostTextCost(body, authorName, entryDate string, capturedAt *time.Time, g Gender) int64 {
	body = strings.TrimSpace(body)
	payload := map[string]any{
		"body":        body,
		"entry_date":  entryDate,
		"captured_at": capturedAtPayload(capturedAt),
	}
	return 550 + 3*textBytes(body) + textBytes(authorName) +
		eventRowCost(payload, summaryPostCreated(g, authorName), authorName)
}

// PostEditTextCost — правка записи: событие и прирост строки. Для проверки до
// чтения прежнего текста берётся полная строка: это не меньше фактического
// прироста.
func PostEditTextCost(body, authorName, entryDate string, g Gender) int64 {
	body = strings.TrimSpace(body)
	payload := map[string]any{"body": body, "entry_date": entryDate}
	return 550 + 3*textBytes(body) + textBytes(authorName) +
		eventRowCost(payload, summaryPostEdited(g, authorName), authorName)
}

// CommentPostID — запись, к которой относится комментарий: нужна для точной
// оценки события правки до записи.
func (c *Chronicle) CommentPostID(ctx context.Context, commentID string) (string, error) {
	var postID string
	err := c.db.QueryRowContext(ctx, `SELECT post_id FROM comments WHERE id = ?`, commentID).Scan(&postID)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return postID, err
}

// CommentTextCost — новый комментарий и его событие.
func CommentTextCost(body, authorName, postID string, g Gender) int64 {
	body = strings.TrimSpace(body)
	payload := map[string]any{"post_id": postID, "body": body}
	return 560 + 3*textBytes(body) + textBytes(authorName) +
		eventRowCost(payload, summaryCommentCreated(g, authorName), authorName)
}

// CommentEditTextCost — правка комментария: событие и полная строка (верхняя
// оценка прироста).
func CommentEditTextCost(body, authorName, postID string, g Gender) int64 {
	body = strings.TrimSpace(body)
	payload := map[string]any{"post_id": postID, "body": body}
	return 560 + 3*textBytes(body) + textBytes(authorName) +
		eventRowCost(payload, summaryCommentEdited(g, authorName), authorName)
}

// ReactionTextCost — новая реакция и её событие.
func ReactionTextCost(emoji, authorName, postID string, g Gender) int64 {
	payload := map[string]any{"post_id": postID, "emoji": emoji}
	return 450 + textBytes(emoji) + textBytes(authorName) +
		eventRowCost(payload, summaryReactionSet(g, authorName, emoji), authorName)
}

// DayTitleTextCost — название дня: строка day_titles и событие.
func DayTitleTextCost(title, authorName, entryDate string, g Gender) int64 {
	title = strings.TrimSpace(title)
	payload := map[string]any{"entry_date": entryDate, "title": title}
	return 250 + 3*textBytes(title) +
		eventRowCost(payload, summaryDayTitled(g, authorName, title, entryDate), authorName)
}

// DayTitleClearTextCost — снятие названия: событие и пустая строка day_titles.
func DayTitleClearTextCost(authorName, entryDate string, g Gender) int64 {
	payload := map[string]any{"entry_date": entryDate}
	return 250 + eventRowCost(payload, summaryDayTitleCleared(g, authorName, entryDate), authorName)
}

// DayCoverTextCost — выбор обложки дня: только событие.
func DayCoverTextCost(authorName, entryDate, postID, blobID string, g Gender) int64 {
	payload := map[string]any{"entry_date": entryDate, "post_id": postID, "blob_id": blobID}
	return eventRowCost(payload, summaryDayCoverSet(g, authorName, entryDate), authorName)
}

// DayCoverClearTextCost — снятие обложки: только событие.
func DayCoverClearTextCost(authorName, entryDate string, g Gender) int64 {
	payload := map[string]any{"entry_date": entryDate}
	return eventRowCost(payload, summaryDayCoverCleared(g, authorName, entryDate), authorName)
}

// ReviveReviver — имя и род того, кто оживит круг, по строкам архива: его
// почта находит учётку архива, участие и имя. false — в архиве его нет
// (оживляет новый человек).
func ReviveReviver(tables map[string]FullArchiveTable, accounts map[string]string, reviverEmail string) (string, Gender, bool) {
	if reviverEmail == "" {
		return "", GenderNone, false
	}
	oldAccount := ""
	for id, email := range accounts {
		if strings.EqualFold(email, reviverEmail) {
			oldAccount = id
			break
		}
	}
	if oldAccount == "" {
		return "", GenderNone, false
	}
	str := func(row map[string]any, col string) string {
		s, _ := row[col].(string)
		return s
	}
	identityID := ""
	if t, ok := tables["memberships"]; ok {
		for i := range t.Rows {
			row := rowMap(t, i)
			if str(row, "account_id") == oldAccount {
				identityID = str(row, "identity_id")
				if str(row, "status") == "active" {
					break
				}
			}
		}
	}
	if identityID == "" {
		return "", GenderNone, false
	}
	g := GenderNone
	if t, ok := tables["identities"]; ok {
		for i := range t.Rows {
			row := rowMap(t, i)
			if str(row, "id") == identityID {
				g, _ = ParseGender(str(row, "gender"))
				break
			}
		}
	}
	name := ""
	var nameAt string
	if t, ok := tables["identity_names"]; ok {
		for i := range t.Rows {
			row := rowMap(t, i)
			if str(row, "identity_id") != identityID || str(row, "erased_at") != "" {
				continue
			}
			if at := str(row, "effective_at"); at >= nameAt {
				name, nameAt = str(row, "name"), at
			}
		}
	}
	if name == "" {
		return "", GenderNone, false
	}
	return name, g, true
}

// ReviveServiceEventCost — служебная строка «круг оживлён»: та же запись,
// которую ReviveCircle добавит после строк архива.
func ReviveServiceEventCost(name string, g Gender, movedFrom, splitFrom string) int64 {
	var payload any
	summary := summaryCircleRevived(g, name)
	if movedFrom != "" {
		summary = summaryCircleMovedIn(movedFrom)
	}
	if splitFrom != "" {
		summary = summaryCircleSplit(splitFrom)
		payload = map[string]string{"from": splitFrom}
	}
	return eventRowCost(payload, summary, name)
}

// ReviveTextBytes — сколько текста триггеры добавят за строки архива до
// вставки (R4). Считаются все таблицы круга, которые несут текст и события;
// строки удалённых записей тоже вставятся и дадут свою служебную часть.
func ReviveTextBytes(tables map[string]FullArchiveTable) int64 {
	var total int64
	str := func(row map[string]any, col string) string {
		s, _ := row[col].(string)
		return s
	}
	eachRow := func(table string, cost func(map[string]any) int64) {
		t, ok := tables[table]
		if !ok {
			return
		}
		for i := range t.Rows {
			total += cost(rowMap(t, i))
		}
	}
	eachRow("posts", func(row map[string]any) int64 {
		return 550 + 3*textBytes(str(row, "body")) + textBytes(str(row, "author_name"))
	})
	eachRow("comments", func(row map[string]any) int64 {
		return 560 + 3*textBytes(str(row, "body")) + textBytes(str(row, "author_name"))
	})
	eachRow("reactions", func(row map[string]any) int64 {
		return 450 + textBytes(str(row, "emoji")) + textBytes(str(row, "author_name"))
	})
	eachRow("events", func(row map[string]any) int64 {
		return EventTextBytes + textBytes(str(row, "payload")) +
			textBytes(str(row, "summary")) + textBytes(str(row, "actor_name"))
	})
	eachRow("day_titles", func(row map[string]any) int64 {
		return 250 + 3*textBytes(str(row, "title"))
	})
	return total
}
