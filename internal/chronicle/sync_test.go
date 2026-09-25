package chronicle_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

func newTestChronicle(t *testing.T) *chronicle.Chronicle {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ch, err := chronicle.New(st)
	if err != nil {
		t.Fatal(err)
	}
	return ch
}

func TestSyncEventsRespectsVisibilitySpan(t *testing.T) {
	ch := newTestChronicle(t)
	ctx := t.Context()
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)

	circle, _, _, err := ch.CreateCircle(ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "секрет",
		EntryDate: "2026-08-29", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = ch.Join(ctx, chronicle.JoinInput{
		CircleID: circle.ID, AccountID: "bob", Name: "Боб", Now: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "bob", Body: "после",
		EntryDate: "2026-08-30", Now: now.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	events, err := ch.SyncEvents(ctx, "bob", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range events {
		if ev.Summary != "" && contains(ev.Summary, "секрет") {
			t.Fatalf("bob saw pre-join event: %q", ev.Summary)
		}
	}
}

func TestFeedSnapshotTrimmedForNewcomer(t *testing.T) {
	ch := newTestChronicle(t)
	ctx := t.Context()
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)

	circle, _, _, err := ch.CreateCircle(ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "старое",
		EntryDate: "2026-08-29", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = ch.Join(ctx, chronicle.JoinInput{
		CircleID: circle.ID, AccountID: "bob", Name: "Боб", Now: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	feed, err := ch.FeedSnapshot(ctx, circle.ID, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if len(feed) != 0 {
		t.Fatalf("newcomer feed: got %d posts, want 0", len(feed))
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
