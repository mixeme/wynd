package chronicle_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func feedDayCards(t *testing.T, e *testEnv, circleID, accountID string) []chronicle.FeedEventSummary {
	t.Helper()
	meta, err := e.ch.FeedMetaForAccount(e.ctx, circleID, accountID)
	if err != nil {
		t.Fatal(err)
	}
	var out []chronicle.FeedEventSummary
	for _, ev := range meta.Events {
		if ev.Day != nil {
			out = append(out, ev)
		}
	}
	return out
}

// Назвали день и тут же выбрали обложку — в ленте одна открытка (3.15).
func TestFeedDayCardMergesTitleAndCover(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(circle.ID, "owner", "плёнки", "2026-08-06", e.after(time.Minute))
	e.seedBlob("blob-p1", "owner")
	e.attachPhoto(p.ID, "blob-p1")
	if err := e.ch.SetDayTitle(e.ctx, chronicle.DayTitleInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-06", Title: "Плёнки", Now: e.after(2 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.SetDayCover(e.ctx, chronicle.DayCoverInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-06",
		PostID: p.ID, BlobID: "blob-p1", Now: e.after(3 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	cards := feedDayCards(t, e, circle.ID, "owner")
	if len(cards) != 1 {
		t.Fatalf("открыток %d, нужна одна: %+v", len(cards), cards)
	}
	d := cards[0].Day
	if d.EntryDate != "2026-08-06" || d.Title != "Плёнки" || d.CoverBlobID != "blob-p1" {
		t.Fatalf("открытка = %+v", d)
	}
	if d.Caption != "Название и обложка дня: Аня" {
		t.Fatalf("подпись = %q", d.Caption)
	}
}

// Переименовали и сменили обложку несколько раз подряд — одна открытка с
// последним; подпись называет то, что меняли.
func TestFeedDayCardMergesRunOfChanges(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(circle.ID, "owner", "плёнки", "2026-08-06", e.after(time.Minute))
	e.seedBlob("blob-p1", "owner")
	e.seedBlob("blob-p2", "owner")
	e.attachPhoto(p.ID, "blob-p1")
	e.attachPhoto(p.ID, "blob-p2")
	title := func(text string, when time.Time) {
		t.Helper()
		if err := e.ch.SetDayTitle(e.ctx, chronicle.DayTitleInput{
			CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-06", Title: text, Now: when,
		}); err != nil {
			t.Fatal(err)
		}
	}
	cover := func(blob string, when time.Time) {
		t.Helper()
		if err := e.ch.SetDayCover(e.ctx, chronicle.DayCoverInput{
			CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-06",
			PostID: p.ID, BlobID: blob, Now: when,
		}); err != nil {
			t.Fatal(err)
		}
	}
	start := e.at(1)
	title("Плёнки", start)
	title("Чердак", start.Add(4*time.Minute))
	title("Чердак и плёнки", start.Add(8*time.Minute))
	cards := feedDayCards(t, e, circle.ID, "owner")
	if len(cards) != 1 || cards[0].Day.Title != "Чердак и плёнки" || cards[0].Day.Caption != "Название дня: Аня" {
		t.Fatalf("три названия подряд: %+v", cards)
	}

	// Окно — между соседними, а не от первой: 16 минут от начала ещё склеиваются.
	cover("blob-p1", start.Add(12*time.Minute))
	cover("blob-p2", start.Add(16*time.Minute))
	cards = feedDayCards(t, e, circle.ID, "owner")
	if len(cards) != 1 {
		t.Fatalf("открыток %d, нужна одна: %+v", len(cards), cards)
	}
	d := cards[0].Day
	if d.Title != "Чердак и плёнки" || d.CoverBlobID != "blob-p2" || d.Caption != "Название и обложка дня: Аня" {
		t.Fatalf("открытка = %+v", d)
	}

	// Перерыв дольше окна — новая открытка.
	title("Август", start.Add(40*time.Minute))
	if cards = feedDayCards(t, e, circle.ID, "owner"); len(cards) != 2 {
		t.Fatalf("после перерыва открыток %d, нужно две", len(cards))
	}
}

// Сменили одно — в открытке оба (3.16); старая открытка остаётся прежней.
func TestFeedDayCardShowsWholeDay(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.join(circle.ID, "kot", "Кот", e.after(time.Minute))
	p := e.post(circle.ID, "owner", "плёнки", "2026-08-06", e.after(2*time.Minute))
	e.post(circle.ID, "kot", "и я там был", "2026-08-06", e.after(3*time.Minute))
	e.seedBlob("blob-p1", "owner")
	e.seedBlob("blob-p2", "owner")
	e.attachPhoto(p.ID, "blob-p1")
	e.attachPhoto(p.ID, "blob-p2")
	title := func(account, text string, when time.Time) {
		t.Helper()
		if err := e.ch.SetDayTitle(e.ctx, chronicle.DayTitleInput{
			CircleID: circle.ID, AccountID: account, EntryDate: "2026-08-06", Title: text, Now: when,
		}); err != nil {
			t.Fatal(err)
		}
	}
	cover := func(account, blob string, when time.Time) {
		t.Helper()
		if err := e.ch.SetDayCover(e.ctx, chronicle.DayCoverInput{
			CircleID: circle.ID, AccountID: account, EntryDate: "2026-08-06",
			PostID: p.ID, BlobID: blob, Now: when,
		}); err != nil {
			t.Fatal(err)
		}
	}
	title("owner", "Плёнки", e.at(1))
	// Позже окна склейки — своя открытка.
	cover("owner", "blob-p1", e.at(2))
	// Другой человек — тоже своя, хоть и сразу.
	cover("kot", "blob-p2", e.at(2).Add(time.Minute))
	title("owner", "Чердак", e.at(3))

	cards := feedDayCards(t, e, circle.ID, "owner")
	if len(cards) != 4 {
		t.Fatalf("открыток %d, нужно четыре: %+v", len(cards), cards)
	}
	// От новых к старым.
	want := []struct{ title, cover string }{
		{"Чердак", "blob-p2"},
		{"Плёнки", "blob-p2"},
		{"Плёнки", "blob-p1"},
		{"Плёнки", ""},
	}
	for i, w := range want {
		d := cards[i].Day
		if d.Title != w.title || d.CoverBlobID != w.cover {
			t.Fatalf("открытка %d = %+v, ждали %+v", i, d, w)
		}
	}

	// Убрали название: строка без открытки, а новая обложка — уже без имени.
	if err := e.ch.ClearDayTitle(e.ctx, circle.ID, "owner", "2026-08-06", e.at(4)); err != nil {
		t.Fatal(err)
	}
	cover("owner", "blob-p1", e.at(5))
	cards = feedDayCards(t, e, circle.ID, "owner")
	if len(cards) != 5 || cards[0].Day.Title != "" || cards[0].Day.CoverBlobID != "blob-p1" {
		t.Fatalf("после снятия названия: %+v", cards[0].Day)
	}
}

// Название, данное до вступления, новичок в открытке не видит — как в «Днях».
func TestFeedDayCardHidesTitleFromBeforeJoin(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.post(circle.ID, "owner", "до", "2026-08-06", e.after(time.Minute))
	if err := e.ch.SetDayTitle(e.ctx, chronicle.DayTitleInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-06", Title: "Плёнки", Now: e.at(1),
	}); err != nil {
		t.Fatal(err)
	}
	e.join(circle.ID, "kot", "Кот", e.at(2))
	p := e.post(circle.ID, "owner", "после", "2026-08-06", e.at(3))
	e.seedBlob("blob-p1", "owner")
	e.attachPhoto(p.ID, "blob-p1")
	if err := e.ch.SetDayCover(e.ctx, chronicle.DayCoverInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-06",
		PostID: p.ID, BlobID: "blob-p1", Now: e.at(4),
	}); err != nil {
		t.Fatal(err)
	}
	cards := feedDayCards(t, e, circle.ID, "kot")
	if len(cards) != 1 || cards[0].Day.Title != "" || cards[0].Day.CoverBlobID != "blob-p1" {
		t.Fatalf("новичок: %+v", cards)
	}
}
