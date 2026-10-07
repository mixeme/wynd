package chronicle_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Инвариант: список дней показывает только дни с видимыми записями. Счёт
// идёт по отрезкам видимости, поэтому день, целиком состоящий из записей до
// вступления, в списке не появляется (REF-6: счёт переписан на групповой
// запрос, правило прежнее).
func TestDaysSnapshotCountsOnlyVisiblePosts(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())

	e.post(circle.ID, "owner", "до новичка", "2026-08-01", e.at(0))
	e.join(circle.ID, "newbie", "Боб", e.at(2))
	e.post(circle.ID, "owner", "при новичке", "2026-08-03", e.at(3))
	e.post(circle.ID, "owner", "и ещё одна", "2026-08-03", e.at(3).Add(time.Hour))

	owner, err := e.ch.DaysSnapshot(e.ctx, circle.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if len(owner) != 2 {
		t.Fatalf("у владельца дней: %d, ожидалось 2", len(owner))
	}

	newbie, err := e.ch.DaysSnapshot(e.ctx, circle.ID, "newbie")
	if err != nil {
		t.Fatal(err)
	}
	if len(newbie) != 1 {
		t.Fatalf("у новичка дней: %d — день до вступления не его", len(newbie))
	}
	if newbie[0].Day.EntryDate != "2026-08-03" {
		t.Fatalf("день новичка: %q", newbie[0].Day.EntryDate)
	}
	if newbie[0].PostCount != 2 {
		t.Fatalf("записей в дне у новичка: %d, ожидалось 2", newbie[0].PostCount)
	}
	for _, d := range owner {
		if d.Day.EntryDate == "2026-08-01" && d.PostCount != 1 {
			t.Fatalf("записей в дне до новичка: %d", d.PostCount)
		}
	}
}

// Инвариант: название и обложка дня — сказанное в свой момент. Новичок видит
// день по своей записи, но название и обложку, данные до его вступления, —
// нет; владелец видит всё. Раньше день приходил с чужим прошлым названием.
func TestDaysSnapshotHidesTitleAndCoverBeforeJoin(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())

	early := e.post(circle.ID, "owner", "до новичка", "2026-08-01", e.after(0))
	e.seedBlob("blob-1", "owner")
	e.attachPhoto(early.ID, "blob-1")
	if err := e.ch.SetDayTitle(e.ctx, chronicle.DayTitleInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-01",
		Title: "Прошлое", Now: e.after(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.SetDayCover(e.ctx, chronicle.DayCoverInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-01",
		PostID: early.ID, BlobID: "blob-1", Now: e.after(2 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	e.join(circle.ID, "newbie", "Боб", e.after(time.Hour))
	e.post(circle.ID, "newbie", "я пришёл", "2026-08-01", e.after(time.Hour+time.Minute))

	newbie, err := e.ch.DaysSnapshot(e.ctx, circle.ID, "newbie")
	if err != nil {
		t.Fatal(err)
	}
	if len(newbie) != 1 || newbie[0].PostCount != 1 {
		t.Fatalf("новичок видит день по своей записи: %+v", newbie)
	}
	d := newbie[0].Day
	if d.Title != "" || d.CoverPostID != "" || d.CoverBlobID != "" {
		t.Fatalf("новичку ушло сказанное до него: title=%q cover=%q/%q", d.Title, d.CoverPostID, d.CoverBlobID)
	}

	owner, err := e.ch.DaysSnapshot(e.ctx, circle.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if len(owner) != 1 || owner[0].Day.Title != "Прошлое" || owner[0].Day.CoverBlobID != "blob-1" {
		t.Fatalf("владелец видит название и обложку: %+v", owner)
	}
}
