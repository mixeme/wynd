package chronicle_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func feedComments(t *testing.T, e *testEnv, circleID, accountID string) []chronicle.Comment {
	t.Helper()
	posts, _, err := e.ch.FeedPage(e.ctx, circleID, accountID, nil)
	if err != nil {
		t.Fatal(err)
	}
	var out []chronicle.Comment
	for _, fp := range posts {
		out = append(out, fp.Comments...)
	}
	return out
}

// Комментарий несёт фото, голосовое и файл; слова необязательны (4.28).
func TestCommentWithMedia(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(circle.ID, "owner", "яблоки", "2026-08-06", e.after(time.Minute))
	for _, b := range []string{"b-photo", "b-voice", "b-file"} {
		e.seedBlob(b, "owner")
	}
	cm, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "owner", PostID: p.ID, Now: e.after(2 * time.Minute),
		Media: []chronicle.MediaInput{
			{BlobID: "b-photo", Kind: chronicle.MediaPhoto, IsCover: true},
			{BlobID: "b-voice", Kind: chronicle.MediaAttachment, Voice: true, AudioDurationMs: 48000, AudioPeaks: []int{10, 90}},
			{BlobID: "b-file", Kind: chronicle.MediaAttachment},
		},
	})
	if err != nil {
		t.Fatalf("CreateComment: %v", err)
	}
	if len(cm.Media) != 3 || cm.Body != "" {
		t.Fatalf("созданный комментарий: %+v", cm)
	}
	got := feedComments(t, e, circle.ID, "owner")
	if len(got) != 1 || len(got[0].Media) != 3 {
		t.Fatalf("в ленте: %+v", got)
	}
	m := got[0].Media
	if m[0].BlobID != "b-photo" || m[0].IsCover || !m[1].Voice || m[1].AudioDurationMs != 48000 ||
		len(m[1].AudioPeaks) != 2 || m[2].BlobID != "b-file" {
		t.Fatalf("вложения: %+v", m)
	}

	// В «Сетку» и запасную обложку дня снимок комментария не попадает.
	grid, _, err := e.ch.GridPage(e.ctx, circle.ID, "owner", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(grid) != 0 {
		t.Fatalf("сетка: %+v", grid)
	}
	days, err := e.ch.DaysSnapshot(e.ctx, circle.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 1 || days[0].FallbackCoverBlobID != "" || days[0].PhotoCount != 0 {
		t.Fatalf("дни: %+v", days)
	}

	// Слова стереть можно: вложения остаются.
	if err := e.ch.EditComment(e.ctx, circle.ID, "owner", cm.ID, "  ", e.after(3*time.Minute)); err != nil {
		t.Fatalf("правка в пустоту при вложениях: %v", err)
	}

	blobs, err := e.ch.DeleteComment(e.ctx, circle.ID, "owner", cm.ID, e.after(4*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(blobs) != 3 {
		t.Fatalf("освобождаемые блобы: %v", blobs)
	}
	if got := feedComments(t, e, circle.ID, "owner"); len(got) != 0 {
		t.Fatalf("после удаления: %+v", got)
	}
}

// Пустой комментарий, видео, лишние и повторные вложения — отказ.
func TestCommentMediaRejects(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(circle.ID, "owner", "яблоки", "2026-08-06", e.after(time.Minute))
	e.seedBlob("b-1", "owner")
	try := func(body string, media []chronicle.MediaInput) error {
		_, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
			CircleID: circle.ID, AccountID: "owner", PostID: p.ID, Body: body,
			Now: e.after(2 * time.Minute), Media: media,
		})
		return err
	}
	if err := try(" ", nil); err != chronicle.ErrInvalid {
		t.Fatalf("пустой: %v", err)
	}
	if err := try("", []chronicle.MediaInput{{BlobID: "b-1", Kind: chronicle.MediaVideo}}); err != chronicle.ErrInvalid {
		t.Fatalf("видео: %v", err)
	}
	if err := try("", []chronicle.MediaInput{
		{BlobID: "b-1", Kind: chronicle.MediaPhoto}, {BlobID: "b-1", Kind: chronicle.MediaPhoto},
	}); err != chronicle.ErrInvalid {
		t.Fatalf("повтор блоба: %v", err)
	}
	many := make([]chronicle.MediaInput, chronicle.MaxCommentMedia+1)
	for i := range many {
		many[i] = chronicle.MediaInput{BlobID: "b-1", Kind: chronicle.MediaPhoto}
	}
	if err := try("", many); err != chronicle.ErrInvalid {
		t.Fatalf("больше предела: %v", err)
	}
	// Комментарий без вложений в пустоту не правится.
	cm, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "owner", PostID: p.ID, Body: "слова", Now: e.after(3 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.ch.EditComment(e.ctx, circle.ID, "owner", cm.ID, "", e.after(4*time.Minute)); err != chronicle.ErrInvalid {
		t.Fatalf("правка в пустоту без вложений: %v", err)
	}
}

// Удаление записи уносит вложения её комментариев; повтор по ключу очереди
// не плодит вложения.
func TestCommentMediaLeavesWithPostAndReplay(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(circle.ID, "owner", "яблоки", "2026-08-06", e.after(time.Minute))
	e.seedBlob("b-photo", "owner")
	in := chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "owner", PostID: p.ID, ClientID: "k1",
		Now:   e.after(2 * time.Minute),
		Media: []chronicle.MediaInput{{BlobID: "b-photo", Kind: chronicle.MediaPhoto}},
	}
	first, err := e.ch.CreateComment(e.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	again, err := e.ch.CreateComment(e.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != first.ID || !again.Replayed || len(again.Media) != 1 {
		t.Fatalf("повтор: %+v", again)
	}
	var n int
	if err := e.ch.DB().QueryRowContext(e.ctx, `SELECT COUNT(*) FROM comment_media`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("строк вложений %d (%v), нужна одна", n, err)
	}
	blobs, err := e.ch.DeletePost(e.ctx, circle.ID, "owner", p.ID, e.after(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(blobs) != 1 || blobs[0] != "b-photo" {
		t.Fatalf("блобы записи: %v", blobs)
	}
	if err := e.ch.DB().QueryRowContext(e.ctx, `SELECT COUNT(*) FROM comment_media`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("после удаления записи строк вложений %d (%v)", n, err)
	}
}
