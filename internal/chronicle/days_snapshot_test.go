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

// Инвариант: сроки правки названия и обложки дня приходят вместе со списком
// дней — одним запросом на весь круг, а не по два на каждый день.
func TestDaysSnapshotCarriesEditableUntil(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.DurationWindow(time.Hour))

	post := e.post(circle.ID, "owner", "запись", "2026-08-01", e.at(0))
	e.seedBlob("blob-1", "owner")
	e.attachPhoto(post.ID, "blob-1")

	if err := e.ch.SetDayTitle(e.ctx, chronicle.DayTitleInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-01",
		Title: "Название", Now: e.at(0),
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.SetDayCover(e.ctx, chronicle.DayCoverInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-01",
		PostID: post.ID, BlobID: "blob-1", Now: e.at(0),
	}); err != nil {
		t.Fatal(err)
	}

	days, err := e.ch.DaysSnapshot(e.ctx, circle.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 1 {
		t.Fatalf("дней: %d", len(days))
	}
	if days[0].TitleEditableUntil == nil || days[0].CoverEditableUntil == nil {
		t.Fatalf("сроки правки не пришли: %+v", days[0])
	}
	if !days[0].TitleEditableUntil.After(e.at(0)) {
		t.Fatalf("срок правки названия в прошлом: %v", days[0].TitleEditableUntil)
	}

	// День без сказанного сроков не несёт.
	e.post(circle.ID, "owner", "другой день", "2026-08-02", e.at(1))
	days, err = e.ch.DaysSnapshot(e.ctx, circle.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range days {
		if d.Day.EntryDate != "2026-08-02" {
			continue
		}
		if d.TitleEditableUntil != nil || d.CoverEditableUntil != nil {
			t.Fatalf("у дня без названия и обложки есть сроки: %+v", d)
		}
	}
}
