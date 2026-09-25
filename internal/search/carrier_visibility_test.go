package search_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/search"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

func newSearchEnv(t *testing.T) (*chronicle.Chronicle, *search.Service) {
	t.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ch, err := chronicle.New(st)
	if err != nil {
		t.Fatal(err)
	}
	return ch, search.New(ch)
}

// Инвариант (SRCH-1): попадание видно, только если видна запись-носитель.
// Новичок не должен находить комментарий к записи, которой не видит, — вместе
// с миниатюрой этой скрытой записи.
func TestSearchHidesCommentOnInvisiblePost(t *testing.T) {
	ch, svc := newSearchEnv(t)
	ctx := t.Context()
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)

	circle, _, _, err := ch.CreateCircle(ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	post, err := ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "старая запись",
		EntryDate: "2026-08-29", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ch.DB().ExecContext(ctx, `
		INSERT INTO accounts (id, email, created_at) VALUES ('owner', 'owner@test.local', ?)
	`, now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err := ch.DB().ExecContext(ctx, `
		INSERT INTO blobs (id, account_id, sha256, size_bytes, mime_type, storage_path, status, created_at)
		VALUES ('blob-secret', 'owner', 'deadbeef', 1, 'image/jpeg', 'bl/ob', 'complete', ?)
	`, now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err := ch.DB().ExecContext(ctx, `
		INSERT INTO post_media (id, post_id, blob_id, kind, sort_order)
		VALUES ('pm1', ?, 'blob-secret', 'photo', 0)
	`, post.ID); err != nil {
		t.Fatal(err)
	}
	// Боб вступает позже записи — она вне его отрезка видимости.
	if _, _, err := ch.Join(ctx, chronicle.JoinInput{
		CircleID: circle.ID, AccountID: "bob", Name: "Боб", Now: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ch.CreateComment(ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "owner", PostID: post.ID,
		Body: "секретный комментарий", Now: now.Add(2 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	hits, err := svc.SearchCircle(ctx, "bob", circle.ID, "секретный", 10, search.Filters{})
	if err != nil {
		t.Fatalf("SearchCircle: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("комментарий к невидимой записи найден: %+v", hits)
	}

	// Владелец находит его и получает миниатюру — обратная сторона инварианта.
	hits, err = svc.SearchCircle(ctx, "owner", circle.ID, "секретный", 10, search.Filters{})
	if err != nil {
		t.Fatalf("SearchCircle owner: %v", err)
	}
	if len(hits) != 1 || hits[0].ThumbBlobID != "blob-secret" {
		t.Fatalf("владелец должен видеть попадание с миниатюрой: %+v", hits)
	}

	// Автор круга виден в списке авторов, бобу — нет.
	authors, err := svc.SearchCircleAuthors(ctx, "bob", circle.ID, "секретный", search.Filters{})
	if err != nil {
		t.Fatalf("SearchCircleAuthors: %v", err)
	}
	if len(authors) != 0 {
		t.Fatalf("авторы скрытого попадания просочились: %v", authors)
	}
}

// Инвариант (SRCH-1): название дня видно только внутри дат отрезка.
func TestSearchHidesDayTitleOutsideSpan(t *testing.T) {
	ch, svc := newSearchEnv(t)
	ctx := t.Context()
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)

	circle, _, _, err := ch.CreateCircle(ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "запись старого дня",
		EntryDate: "2026-08-20", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := ch.SetDayTitle(ctx, chronicle.DayTitleInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-20",
		Title: "днёмназванием", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ch.Join(ctx, chronicle.JoinInput{
		CircleID: circle.ID, AccountID: "bob", Name: "Боб", Now: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	hits, err := svc.SearchCircle(ctx, "bob", circle.ID, "днёмназванием", 10, search.Filters{})
	if err != nil {
		t.Fatalf("SearchCircle: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("название дня вне отрезка найдено: %+v", hits)
	}

	// День внутри отрезка находится.
	if _, err := ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "запись нового дня",
		EntryDate: "2026-09-01", Now: now.Add(2 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if err := ch.SetDayTitle(ctx, chronicle.DayTitleInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-09-01",
		Title: "свежийдень", Now: now.Add(3 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	hits, err = svc.SearchCircle(ctx, "bob", circle.ID, "свежийдень", 10, search.Filters{})
	if err != nil {
		t.Fatalf("SearchCircle: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("день внутри отрезка должен находиться: %+v", hits)
	}
}
