package chronicle_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// График отсечки считает текст, как квота (C55): круг из одних текстовых
// записей — со столбиками, «Освободится» не ноль, комментарий добавляет к
// столбику столько же, сколько к квоте.
func TestVolumeCountsTextLikeQuota(t *testing.T) {
	ch := newTestChronicle(t)
	ctx := t.Context()
	now := time.Now().UTC()
	circle, _, _, err := ch.CreateCircle(ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня", Now: now.Add(-3 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	post, err := ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "на даче", EntryDate: now.Format("2006-01-02"), Now: now.Add(-2 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	total := func() int64 {
		vol, _, err := ch.MediaVolumeChart(ctx, circle.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(vol) == 0 {
			t.Fatal("текстовый круг без столбиков")
		}
		return vol[len(vol)-1].CumulativeBytes
	}
	textBytes := func() int64 {
		var n int64
		if err := ch.DB().QueryRowContext(ctx, `SELECT text_bytes FROM circles WHERE id = ?`, circle.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before, quotaBefore := total(), textBytes()
	if before <= 0 {
		t.Fatalf("вес записи: %d", before)
	}
	if _, err := ch.CreateComment(ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "owner", PostID: post.ID, Body: "а компот будет?", Now: now.Add(-time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	// Комментарий пишет и событие (его вес квота тоже считает, но архивация
	// служебные строки не трогает), поэтому сравниваем без него: прирост
	// столбика — ровно прирост квоты за строку комментария.
	grew := total() - before
	want := int64(560 + 3*len("а компот будет?") + len("Аня"))
	if grew != want {
		t.Fatalf("комментарий добавил к столбику %d, ждали %d (квота выросла на %d)", grew, want, textBytes()-quotaBefore)
	}
	freed, err := ch.FreedBytesBeforeCutoff(ctx, circle.ID, now.Add(48*time.Hour).Format("2006-01-02"))
	if err != nil {
		t.Fatal(err)
	}
	if freed != total() {
		t.Fatalf("освободится %d, а весь график — %d", freed, total())
	}
}
