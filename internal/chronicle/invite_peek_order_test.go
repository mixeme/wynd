package chronicle_test

import (
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Инвариант (wynd.html, «Вступление»): ряд 1.3 — владелец, пригласивший,
// дальше самые активные за 90 дней, при равенстве раньше вступивший.
// Сказанное старше 90 дней в счёт не идёт.
func TestInvitePeekOrdersByRecentActivity(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.join(circle.ID, "oldie", "Дед", e.at(1))
	for range 3 {
		e.post(circle.ID, "oldie", "давно", "2026-08-02", e.at(2)) // старше 90 дней к now
	}
	e.join(circle.ID, "silent", "Тётя", e.at(3))
	e.join(circle.ID, "newbie", "Боря", e.at(150))
	e.post(circle.ID, "newbie", "привет", "2026-12-30", e.at(160))

	peek, err := e.ch.InvitePeekForCircle(e.ctx, circle.ID, "silent", e.at(200))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range peek.Members {
		got = append(got, m.Name)
	}
	want := []string{"Аня", "Тётя", "Боря", "Дед"}
	if len(got) != len(want) {
		t.Fatalf("members: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order: got %v, want %v", got, want)
		}
	}
}
