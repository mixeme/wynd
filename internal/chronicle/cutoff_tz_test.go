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
