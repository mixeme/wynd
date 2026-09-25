package chronicle_test

import (
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func TestListAccountCirclesBatchUnreadAndLastEvent(t *testing.T) {
	e := newTestEnv(t)
	circleA := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	circleB := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())

	postA := e.post(circleA.ID, "owner", "в круге A", "2026-08-01", e.at(0))
	e.post(circleB.ID, "owner", "в круге B", "2026-08-01", e.at(1))

	if err := e.ch.SetReadCursor(e.ctx, "owner", circleA.ID, postA.EventSeq, e.at(2)); err != nil {
		t.Fatal(err)
	}

	list, err := e.ch.ListAccountCircles(e.ctx, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("circles: got %d, want 2", len(list))
	}

	byID := make(map[string]chronicle.CircleSummary, len(list))
	for _, row := range list {
		byID[row.ID] = row
	}
	rowA := byID[circleA.ID]
	rowB := byID[circleB.ID]
	if rowA.Unread != 0 {
		t.Fatalf("circle A unread: got %d, want 0", rowA.Unread)
	}
	if rowB.Unread != 1 {
		t.Fatalf("circle B unread: got %d, want 1", rowB.Unread)
	}
	if rowA.LastReadSeq != postA.EventSeq {
		t.Fatalf("circle A cursor: got %d, want %d", rowA.LastReadSeq, postA.EventSeq)
	}
	if rowB.LastReadSeq != 0 {
		t.Fatalf("circle B cursor: got %d, want 0", rowB.LastReadSeq)
	}
	if rowA.LastSummary == "" || rowA.LastAt == nil {
		t.Fatal("circle A should keep last visible event after read")
	}
	if rowB.LastSummary == "" || rowB.LastAt == nil {
		t.Fatal("circle B should have last visible event")
	}
}

func TestListAccountCirclesRespectsVisibility(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.post(circle.ID, "owner", "до входа", "2026-08-01", e.at(0))
	e.join(circle.ID, "newbie", "Боря", e.at(1))
	e.post(circle.ID, "newbie", "после входа", "2026-08-02", e.at(1))

	list, err := e.ch.ListAccountCircles(e.ctx, "newbie")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("circles: got %d, want 1", len(list))
	}
	if list[0].Unread != 1 {
		t.Fatalf("unread: got %d, want 1", list[0].Unread)
	}
	unreadLegacy, err := e.ch.UnreadPostCount(e.ctx, "newbie", circle.ID)
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Unread != unreadLegacy {
		t.Fatalf("batch unread %d != legacy %d", list[0].Unread, unreadLegacy)
	}
	summary, at, ok, err := e.ch.LastVisibleEvent(e.ctx, circle.ID, "newbie")
	if err != nil || !ok {
		t.Fatal("legacy last event")
	}
	if list[0].LastSummary != summary || list[0].LastAt == nil || !list[0].LastAt.Equal(at) {
		t.Fatalf("last event mismatch: %+v vs %q %v", list[0], summary, at)
	}
}
