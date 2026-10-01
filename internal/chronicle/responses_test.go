package chronicle_test

import (
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Отклики собирают комментарии, реакции и название дня — к старой записи
// тоже, без своих действий, свежее сверху.
func TestResponsesCollectOthersActivityNewestFirst(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.join(circle.ID, "kot", "Кот", e.at(0))
	old := e.post(circle.ID, "owner", "Первые яблоки\nвторая строка", "2026-07-12", e.at(1))
	e.post(circle.ID, "owner", "свежая", "2026-08-01", e.at(5))

	if _, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "kot", PostID: old.ID, Body: "а компот?", Now: e.at(6),
	}); err != nil {
		t.Fatal(err)
	}
	// свой комментарий — не отклик
	if _, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "owner", PostID: old.ID, Body: "будет", Now: e.at(6).Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ch.SetReaction(e.ctx, chronicle.ReactionInput{
		CircleID: circle.ID, AccountID: "kot", PostID: old.ID, Emoji: "heart", Now: e.at(7),
	}); err != nil {
		t.Fatal(err)
	}

	page, err := e.ch.Responses(e.ctx, circle.ID, "owner", 0)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, r := range page.Items {
		kinds = append(kinds, r.Kind)
		if r.ActorIdentityID == "" || r.ActorName != "Кот" {
			t.Fatalf("отклик не от Кота: %+v", r)
		}
	}
	if len(kinds) < 2 || kinds[0] != chronicle.ResponseReaction || kinds[len(kinds)-1] != chronicle.ResponseComment {
		t.Fatalf("порядок: %v", kinds)
	}
	ref, ok := page.Posts[old.ID]
	if !ok || ref.Excerpt != "Первые яблоки" || ref.AuthorName != "Аня" {
		t.Fatalf("рамка записи: %+v", ref)
	}

	n, err := e.ch.UnreadResponses(e.ctx, circle.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if n != len(page.Items) {
		t.Fatalf("непрочитанных: %d, откликов %d", n, len(page.Items))
	}
	if err := e.ch.SetResponseReadSeq(e.ctx, circle.ID, "owner", page.Items[0].Seq, e.at(9)); err != nil {
		t.Fatal(err)
	}
	if n, _ := e.ch.UnreadResponses(e.ctx, circle.ID, "owner"); n != 0 {
		t.Fatalf("после просмотра: %d", n)
	}
	// назад отметка не отступает
	if err := e.ch.SetResponseReadSeq(e.ctx, circle.ID, "owner", 1, e.at(9)); err != nil {
		t.Fatal(err)
	}
	if n, _ := e.ch.UnreadResponses(e.ctx, circle.ID, "owner"); n != 0 {
		t.Fatalf("отметка отступила: %d", n)
	}
}

// Удалённый комментарий исчезает из откликов: строки «удалил» нет.
func TestResponsesDropDeletedComment(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.join(circle.ID, "kot", "Кот", e.at(0))
	p := e.post(circle.ID, "owner", "запись", "2026-08-01", e.at(1))
	cm, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "kot", PostID: p.ID, Body: "ой", Now: e.at(2),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.ch.DeleteComment(e.ctx, circle.ID, "kot", cm.ID, e.at(2).Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	page, err := e.ch.Responses(e.ctx, circle.ID, "owner", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("удалённый комментарий остался: %+v", page.Items)
	}
}

// Отклик к записи, которой участник не видит (до его входа), ему не виден.
func TestResponsesRespectVisibility(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.join(circle.ID, "kot", "Кот", e.at(0))
	before := e.post(circle.ID, "owner", "до входа", "2026-08-01", e.at(0).Add(time.Hour))
	e.join(circle.ID, "newbie", "Боря", e.at(1))
	if _, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "kot", PostID: before.ID, Body: "старое", Now: e.at(2),
	}); err != nil {
		t.Fatal(err)
	}
	page, err := e.ch.Responses(e.ctx, circle.ID, "newbie", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("новичок видит отклик к невидимой записи: %+v", page.Items)
	}
}

// Название дня — строка в ленте (3.1), а не отклик: в «Откликах» его нет,
// в событиях ленты есть, и с датой — какой это день.
func TestDayTitleGoesToFeedNotResponses(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.join(circle.ID, "kot", "Кот", e.at(0))
	e.post(circle.ID, "kot", "яблоки", "2026-08-06", e.at(1))
	if err := e.ch.SetDayTitle(e.ctx, chronicle.DayTitleInput{
		CircleID: circle.ID, AccountID: "kot", EntryDate: "2026-08-06", Title: "Плёнки", Now: e.at(2),
	}); err != nil {
		t.Fatal(err)
	}
	page, err := e.ch.Responses(e.ctx, circle.ID, "owner", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("название дня в откликах: %+v", page.Items)
	}
	meta, err := e.ch.FeedMetaForAccount(e.ctx, circle.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range meta.Events {
		if ev.Summary == "Название для 6 августа — «Плёнки»: Кот" {
			found = true
		}
	}
	if !found {
		t.Fatalf("названия дня нет в ленте: %+v", meta.Events)
	}
}

// Смена фото — строка в ленте (3.1). Первое фото при вступлении — часть
// вступления, строки не даёт; то же фото повторно — тоже.
func TestAvatarChangeGoesToFeed(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.join(circle.ID, "kot", "Кот", e.at(0))
	e.seedBlob("ava1", "kot")
	e.seedBlob("ava2", "kot")
	set := func(blob string, when time.Time) {
		t.Helper()
		if err := e.ch.UpdateIdentity(e.ctx, circle.ID, "kot", chronicle.UpdateIdentityInput{AvatarBlobID: &blob}, when); err != nil {
			t.Fatal(err)
		}
	}
	lines := func() []string {
		meta, err := e.ch.FeedMetaForAccount(e.ctx, circle.ID, "owner")
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, ev := range meta.Events {
			if strings.Contains(strings.ToLower(ev.Summary), "фото") {
				out = append(out, ev.Summary)
			}
		}
		return out
	}
	set("ava1", e.at(0).Add(time.Minute))
	if got := lines(); len(got) != 0 {
		t.Fatalf("фото при вступлении дало строку: %v", got)
	}
	set("ava1", e.at(1))
	set("ava2", e.at(2))
	set("", e.at(3))
	got := lines()
	if len(got) != 2 || got[0] != "Фото убрано: Кот" || got[1] != "Новое фото: Кот" {
		t.Fatalf("строки фото: %v", got)
	}
}
