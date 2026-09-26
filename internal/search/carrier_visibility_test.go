package search_test

import (
	"errors"
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

// Инвариант (SRCH-2): запрос из нескольких слов — это AND по словам, а не
// одна фраза. Управляющие символы не доходят до FTS.
func TestSearchTokenizesQuery(t *testing.T) {
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
		CircleID: circle.ID, AccountID: "owner",
		Body: "море и лето на даче", EntryDate: "2026-08-30", Now: now,
	}); err != nil {
		t.Fatal(err)
	}

	hits, err := svc.SearchCircle(ctx, "owner", circle.ID, "море даче", 10, search.Filters{})
	if err != nil {
		t.Fatalf("SearchCircle: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("несоседние слова не нашлись: %+v", hits)
	}

	// Слово, которого нет, отсекает результат: это AND, а не OR.
	hits, err = svc.SearchCircle(ctx, "owner", circle.ID, "море горы", 10, search.Filters{})
	if err != nil {
		t.Fatalf("SearchCircle: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("AND превратился в OR: %+v", hits)
	}

	// NUL и прочие управляющие символы: отказ или пустой результат, не 500.
	for _, q := range []string{"мо\x00ре", "\x00", "   ", `"`, "NEAR(", "AND"} {
		if _, err := svc.SearchCircle(ctx, "owner", circle.ID, q, 10, search.Filters{}); err != nil &&
			!errors.Is(err, chronicle.ErrInvalid) {
			t.Fatalf("q=%q: err = %v", q, err)
		}
	}
}

// Инвариант (SRCH-2): каждое слово ищется по началу — поиск находит, пока
// слово набирается («дач» → «даче»), и ловит падежи. Середину слова не ищем:
// для неё нужен другой токенизатор.
func TestSearchMatchesWordPrefix(t *testing.T) {
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
		CircleID: circle.ID, AccountID: "owner",
		Body: "Яблони на даче зацвели", EntryDate: "2026-08-30", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	// Регистр не важен: unicode61 сворачивает его и для кириллицы.
	for _, q := range []string{"дач", "ябл дач", "Ябло", "зацвели", "ЯБЛОНИ", "ДАЧ", "яблони"} {
		hits, err := svc.SearchCircle(ctx, "owner", circle.ID, q, 10, search.Filters{})
		if err != nil {
			t.Fatalf("q=%q: %v", q, err)
		}
		if len(hits) != 1 {
			t.Fatalf("q=%q: начало слова не нашлось: %+v", q, hits)
		}
	}
	hits, err := svc.SearchCircle(ctx, "owner", circle.ID, "бло", 10, search.Filters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("середина слова не должна находиться: %+v", hits)
	}
}

// Название дня ищется по моменту, когда его дали, а не по дате дня: новичок,
// вступивший в тот же день, не находит название, данное до него.
func TestSearchHidesDayTitleGivenBeforeJoinSameDate(t *testing.T) {
	ch, svc := newSearchEnv(t)
	ctx := t.Context()
	now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	circle, _, _, err := ch.CreateCircle(ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "утро", EntryDate: "2026-09-26", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := ch.SetDayTitle(ctx, chronicle.DayTitleInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-09-26",
		Title: "дотвоегоприхода", Now: now.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ch.Join(ctx, chronicle.JoinInput{
		CircleID: circle.ID, AccountID: "bob", Name: "Боб", Now: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	hits, err := svc.SearchCircle(ctx, "bob", circle.ID, "дотвоегоприхода", 10, search.Filters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("название дня, данное до вступления, найдено: %+v", hits)
	}
	hits, err = svc.SearchCircle(ctx, "owner", circle.ID, "дотвоегоприхода", 10, search.Filters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("владелец не нашёл своё название дня: %+v", hits)
	}
}

// Инвариант (SRCH-3): в поиске лежит актуальная версия названия дня. Правка
// или удаление старой версии не должны уносить день из поиска.
func TestSearchKeepsCurrentDayTitle(t *testing.T) {
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
		CircleID: circle.ID, AccountID: "owner", Body: "запись",
		EntryDate: "2026-08-30", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	for i, title := range []string{"первоеназвание", "второеназвание"} {
		if err := ch.SetDayTitle(ctx, chronicle.DayTitleInput{
			CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-30",
			Title: title, Now: now.Add(time.Duration(i+1) * time.Hour),
		}); err != nil {
			t.Fatalf("SetDayTitle %s: %v", title, err)
		}
	}

	// Удаляем старую версию названия — так делает purge старых строк.
	if _, err := ch.DB().ExecContext(ctx, `
		DELETE FROM day_titles WHERE circle_id = ? AND title = 'первоеназвание'
	`, circle.ID); err != nil {
		t.Fatal(err)
	}

	hits, err := svc.SearchCircle(ctx, "owner", circle.ID, "второеназвание", 10, search.Filters{})
	if err != nil {
		t.Fatalf("SearchCircle: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("актуальное название дня пропало из поиска: %+v", hits)
	}
	hits, err = svc.SearchCircle(ctx, "owner", circle.ID, "первоеназвание", 10, search.Filters{})
	if err != nil {
		t.Fatalf("SearchCircle: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("удалённая версия названия осталась в поиске: %+v", hits)
	}
}

// Инвариант (аудит 2026-09-22): комментарий виден в поиске, только если сам
// попадает в отрезок читателя. SRCH-1 привязал попадание к записи-носителю,
// но вышедший с доступом находил комментарии, написанные после его ухода,
// которые лента скрывает.
func TestSearchHidesCommentWrittenAfterLeaving(t *testing.T) {
	ch, svc := newSearchEnv(t)
	ctx := t.Context()
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	circle, _, _, err := ch.CreateCircle(ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ch.Join(ctx, chronicle.JoinInput{
		CircleID: circle.ID, AccountID: "bob", Name: "Боб", Now: now.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	post, err := ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "запись", EntryDate: "2026-08-30", Now: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ch.CreateComment(ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "owner", PostID: post.ID, Body: "общее слово", Now: now.Add(90 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := ch.LeaveWithAccess(ctx, circle.ID, "bob", now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := ch.CreateComment(ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "owner", PostID: post.ID, Body: "секретное слово", Now: now.Add(3 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	hits, err := svc.SearchCircle(ctx, "bob", circle.ID, "слово", 10, search.Filters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Snippet != "общее слово" {
		t.Fatalf("вышедший должен видеть только комментарий до ухода: %+v", hits)
	}
	all, err := svc.SearchAll(ctx, "bob", "секретное", 10, search.Filters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Fatalf("глобальный поиск отдал комментарий после ухода: %+v", all)
	}
}
