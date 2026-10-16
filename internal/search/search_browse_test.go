package search_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/search"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

// Поиск без слов — «все записи за период» (концепция: текст — один фильтр из
// многих). Запись из одних фото в индексе слов нет, но период её находит;
// чужое прошлое (до вступления) не находится, как и в ленте.
func TestSearchWithoutWords(t *testing.T) {
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
	start := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

	circle, _, _, err := ch.CreateCircle(ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня", Now: start,
	})
	if err != nil {
		t.Fatal(err)
	}
	post := func(body, day string, at time.Time) chronicle.Post {
		p, err := ch.CreatePost(ctx, chronicle.PostInput{
			CircleID: circle.ID, AccountID: "owner", Body: body, EntryDate: day, Now: at,
			AllowEmptyBody: body == "",
		})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	before := post("до Боба", "2026-09-02", start.Add(time.Hour))
	_ = before
	if _, _, err := ch.Join(ctx, chronicle.JoinInput{
		CircleID: circle.ID, AccountID: "bob", Name: "Боб", Now: start.Add(2 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	inPeriod := post("яблоки", "2026-09-05", start.Add(3*time.Hour))
	seedBlob(t, ch, ctx, "blob-photo", "owner", start)
	photoOnly := post("", "2026-09-06", start.Add(4*time.Hour))
	if err := ch.AttachMedia(ctx, photoOnly.ID, []chronicle.MediaInput{{
		BlobID: "blob-photo", Kind: chronicle.MediaPhoto,
	}}); err != nil {
		t.Fatal(err)
	}
	post("позже периода", "2026-09-20", start.Add(5*time.Hour))

	period := search.Filters{From: "2026-09-01", To: "2026-09-10"}
	hits, err := svc.SearchCircle(ctx, "bob", circle.ID, "", 0, period)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 || hits[0].PostID != photoOnly.ID || hits[1].PostID != inPeriod.ID {
		t.Fatalf("за период — запись из фото и «яблоки», новые сверху; до вступления — нет: %+v", hits)
	}
	if hits[0].ThumbBlobID != "blob-photo" || hits[1].AuthorName != "Аня" {
		t.Fatalf("миниатюра и автор: %+v", hits)
	}
	all, err := svc.SearchAll(ctx, "bob", "  ", 0, period)
	if err != nil || len(all) != 2 {
		t.Fatalf("все круги без слов: %v %+v", err, all)
	}
	authors, err := svc.SearchCircleAuthors(ctx, "bob", circle.ID, "", period)
	if err != nil || len(authors) != 1 || authors[0] != "Аня" {
		t.Fatalf("авторы без слов: %v %v", err, authors)
	}
	// Ни слов, ни фильтров — по-прежнему отказ.
	if _, err := svc.SearchCircle(ctx, "bob", circle.ID, "", 0, search.Filters{}); err != chronicle.ErrInvalid {
		t.Fatalf("пустой поиск: %v", err)
	}
}
