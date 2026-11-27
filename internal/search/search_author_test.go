package search_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/search"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

// Замечание владельца 27.11: поиск не находил по имени автора. Имя — часть
// сказанного в круге, и набранное имя должно находить записи и комментарии
// человека, в том числе запись из одних фото (тела нет, а имя есть).
func TestSearchFindsByAuthorName(t *testing.T) {
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
	now := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)

	circle, _, _, err := ch.CreateCircle(ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ch.Join(ctx, chronicle.JoinInput{
		CircleID: circle.ID, AccountID: "bob", Name: "Боб", Now: now,
	}); err != nil {
		t.Fatal(err)
	}

	textPost, err := ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "снимки с дачи",
		EntryDate: "2026-11-20", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	seedBlob(t, ch, ctx, "blob-photo", "owner", now)
	photoPost, err := ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-11-21",
		AllowEmptyBody: true, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ch.AttachMedia(ctx, photoPost.ID, []chronicle.MediaInput{{
		BlobID: "blob-photo", Kind: chronicle.MediaPhoto,
	}}); err != nil {
		t.Fatal(err)
	}
	comment, err := ch.CreateComment(ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "bob", PostID: textPost.ID,
		Body: "спасибо за снимки", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}

	find := func(account, q string) []search.Hit {
		t.Helper()
		hits, err := svc.SearchCircle(ctx, account, circle.ID, q, 50, search.Filters{})
		if err != nil {
			t.Fatalf("поиск %q: %v", q, err)
		}
		return hits
	}

	// Имя целиком и по началу: запись с текстом и запись из одних фото.
	for _, q := range []string{"Аня", "ан"} {
		hits := find("owner", q)
		if len(hits) != 2 {
			t.Fatalf("поиск %q: %d попаданий, ждём 2", q, len(hits))
		}
		seen := map[string]bool{}
		for _, h := range hits {
			if h.AuthorName != "Аня" {
				t.Fatalf("поиск %q: автор %q", q, h.AuthorName)
			}
			seen[h.PostID] = true
		}
		if !seen[textPost.ID] || !seen[photoPost.ID] {
			t.Fatalf("поиск %q: попадания %v", q, seen)
		}
	}

	// Комментарий находится по имени своего автора.
	byComment := find("owner", "Боб")
	if len(byComment) != 1 || byComment[0].CommentID != comment.ID {
		t.Fatalf("поиск по имени комментатора: %+v", byComment)
	}

	// Поиск по слову тела не сломан.
	byBody := find("owner", "дач")
	if len(byBody) != 1 || byBody[0].PostID != textPost.ID {
		t.Fatalf("поиск по слову: %+v", byBody)
	}

	// И общий поиск по всем кругам тоже знает имя.
	all, err := svc.SearchAll(ctx, "owner", "Аня", 50, search.Filters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("общий поиск по имени: %d попаданий, ждём 2", len(all))
	}

	// Фильтр автора по-прежнему работает и без слов.
	filtered, err := svc.SearchCircle(ctx, "owner", circle.ID, "", 50, search.Filters{Author: "Аня"})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 2 {
		t.Fatalf("фильтр автора: %d попаданий, ждём 2", len(filtered))
	}

	// Авторы попаданий — из индекса, а не из состава круга: по имени находится
	// ровно тот, кого назвали.
	authors, err := svc.SearchCircleAuthors(ctx, "owner", circle.ID, "Аня", search.Filters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(authors) != 1 || authors[0] != "Аня" {
		t.Fatalf("авторы попаданий: %v", authors)
	}
}

// Имя — такое же сказанное: новичок не находит им то, что было до его
// прихода, а удалённая запись уходит из поиска по имени.
func TestSearchByAuthorRespectsVisibility(t *testing.T) {
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
	now := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)

	circle, _, _, err := ch.CreateCircle(ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	post, err := ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "до прихода",
		EntryDate: "2026-11-20", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ch.Join(ctx, chronicle.JoinInput{
		CircleID: circle.ID, AccountID: "bob", Name: "Боб", Now: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	hits, err := svc.SearchCircle(ctx, "bob", circle.ID, "Аня", 50, search.Filters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("новичок нашёл по имени: %+v", hits)
	}

	if _, err := ch.DeletePost(ctx, circle.ID, "owner", post.ID, now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	hits, err = svc.SearchCircle(ctx, "owner", circle.ID, "Аня", 50, search.Filters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("удалённая запись нашлась по имени: %+v", hits)
	}
}
