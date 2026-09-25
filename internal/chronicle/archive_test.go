package chronicle_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/archive"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Инвариант: пока идёт цикл архивации, комментарий и реакция на запись до
// отсечки запрещены, после отсечки — нет.
func TestArchiveCycleBlocksInteractionBeforeCutoff(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	old := e.post(circle.ID, "owner", "до отсечки", "2026-08-01", e.at(0))
	newer := e.post(circle.ID, "owner", "после отсечки", "2026-08-06", e.at(6))
	if err := e.ch.StartArchiveCycle(e.ctx, circle.ID, "owner", "2026-08-05", e.at(10), 86400, e.at(1)); err != nil {
		t.Fatal(err)
	}
	_, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "owner", PostID: old.ID, Body: "нельзя", Now: e.at(2),
	})
	if !errors.Is(err, chronicle.ErrForbidden) {
		t.Fatalf("comment before cutoff: %v", err)
	}
	_, err = e.ch.SetReaction(e.ctx, chronicle.ReactionInput{
		CircleID: circle.ID, AccountID: "owner", PostID: old.ID, Emoji: "heart", Now: e.at(2),
	})
	if !errors.Is(err, chronicle.ErrForbidden) {
		t.Fatalf("reaction before cutoff: %v", err)
	}
	if _, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "owner", PostID: newer.ID, Body: "можно", Now: e.at(7),
	}); err != nil {
		t.Fatalf("comment after cutoff: %v", err)
	}
	if _, err := e.ch.SetReaction(e.ctx, chronicle.ReactionInput{
		CircleID: circle.ID, AccountID: "owner", PostID: newer.ID, Emoji: "heart", Now: e.at(7),
	}); err != nil {
		t.Fatalf("reaction after cutoff: %v", err)
	}
}

// Инвариант: комментарий и реакция на запись вне своего отрезка видимости —
// forbidden.
func TestPrejoinCommentAndReactionForbidden(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(circle.ID, "owner", "раньше входа", "2026-08-01", e.at(0))
	e.join(circle.ID, "guest", "Боря", e.at(3))
	_, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "guest", PostID: p.ID, Body: "нельзя", Now: e.at(4),
	})
	if !errors.Is(err, chronicle.ErrForbidden) {
		t.Fatalf("prejoin comment: %v", err)
	}
	_, err = e.ch.SetReaction(e.ctx, chronicle.ReactionInput{
		CircleID: circle.ID, AccountID: "guest", PostID: p.ID, Emoji: "heart", Now: e.at(4),
	})
	if !errors.Is(err, chronicle.ErrForbidden) {
		t.Fatalf("prejoin reaction: %v", err)
	}
}

// Инвариант: отсечку двигают до первого скачивания архива и не двигают после.
func TestArchiveCutoffMovesUntilLocked(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	deadline := e.at(10)
	if err := e.ch.StartArchiveCycle(e.ctx, circle.ID, "owner", "2026-08-05", deadline, 86400, e.at(0)); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.MoveCutoff(e.ctx, circle.ID, "owner", "2026-08-10", e.at(1)); err != nil {
		t.Fatal(err)
	}
	cycle, err := e.ch.GetArchiveCycle(e.ctx, circle.ID)
	if err != nil || cycle.CutoffDate != "2026-08-10" {
		t.Fatalf("cutoff: %+v err=%v", cycle, err)
	}
	if _, err := e.ch.LockCutoff(e.ctx, circle.ID, e.at(2)); err != nil {
		t.Fatal(err)
	}
	err = e.ch.MoveCutoff(e.ctx, circle.ID, "owner", "2026-08-15", e.at(3))
	if !errors.Is(err, chronicle.ErrForbidden) {
		t.Fatalf("locked cutoff move: %v", err)
	}
}

