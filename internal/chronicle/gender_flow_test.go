package chronicle_test

import (
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Инвариант (план 46, A5): род выбирают при создании круга и при вступлении,
// меняют в «Кто вы в этом круге»; новые строки идут по выбору, старые не
// переписываются.
func TestGenderFlowsIntoNewSummaries(t *testing.T) {
	e := newTestEnv(t)
	circle, _, _, err := e.ch.CreateCircle(e.ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня",
		OwnerGender: chronicle.GenderFemale, Now: e.at(0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.ch.Join(e.ctx, chronicle.JoinInput{
		CircleID: circle.ID, AccountID: "bob", Name: "Боб", Gender: chronicle.GenderMale, Now: e.at(1),
	}); err != nil {
		t.Fatal(err)
	}
	e.post(circle.ID, "owner", "первая", "2026-08-01", e.at(2))

	male := chronicle.GenderMale
	if err := e.ch.UpdateIdentity(e.ctx, circle.ID, "owner", chronicle.UpdateIdentityInput{Gender: &male}, e.at(3)); err != nil {
		t.Fatal(err)
	}
	e.post(circle.ID, "owner", "вторая", "2026-08-01", e.at(4))

	rows, err := e.ch.DB().QueryContext(e.ctx, `SELECT summary FROM events WHERE circle_id = ? ORDER BY seq`, circle.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		got = append(got, s)
	}
	want := []string{"Создан круг «Семья»", "Боб вступил в круг", "Аня опубликовала запись", "Аня опубликовал запись"}
	if len(got) != len(want) {
		t.Fatalf("events: %q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event %d: %q, want %q (all: %q)", i, got[i], want[i], got)
		}
	}
}
