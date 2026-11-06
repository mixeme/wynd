package chronicle_test

import (
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func circleTextBytes(t *testing.T, e *testEnv, circleID string) int64 {
	t.Helper()
	var used int64
	if err := e.ch.DB().QueryRowContext(e.ctx, `SELECT text_bytes FROM circles WHERE id = ?`, circleID).Scan(&used); err != nil {
		t.Fatal(err)
	}
	return used
}

// R6: оценка записи до вставки совпадает с тем, что триггеры quota_text_*
// добавляют на самом деле, включая имена авторов и строку события.
func TestTextCostMatchesTriggers(t *testing.T) {
	e := newTestEnv(t)
	e.seedAccount("owner")
	e.seedAccount("member")
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	longName := strings.Repeat("Ярослава", 12)[:100]
	e.join(circle.ID, "member", longName, e.at(0))

	// Запись владельца: ровно PostTextCost.
	before := circleTextBytes(t, e, circle.ID)
	cost := chronicle.PostTextCost("Привет, мир!", "Аня", "2026-08-30", nil, chronicle.GenderNone)
	e.post(circle.ID, "owner", "Привет, мир!", "2026-08-30", e.at(1))
	if got := circleTextBytes(t, e, circle.ID) - before; got != cost {
		t.Fatalf("запись: прирост %d, оценка %d", got, cost)
	}

	// Комментарий участника с длинным не-ASCII именем.
	post := e.post(circle.ID, "owner", "ещё запись", "2026-08-30", e.at(1))
	before = circleTextBytes(t, e, circle.ID)
	cost = chronicle.CommentTextCost("Красота!", longName, post.ID, chronicle.GenderNone)
	if _, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "member", PostID: post.ID,
		Body: "Красота!", Now: e.at(2),
	}); err != nil {
		t.Fatal(err)
	}
	if got := circleTextBytes(t, e, circle.ID) - before; got != cost {
		t.Fatalf("комментарий: прирост %d, оценка %d", got, cost)
	}

	// Реакция.
	before = circleTextBytes(t, e, circle.ID)
	cost = chronicle.ReactionTextCost("heart", "Аня", post.ID, chronicle.GenderNone)
	if _, err := e.ch.SetReaction(e.ctx, chronicle.ReactionInput{
		CircleID: circle.ID, AccountID: "owner", PostID: post.ID,
		Emoji: "heart", Now: e.at(2),
	}); err != nil {
		t.Fatal(err)
	}
	if got := circleTextBytes(t, e, circle.ID) - before; got != cost {
		t.Fatalf("реакция: прирост %d, оценка %d", got, cost)
	}

	// Название дня.
	before = circleTextBytes(t, e, circle.ID)
	cost = chronicle.DayTitleTextCost("Дача", "Аня", "2026-08-30", chronicle.GenderNone)
	if err := e.ch.SetDayTitle(e.ctx, chronicle.DayTitleInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-30",
		Title: "Дача", Now: e.at(2),
	}); err != nil {
		t.Fatal(err)
	}
	if got := circleTextBytes(t, e, circle.ID) - before; got != cost {
		t.Fatalf("название дня: прирост %d, оценка %d", got, cost)
	}

	// Правка не больше оценки: до чтения прежнего текста берётся полная строка.
	before = circleTextBytes(t, e, circle.ID)
	limit := chronicle.PostEditTextCost("Привет, мир! Исправлено", "Аня", "2026-08-30", chronicle.GenderNone)
	if err := e.ch.EditPost(e.ctx, circle.ID, "owner", post.ID, "Привет, мир! Исправлено", "2026-08-30", e.at(3)); err != nil {
		t.Fatal(err)
	}
	if got := circleTextBytes(t, e, circle.ID) - before; got <= 0 || got > limit {
		t.Fatalf("правка: прирост %d, верхняя оценка %d", got, limit)
	}
}

// R4: до вставки считается весь текст архива, а после оживления триггеры
// дают ровно эту сумму плюс служебную строку.
func TestReviveTextBytesMatchesInsert(t *testing.T) {
	e := newTestEnv(t)
	e.seedAccount("owner")
	e.seedAccount("bob")
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.join(circle.ID, "bob", "Боб", e.at(0))
	post := e.post(circle.ID, "owner", "на даче", "2026-08-01", e.at(1))
	if _, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "bob", PostID: post.ID, Body: "красота", Now: e.at(2),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ch.SetReaction(e.ctx, chronicle.ReactionInput{
		CircleID: circle.ID, AccountID: "bob", PostID: post.ID, Emoji: "heart", Now: e.at(2),
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.SetDayTitle(e.ctx, chronicle.DayTitleInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-01",
		Title: "Дача", Now: e.at(2),
	}); err != nil {
		t.Fatal(err)
	}
	data, err := e.ch.FullArchiveData(e.ctx, circle.ID)
	if err != nil {
		t.Fatal(err)
	}
	tables, accounts, blobs := archiveOverJSON(t, data)

	reviverName, reviverGender, ok := chronicle.ReviveReviver(tables, accounts, "owner@test.local")
	if !ok || reviverName != "Аня" {
		t.Fatalf("имя оживляющего: %q %v %v", reviverName, reviverGender, ok)
	}
	want := chronicle.ReviveTextBytes(tables) +
		chronicle.ReviveServiceEventCost(reviverName, reviverGender, "", "")

	res, err := e.ch.ReviveCircle(e.ctx, chronicle.ReviveInput{
		FormatVersion: 2, SchemaVersion: 2, Tables: tables, Accounts: accounts, BlobMap: blobs,
		ReviverID: "owner", Now: e.at(3),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.TextBytes != want {
		t.Fatalf("текст оживлённого круга %d, до вставки насчитали %d", res.TextBytes, want)
	}
}
