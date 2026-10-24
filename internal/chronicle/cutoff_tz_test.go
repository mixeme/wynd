package chronicle_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// «Архивировать всё до 24 октября» — до полуночи 24-го по поясу устройства
// владельца (C59, владелец 2026-10-24: «По устройству»): запись, внесённая
// в 00:48 по Москве, — уже 24-е и остаётся; по UTC она 23-го и ушла бы.
func TestCutoffByOwnerTimeZone(t *testing.T) {
	e := newTestEnv(t)
	e.seedAccount("owner")
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	late := time.Date(2026, 10, 23, 21, 48, 0, 0, time.UTC) // 00:48 24-го по Москве
	e.post(circle.ID, "owner", "после полуночи", "2026-10-24", late)
	e.post(circle.ID, "owner", "днём раньше", "2026-10-23", late.Add(-6*time.Hour))

	for tz, want := range map[string]int{"Europe/Moscow": 1, "": 0} {
		kept, err := e.ch.PostsKeptAtCutoffTZ(e.ctx, circle.ID, "2026-10-24", tz)
		if err != nil || kept != want {
			t.Errorf("пояс %q: останется %d, ждали %d (%v)", tz, kept, want, err)
		}
	}

	// Удаление — по поясу, сохранённому с отсечкой.
	if err := e.ch.SetArchiveTimeZone(e.ctx, circle.ID, "Europe/Moscow"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ch.PurgeBeforeCutoff(e.ctx, circle.ID, "2026-10-24", late.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	feed, err := e.ch.FullArchiveFeed(e.ctx, circle.ID, late.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(feed) != 1 || feed[0].Post.Body != "после полуночи" {
		t.Fatalf("после удаления: %+v", feed)
	}
	// Незнакомый пояс не пишется.
	if chronicle.ValidTimeZone("Марс/Олимп") {
		t.Fatal("незнакомый пояс принят")
	}
}

// 6.17а (владелец 2026-10-24): строка — только если под отсечку попали
// записи, которых владелец не видел (внесённые раньше начала его отрезка).
func TestUnseenPostsAtCutoff(t *testing.T) {
	e := newTestEnv(t)
	e.seedAccount("owner")
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	day := func(d int) time.Time { return time.Date(2026, 10, d, 12, 0, 0, 0, time.UTC) }
	e.post(circle.ID, "owner", "двадцатое", "2026-10-20", day(20))
	e.post(circle.ID, "owner", "двадцать первое", "2026-10-21", day(21))
	e.post(circle.ID, "owner", "двадцать пятое", "2026-10-25", day(25))
	seen := day(24) // отрезок владельца — с 24-го
	for cutoff, want := range map[string]int{
		"2026-10-19": 0, // раньше первой записи — ничего
		"2026-10-21": 1, // уходит только невиданное 20-е
		"2026-10-23": 2,
		"2026-10-26": 2, // уходит и виденное 25-е, но невиданных — две
	} {
		n, err := e.ch.UnseenPostsAtCutoff(e.ctx, circle.ID, cutoff, "", seen)
		if err != nil || n != want {
			t.Errorf("до %s: невиданных %d, ждали %d (%v)", cutoff, n, want, err)
		}
	}
}
