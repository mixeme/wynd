package search_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/search"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

func TestSearchRespectsVisibilitySpan(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ch, err := chronicle.New(st)
	if err != nil {
		t.Fatal(err)
	}
	svc := search.New(ch)
	ctx := t.Context()
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)

	circle, _, _, err := ch.CreateCircle(ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "уникальный секрет",
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

	hits, err := svc.SearchCircle(ctx, "bob", circle.ID, "уникальный", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("newcomer search: got %d hits, want 0", len(hits))
	}

	ownerHits, err := svc.SearchCircle(ctx, "owner", circle.ID, "уникальный", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ownerHits) != 1 {
		t.Fatalf("owner search: got %d hits, want 1", len(ownerHits))
	}
	if ownerHits[0].AuthorName == "" {
		t.Fatal("circle search should include author")
	}

	all, err := svc.SearchAll(ctx, "bob", "уникальный", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Fatalf("newcomer global search: got %d hits, want 0", len(all))
	}

	ownerAll, err := svc.SearchAll(ctx, "owner", "уникальный", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ownerAll) != 1 {
		t.Fatalf("owner global search: got %d hits, want 1", len(ownerAll))
	}
	if ownerAll[0].AuthorName != "" {
		t.Fatalf("global search must omit author, got %q", ownerAll[0].AuthorName)
	}
}