// Инвариант: архивы двух участников с разными отрезками не совпадают — каждый
// получает своё.
func TestArchiveSnapshotsDifferBySpan(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.post(circle.ID, "owner", "ранняя", "2026-08-01", e.at(0))
	e.join(circle.ID, "guest", "Боря", e.at(2))
	e.post(circle.ID, "guest", "после входа", "2026-08-02", e.at(3))

	cutoff := "2026-08-10"
	ownerSnap, err := e.ch.ArchiveSnapshot(e.ctx, circle.ID, "owner", cutoff)
	if err != nil {
		t.Fatal(err)
	}
	guestSnap, err := e.ch.ArchiveSnapshot(e.ctx, circle.ID, "guest", cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if len(ownerSnap) != 2 || len(guestSnap) != 1 {
		t.Fatalf("owner=%d guest=%d posts", len(ownerSnap), len(guestSnap))
	}
}

// Инвариант: архив открывается офлайн — в HTML нет внешних ссылок, но авторы
// на месте.
func TestArchiveHTMLHasNoExternalLinks(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	posts := []chronicle.FeedPost{{
		Post: chronicle.Post{
			ID: "p1", AuthorName: "Аня", Body: "текст", EntryDate: "2026-08-01", CreatedAt: now,
		},
	}}
	html := archive.BuildFeedHTMLForTest("Семья", "2026-08-10", posts)
	if archive.HasExternalLinks(html) {
		t.Fatal("archive HTML must not reference external URLs")
	}
	if !strings.Contains(html, "Аня") {
		t.Fatal("expected author in HTML")
	}
}

// Инвариант: чистка по отсечке стирает сказанное, но оставляет служебные
// события — структура круга остаётся.
func TestArchivePurgeKeepsStructuralEvents(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.post(circle.ID, "owner", "удалится", "2026-08-01", e.at(0))
	deadline := e.at(5)
	if err := e.ch.StartArchiveCycle(e.ctx, circle.ID, "owner", "2026-08-10", deadline, 86400, e.at(1)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ch.PurgeBeforeCutoff(e.ctx, circle.ID, "2026-08-10", e.at(6)); err != nil {
		t.Fatal(err)
	}
	var serviceCount int
	err := e.ch.DB().QueryRowContext(e.ctx, `
		SELECT COUNT(*) FROM events WHERE circle_id = ? AND is_service = 1
	`, circle.ID).Scan(&serviceCount)
	if err != nil || serviceCount < 3 {
		t.Fatalf("structural events remain: count=%d err=%v", serviceCount, err)
	}
	remains, err := eventTextRemains(e, circle.ID, "удалится")
	if err != nil || remains {
		t.Fatal("said content should be scrubbed")
	}
}

// Инвариант: в архив не попадает ничего после отсечки — ни записи, ни
// комментарии, ни реакции.
func TestArchiveSnapshotOmitsContentAfterCutoff(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(circle.ID, "owner", "ранняя", "2026-08-01", e.at(0))
	if _, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "owner", PostID: p.ID, Body: "после отсечки", Now: e.at(6),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ch.SetReaction(e.ctx, chronicle.ReactionInput{
		CircleID: circle.ID, AccountID: "owner", PostID: p.ID, Emoji: "heart", Now: e.at(6),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "owner", PostID: p.ID, Body: "до отсечки", Now: e.at(1),
	}); err != nil {
		t.Fatal(err)
	}

	snap, err := e.ch.ArchiveSnapshot(e.ctx, circle.ID, "owner", "2026-08-05")
	if err != nil {
		t.Fatal(err)
	}
	if len(snap) != 1 {
		t.Fatalf("posts: %d", len(snap))
	}
	if len(snap[0].Comments) != 1 || snap[0].Comments[0].Body != "до отсечки" {
		t.Fatalf("comments: %+v", snap[0].Comments)
	}
	if len(snap[0].Reactions) != 0 {
		t.Fatalf("reaction after cutoff leaked: %+v", snap[0].Reactions)
	}
}

// Инвариант: новый цикл архивации начинается только после закрытия прежнего.
func TestArchiveNewCycleAfterLock(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	if err := e.ch.StartArchiveCycle(e.ctx, circle.ID, "owner", "2026-08-05", e.at(10), 86400, e.at(0)); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.StartArchiveCycle(e.ctx, circle.ID, "owner", "2026-08-08", e.at(11), 86400, e.at(1)); err == nil {
		t.Fatal("unlocked cycle must not restart")
	}
	if _, err := e.ch.LockCutoff(e.ctx, circle.ID, e.at(2)); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.StartArchiveCycle(e.ctx, circle.ID, "owner", "2026-08-12", e.at(20), 86400, e.at(3)); err != nil {
		t.Fatal(err)
	}
	cycle, err := e.ch.GetArchiveCycle(e.ctx, circle.ID)
	if err != nil || cycle.CutoffDate != "2026-08-12" || cycle.CutoffLockedAt != nil {
		t.Fatalf("new cycle: %+v err=%v", cycle, err)
	}
}

// Инвариант: напоминание о сроке уходит не раньше своего окна и переносится
// вместе со сроком.
func TestArchiveReminderWaitsForWindowAndReschedules(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	deadline := e.at(10)
	if err := e.ch.StartArchiveCycle(e.ctx, circle.ID, "owner", "2026-08-05", deadline, 86400, e.at(0)); err != nil {
		t.Fatal(err)
	}
	ids, err := e.ch.CirclesDueForArchiveReminder(e.ctx, e.at(0))
	if err != nil || len(ids) != 0 {
		t.Fatalf("immediate reminder: %v %v", ids, err)
	}
	ids, err = e.ch.CirclesDueForArchiveReminder(e.ctx, e.at(8))
	if err != nil || len(ids) != 0 {
		t.Fatalf("too early: %v %v", ids, err)
	}
	ids, err = e.ch.CirclesDueForArchiveReminder(e.ctx, e.at(9))
	if err != nil || len(ids) != 1 {
		t.Fatalf("due reminder: %v %v", ids, err)
	}
	if err := e.ch.MarkArchiveReminderSent(e.ctx, circle.ID, e.at(9)); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.MoveDeadline(e.ctx, circle.ID, "owner", e.at(12), e.at(9)); err != nil {
		t.Fatal(err)
	}
	ids, err = e.ch.CirclesDueForArchiveReminder(e.ctx, e.at(10))
	if err != nil || len(ids) != 0 {
		t.Fatalf("after move, same interval not yet: %v %v", ids, err)
	}
	ids, err = e.ch.CirclesDueForArchiveReminder(e.ctx, e.at(11))
	if err != nil || len(ids) != 1 {
		t.Fatalf("rescheduled reminder: %v %v", ids, err)
	}
}

// Инвариант: чистка по сроку срабатывает, даже если архив скачали не все.
func TestArchivePurgeRunsRegardlessOfDownloads(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.post(circle.ID, "owner", "старая", "2026-08-01", e.at(0))
	if err := e.ch.StartArchiveCycle(e.ctx, circle.ID, "owner", "2026-08-10", e.at(2), 86400, e.at(0)); err != nil {
		t.Fatal(err)
	}
	ids, err := e.ch.CirclesDueForArchivePurge(e.ctx, e.at(3))
	if err != nil || len(ids) != 1 {
		t.Fatalf("due purge: %v %v", ids, err)
	}
}

// Инвариант (план 42, ARC-7): срок цикла — не раньше чем через сутки от
// старта или переноса; иначе участники не успевают скачать архив.
func TestArchiveDeadlineAtLeastADayAhead(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	now := e.at(0)
	for _, d := range []time.Time{now.Add(-time.Hour), now.Add(23 * time.Hour)} {
		if err := e.ch.StartArchiveCycle(e.ctx, circle.ID, "owner", "2026-08-10", d, 86400, now); !errors.Is(err, chronicle.ErrInvalid) {
			t.Fatalf("start with deadline %v: %v, want ErrInvalid", d, err)
		}
	}
	if err := e.ch.StartArchiveCycle(e.ctx, circle.ID, "owner", "2026-08-10", now.Add(24*time.Hour), 86400, now); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.MoveDeadline(e.ctx, circle.ID, "owner", e.at(1), e.at(1)); !errors.Is(err, chronicle.ErrInvalid) {
		t.Fatalf("move to now: %v, want ErrInvalid", err)
	}
	if err := e.ch.MoveDeadline(e.ctx, circle.ID, "owner", e.at(3), e.at(1)); err != nil {
		t.Fatal(err)
	}
}

// Инвариант: оценка архива не считает недогруженные файлы — обещанный объём
// не завышается.
func TestEstimateArchivePersonalSkipsIncompleteBlobs(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(circle.ID, "owner", "фото", "2026-08-01", e.at(0))
	e.seedBlob("ok", "owner")
	e.attachPhoto(p.ID, "ok")
	_, err := e.ch.DB().ExecContext(e.ctx, `
		INSERT INTO blobs (id, account_id, sha256, size_bytes, mime_type, storage_path, status, created_at)
		VALUES ('pending', 'owner', 'deadbeef', 99, 'image/jpeg', 'de/ad', 'pending', ?)
	`, e.t0.UTC().Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	e.attachPhoto(p.ID, "pending")
	stats, err := e.ch.EstimateArchivePersonal(e.ctx, circle.ID, "owner", "2026-08-20")
	if err != nil {
		t.Fatal(err)
	}
	if stats.PostCount != 1 {
		t.Fatalf("posts=%d", stats.PostCount)
	}
	if stats.MediaFiles != 1 {
		t.Fatalf("files=%d want 1 (incomplete blob must not count)", stats.MediaFiles)
	}
	if stats.MediaBytes != 1 {
		t.Fatalf("bytes=%d", stats.MediaBytes)
	}
}
