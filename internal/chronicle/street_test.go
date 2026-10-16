package chronicle_test

import (
	"errors"
	"reflect"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func (e *testEnv) groups(accountID string) []chronicle.StreetGroup {
	e.t.Helper()
	g, err := e.ch.AccountGroups(e.ctx, accountID)
	if err != nil {
		e.t.Fatalf("AccountGroups: %v", err)
	}
	return g
}

// Группы учётки (A2): круг — не больше чем в одной; переименование в занятое
// название сливает группы; опустевшая группа уходит сама.
func TestStreetGroups(t *testing.T) {
	e := newTestEnv(t)
	a := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	b := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())

	if err := e.ch.PutCircleInGroup(e.ctx, a.ID, "owner", "  Родные ", e.at(1)); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.PutCircleInGroup(e.ctx, b.ID, "owner", "Друзья", e.at(2)); err != nil {
		t.Fatal(err)
	}
	want := []chronicle.StreetGroup{{Name: "Друзья", CircleIDs: []string{b.ID}}, {Name: "Родные", CircleIDs: []string{a.ID}}}
	if got := e.groups("owner"); !reflect.DeepEqual(got, want) {
		t.Fatalf("группы: %+v", got)
	}

	// Круг переезжает — прежняя группа, опустев, уходит.
	if err := e.ch.PutCircleInGroup(e.ctx, b.ID, "owner", "Родные", e.at(3)); err != nil {
		t.Fatal(err)
	}
	want = []chronicle.StreetGroup{{Name: "Родные", CircleIDs: []string{a.ID, b.ID}}}
	if got := e.groups("owner"); !reflect.DeepEqual(got, want) {
		t.Fatalf("после переезда: %+v", got)
	}

	if err := e.ch.RenameGroup(e.ctx, "owner", "Родные", "Семья"); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.RemoveCircleFromGroup(e.ctx, a.ID, "owner"); err != nil {
		t.Fatal(err)
	}
	want = []chronicle.StreetGroup{{Name: "Семья", CircleIDs: []string{b.ID}}}
	if got := e.groups("owner"); !reflect.DeepEqual(got, want) {
		t.Fatalf("после переименования: %+v", got)
	}

	// Переименование в название, которое уже есть, сливает группы.
	if err := e.ch.PutCircleInGroup(e.ctx, a.ID, "owner", "Дача", e.at(4)); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.RenameGroup(e.ctx, "owner", "Дача", "Семья"); err != nil {
		t.Fatal(err)
	}
	if got := e.groups("owner"); len(got) != 1 || len(got[0].CircleIDs) != 2 {
		t.Fatalf("после слияния: %+v", got)
	}

	if err := e.ch.DropGroup(e.ctx, "owner", "Семья"); err != nil {
		t.Fatal(err)
	}
	if got := e.groups("owner"); len(got) != 0 {
		t.Fatalf("после удаления: %+v", got)
	}
	if err := e.ch.PutCircleInGroup(e.ctx, a.ID, "owner", "   ", e.at(5)); !errors.Is(err, chronicle.ErrInvalid) {
		t.Fatalf("пустое название: %v", err)
	}
}

// Ушёл из круга — круг уходит из групп; ушёл совсем — и закрепление.
// Удалённый круг уходит отовсюду. Чужой круг не закрепить.
func TestStreetFollowsMembership(t *testing.T) {
	e := newTestEnv(t)
	c := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.join(c.ID, "bob", "Боб", e.at(1))

	if err := e.ch.PinCircle(e.ctx, c.ID, "stranger", e.at(1)); !errors.Is(err, chronicle.ErrNotFound) {
		t.Fatalf("чужой круг закреплён: %v", err)
	}
	if err := e.ch.PinCircle(e.ctx, c.ID, "bob", e.at(2)); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.PutCircleInGroup(e.ctx, c.ID, "bob", "Семья", e.at(2)); err != nil {
		t.Fatal(err)
	}

	if err := e.ch.LeaveWithAccess(e.ctx, c.ID, "bob", e.at(3)); err != nil {
		t.Fatal(err)
	}
	if got := e.groups("bob"); len(got) != 0 {
		t.Fatalf("вышел, а круг в группе: %+v", got)
	}
	pins, _ := e.ch.AccountPins(e.ctx, "bob")
	if !pins[c.ID].Equal(e.at(2)) {
		t.Fatalf("вышедший с доступом потерял закрепление: %v", pins)
	}
	if err := e.ch.PutCircleInGroup(e.ctx, c.ID, "bob", "Семья", e.at(3)); !errors.Is(err, chronicle.ErrForbidden) {
		t.Fatalf("вышедший положил круг в группу: %v", err)
	}

	if err := e.ch.Leave(e.ctx, c.ID, "bob", e.at(4)); err != nil {
		t.Fatal(err)
	}
	if pins, _ := e.ch.AccountPins(e.ctx, "bob"); len(pins) != 0 {
		t.Fatalf("ушёл совсем, а закрепление осталось: %v", pins)
	}

	if err := e.ch.PinCircle(e.ctx, c.ID, "owner", e.at(5)); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.PutCircleInGroup(e.ctx, c.ID, "owner", "Семья", e.at(5)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ch.DB().ExecContext(e.ctx, `DELETE FROM circles WHERE id = ?`, c.ID); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := e.ch.DB().QueryRowContext(e.ctx, `
		SELECT (SELECT count(*) FROM circle_groups) + (SELECT count(*) FROM circle_group_items) + (SELECT count(*) FROM circle_pins)
	`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatalf("после удаления круга осталось строк раскладки: %d", left)
	}
}
