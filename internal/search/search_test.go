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
	if ownerAll[0].AuthorName == "" {
		t.Fatal("global search should include author")
	}
}

func TestSearchDayTitle(t *testing.T) {
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
	now := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)

	circle, _, _, err := ch.CreateCircle(ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "снимки с дачи",
		EntryDate: "2026-09-03", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ch.SetDayTitle(ctx, chronicle.DayTitleInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-09-03",
		Title: "Компот с дачи", Now: now,
	}); err != nil {
		t.Fatal(err)
	}

	hits, err := svc.SearchCircle(ctx, "owner", circle.ID, "компот", 10)
	if err != nil {
		t.Fatal(err)
	}
	var day *search.Hit
	for i := range hits {
		if hits[i].Kind == "day" {
			day = &hits[i]
			break
		}
	}
	if day == nil {
		t.Fatal("expected day hit")
	}
	if day.Title != "Компот с дачи" {
		t.Fatalf("day title: got %q", day.Title)
	}
	if day.EntryDate != "2026-09-03" {
		t.Fatalf("entry_date: got %q", day.EntryDate)
	}
	if day.PostID != "" {
		t.Fatalf("day post_id must be empty, got %q", day.PostID)
	}
	if day.AuthorName != "" {
		t.Fatalf("day must not have author, got %q", day.AuthorName)
	}

	all, err := svc.SearchAll(ctx, "owner", "компот", 10)
	if err != nil {
		t.Fatal(err)
	}
	var globalDay *search.Hit
	for i := range all {
		if all[i].Kind == "day" {
			globalDay = &all[i]
			break
		}
	}
	if globalDay == nil {
		t.Fatal("expected day hit in global search")
	}
	if globalDay.Title != "Компот с дачи" {
		t.Fatalf("global day title: got %q", globalDay.Title)
	}

	if err := ch.SetDayTitle(ctx, chronicle.DayTitleInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-09-03",
		Title: "Варенье с дачи", Now: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	renamed, err := svc.SearchCircle(ctx, "owner", circle.ID, "варенье", 10)
	if err != nil {
		t.Fatal(err)
	}
	var dayHits int
	for _, h := range renamed {
		if h.Kind == "day" {
			dayHits++
			if h.Title != "Варенье с дачи" {
				t.Fatalf("renamed day title: got %q", h.Title)
			}
		}
	}
	if dayHits != 1 {
		t.Fatalf("renamed day hits: got %d, want 1", dayHits)
	}
	stale, err := svc.SearchCircle(ctx, "owner", circle.ID, "компот", 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range stale {
		if h.Kind == "day" {
			t.Fatalf("old day title still in FTS: %+v", h)
		}
	}
}
