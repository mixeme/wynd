package chronicle_test

import (
	"errors"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func TestInvariantNewbieDoesNotSeePast(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.post(circle.ID, "owner", "ранняя запись", "2026-08-01", e.at(0))

	e.join(circle.ID, "newbie", "Боря", e.at(1))
	e.post(circle.ID, "newbie", "после входа", "2026-08-02", e.at(1))

	seqs, err := e.ch.VisiblePostSeqs(e.ctx, circle.ID, "newbie")
	if err != nil {
		t.Fatal(err)
	}
	if len(seqs) != 1 {
		t.Fatalf("visible posts: got %d, want 1", len(seqs))
	}
	ok, err := e.ch.CanReadEvent(e.ctx, circle.ID, "newbie", e.at(0))
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("newbie must not see events before join")
	}
}

func TestInvariantLeftWithAccessReadsUntilLeave(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.join(circle.ID, "guest", "Боря", e.at(0))
	e.post(circle.ID, "owner", "до ухода", "2026-08-01", e.at(0))
	e.post(circle.ID, "guest", "от гостя", "2026-08-01", e.at(1))

	if err := e.ch.LeaveWithAccess(e.ctx, circle.ID, "guest", e.at(2)); err != nil {
		t.Fatal(err)
	}
	e.post(circle.ID, "owner", "после ухода", "2026-08-03", e.at(3))

	ok, err := e.ch.CanReadEvent(e.ctx, circle.ID, "guest", e.at(1))
	if err != nil || !ok {
		t.Fatal("guest should read events before leave")
	}
	ok, err = e.ch.CanReadEvent(e.ctx, circle.ID, "guest", e.at(3))
	if err != nil || ok {
		t.Fatal("guest must not read events after leave")
	}
	canWrite, err := e.ch.CanWrite(e.ctx, circle.ID, "guest", e.at(4))
	if err != nil || canWrite {
		t.Fatal("guest must not write after leave")
	}
}

func TestInvariantGoneAndExcludedCannotRead(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.join(circle.ID, "gone", "Боря", e.at(0))
	e.join(circle.ID, "kicked", "Вера", e.at(0))
	e.post(circle.ID, "owner", "запись", "2026-08-01", e.at(0))

	if err := e.ch.Leave(e.ctx, circle.ID, "gone", e.at(1)); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.Exclude(e.ctx, circle.ID, "owner", "kicked", e.at(1)); err != nil {
		t.Fatal(err)
	}

	for _, account := range []string{"gone", "kicked"} {
		ok, err := e.ch.CanReadEvent(e.ctx, circle.ID, account, e.at(0))
		if err != nil || ok {
			t.Fatalf("%s must not read", account)
		}
	}

	rows, err := e.ch.DB().QueryContext(e.ctx, `
		SELECT event_type, summary FROM events
		WHERE circle_id = ? AND event_type IN ('member.left', 'member.left_with_access')
		ORDER BY seq
	`, circle.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var n int
	for rows.Next() {
		var typ, summary string
		if err := rows.Scan(&typ, &summary); err != nil {
			t.Fatal(err)
		}
		if typ != "member.left" {
			t.Fatalf("leave event type: %q", typ)
		}
		if summary != "Боря покинул круг" && summary != "Вера покинул круг" {
			t.Fatalf("leave summary: %q", summary)
		}
		n++
	}
	if n != 2 {
		t.Fatalf("leave events: %d", n)
	}
}

func TestInvariantRejoinDoesNotBridgeGap(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.join(circle.ID, "guest", "Боря", e.at(0))
	e.post(circle.ID, "owner", "в первом отрезке", "2026-08-01", e.at(0))
	if err := e.ch.LeaveWithAccess(e.ctx, circle.ID, "guest", e.at(1)); err != nil {
		t.Fatal(err)
	}
	e.post(circle.ID, "owner", "в дыре", "2026-08-02", e.at(2))
	e.join(circle.ID, "guest", "Боря", e.at(3))
	e.post(circle.ID, "owner", "после возврата", "2026-08-04", e.at(4))

	ok, err := e.ch.CanReadEvent(e.ctx, circle.ID, "guest", e.at(0))
	if err != nil || !ok {
		t.Fatal("first span visible")
	}
	ok, err = e.ch.CanReadEvent(e.ctx, circle.ID, "guest", e.at(2))
	if err != nil || ok {
		t.Fatal("gap must stay invisible after rejoin")
	}
	ok, err = e.ch.CanReadEvent(e.ctx, circle.ID, "guest", e.at(4))
	if err != nil || !ok {
		t.Fatal("second span visible")
	}
}

func TestInvariantChronicleForbidsEditAndDelete(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.ChronicleWindow())
	p := e.post(circle.ID, "owner", "навсегда", "2026-08-01", e.at(0))

	err := e.ch.EditPost(e.ctx, circle.ID, "owner", p.ID, "правка", "", e.at(1))
	if !errors.Is(err, chronicle.ErrForbidden) {
		t.Fatalf("edit: got %v, want ErrForbidden", err)
	}
	err = e.ch.DeletePost(e.ctx, circle.ID, "owner", p.ID, e.at(1))
	if !errors.Is(err, chronicle.ErrForbidden) {
		t.Fatalf("delete: got %v, want ErrForbidden", err)
	}
}

func TestInvariantEditWindowSnapshotNotRetroactive(t *testing.T) {
	e := newTestEnv(t)
	day := chronicle.DurationWindow(24 * time.Hour)
	hour := chronicle.DurationWindow(time.Hour)
	circle := e.createCircle("owner", "Аня", day)
	pDay := e.post(circle.ID, "owner", "при сутках", "2026-08-01", e.after(0))

	if err := e.ch.SetEditWindow(e.ctx, circle.ID, "owner", hour, e.after(time.Hour)); err != nil {
		t.Fatal(err)
	}
	pHour := e.post(circle.ID, "owner", "при часе", "2026-08-02", e.after(time.Hour))

	if err := e.ch.SetEditWindow(e.ctx, circle.ID, "owner", chronicle.UnlimitedWindow(), e.after(2*time.Hour)); err != nil {
		t.Fatal(err)
	}

	w1, created1, err := e.ch.LoadPostEditWindow(e.ctx, pDay.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !w1.CanEdit(created1, e.after(2*time.Hour)) {
		t.Fatal("post published under day window should remain editable for a day")
	}
	w2, created2, err := e.ch.LoadPostEditWindow(e.ctx, pHour.ID)
	if err != nil {
		t.Fatal(err)
	}
	if w2.CanEdit(created2, e.after(3*time.Hour)) {
		t.Fatal("post published under hour window must not revive under unlimited")
	}

	if err := e.ch.SetEditWindow(e.ctx, circle.ID, "owner", chronicle.ChronicleWindow(), e.after(12*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if !w1.CanEdit(created1, e.after(12*time.Hour)) {
		t.Fatal("switch to chronicle must not cancel remaining edit window")
	}
}

func TestInvariantCommentUsesOwnEditWindow(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.ChronicleWindow())
	p := e.post(circle.ID, "owner", "старая", "2026-08-01", e.at(0))

	if err := e.ch.SetEditWindow(e.ctx, circle.ID, "owner", chronicle.UnlimitedWindow(), e.at(1)); err != nil {
		t.Fatal(err)
	}
	c, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "owner", PostID: p.ID, Body: "коммент", Now: e.at(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !c.EditWindow.IsUnlimited() {
		t.Fatal("comment should snapshot unlimited window from its publication")
	}
}

func TestInvariantServiceEventsNotDeletable(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())

	var seq int64
	err := e.ch.DB().QueryRowContext(e.ctx, `
		SELECT seq FROM events WHERE circle_id = ? AND is_service = 1 LIMIT 1
	`, circle.ID).Scan(&seq)
	if err != nil {
		t.Fatal(err)
	}
	err = e.ch.DeleteServiceEvent(e.ctx, circle.ID, "owner", seq, e.at(0))
	if !errors.Is(err, chronicle.ErrForbidden) {
		t.Fatalf("got %v, want ErrForbidden", err)
	}
}

func TestInvariantLeaveDoesNotExpandDeleteRights(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.ChronicleWindow())
	e.join(circle.ID, "guest", "Боря", e.at(0))
	p := e.post(circle.ID, "guest", "моя", "2026-08-01", e.at(0))
	if err := e.ch.LeaveWithAccess(e.ctx, circle.ID, "guest", e.at(1)); err != nil {
		t.Fatal(err)
	}
	err := e.ch.DeletePost(e.ctx, circle.ID, "guest", p.ID, e.at(2))
	if !errors.Is(err, chronicle.ErrForbidden) {
		t.Fatalf("got %v, want ErrForbidden", err)
	}
}

func TestInvariantBranchDeletionScrubsText(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(circle.ID, "owner", "секрет", "2026-08-01", e.at(0))
	if _, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "owner", PostID: p.ID, Body: "тоже секрет", Now: e.at(0),
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.DeletePost(e.ctx, circle.ID, "owner", p.ID, e.at(1)); err != nil {
		t.Fatal(err)
	}
	body, err := e.ch.PostBody(e.ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		t.Fatalf("post body: %q", body)
	}
	comments, err := e.ch.CommentBodies(e.ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range comments {
		if c != "" {
			t.Fatalf("comment body remains: %q", c)
		}
	}
	if remains, err := eventTextRemains(e, circle.ID, "секрет"); err != nil {
		t.Fatal(err)
	} else if remains {
		t.Fatal("secret text remains in event payloads")
	}
	var deletedEvents int
	if err := e.ch.DB().QueryRowContext(e.ctx, `
		SELECT COUNT(*) FROM events WHERE circle_id = ? AND event_type LIKE '%.deleted'
	`, circle.ID).Scan(&deletedEvents); err != nil {
		t.Fatal(err)
	}
	if deletedEvents != 0 {
		t.Fatal("deletion of a post is not a journal event")
	}
}

func TestInvariantBackdatedDoesNotChangeFeedOrder(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p1 := e.post(circle.ID, "owner", "первая", "2026-08-01", e.at(0))
	p2 := e.post(circle.ID, "owner", "вторая", "2026-07-01", e.at(1))

	order, err := e.ch.FeedOrder(e.ctx, circle.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != p1.ID || order[1] != p2.ID {
		t.Fatalf("feed order: %v", order)
	}
}

func TestInvariantDayCollapseAndRevive(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(circle.ID, "owner", "единственная", "2026-08-05", e.at(0))
	if err := e.ch.SetDayTitle(e.ctx, chronicle.DayTitleInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-05", Title: "Пятница", Now: e.at(0),
	}); err != nil {
		t.Fatal(err)
	}
	e.seedBlob("blob-1", "owner")
	e.attachPhoto(p.ID, "blob-1")
	if err := e.ch.SetDayCover(e.ctx, chronicle.DayCoverInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-05",
		PostID: p.ID, BlobID: "blob-1", Now: e.at(0),
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.DeletePost(e.ctx, circle.ID, "owner", p.ID, e.at(1)); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.AssertDayCascadeRemoved(e.ctx, circle.ID, "2026-08-05"); err != nil {
		t.Fatal(err)
	}

	e.post(circle.ID, "owner", "снова", "2026-08-05", e.at(2))
	day, err := e.ch.GetDay(e.ctx, circle.ID, "2026-08-05")
	if err != nil {
		t.Fatal(err)
	}
	if day.Title != "" || day.CoverBlobID != "" {
		t.Fatalf("revived day should be empty: %+v", day)
	}
}

func TestInvariantDayCoverRequiresEntry(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.join(circle.ID, "guest", "Боря", e.at(0))
	p := e.post(circle.ID, "owner", "запись", "2026-08-01", e.at(0))

	err := e.ch.SetDayCover(e.ctx, chronicle.DayCoverInput{
		CircleID: circle.ID, AccountID: "guest", EntryDate: "2026-08-01",
		PostID: p.ID, BlobID: "blob-1", Now: e.at(0),
	})
	if !errors.Is(err, chronicle.ErrForbidden) {
		t.Fatalf("got %v, want ErrForbidden", err)
	}
}

func TestInvariantEraseIdentityNames(t *testing.T) {
	e := newTestEnv(t)
	circle, _, mem, err := e.ch.CreateCircle(e.ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня",
		EditWindow: chronicle.UnlimitedWindow(), Now: e.t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	before, err := e.ch.EventCount(e.ctx, circle.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.ch.ErasePersonalData(e.ctx, mem.IdentityID, e.at(0)); err != nil {
		t.Fatal(err)
	}
	after, err := e.ch.EventCount(e.ctx, circle.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("events must not change")
	}
	_, err = e.ch.ResolveIdentityName(e.ctx, mem.IdentityID)
	if !errors.Is(err, chronicle.ErrNotFound) {
		t.Fatalf("name should not resolve: %v", err)
	}
}

func TestOwnerCannotLeaveWithoutTransfer(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	err := e.ch.Leave(e.ctx, circle.ID, "owner", e.at(0))
	if !errors.Is(err, chronicle.ErrForbidden) {
		t.Fatalf("got %v, want ErrForbidden", err)
	}
	err = e.ch.LeaveWithAccess(e.ctx, circle.ID, "owner", e.at(0))
	if !errors.Is(err, chronicle.ErrForbidden) {
		t.Fatalf("leave with access: got %v, want ErrForbidden", err)
	}
}

func TestEventSummariesAreServerSide(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.post(circle.ID, "owner", "текст", "2026-08-01", e.at(0))

	var summary string
	err := e.ch.DB().QueryRowContext(e.ctx, `
		SELECT summary FROM events WHERE circle_id = ? AND event_type = 'post.created'
	`, circle.ID).Scan(&summary)
	if err != nil {
		t.Fatal(err)
	}
	if summary != "Аня опубликовал запись" {
		t.Fatalf("summary: %q", summary)
	}
}

func TestInvariantBranchDeletionScrubsEditedPayload(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(circle.ID, "owner", "черновик", "2026-08-01", e.at(0))
	if err := e.ch.EditPost(e.ctx, circle.ID, "owner", p.ID, "секрет-правка", "", e.at(0)); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.DeletePost(e.ctx, circle.ID, "owner", p.ID, e.at(1)); err != nil {
		t.Fatal(err)
	}
	if remains, err := eventTextRemains(e, circle.ID, "секрет-правка"); err != nil {
		t.Fatal(err)
	} else if remains {
		t.Fatal("edited body remains in events")
	}
	if remains, err := eventTextRemains(e, circle.ID, "черновик"); err != nil {
		t.Fatal(err)
	} else if remains {
		t.Fatal("original body remains in events")
	}
}

func TestInvariantCommentWrongCircleRejected(t *testing.T) {
	e := newTestEnv(t)
	a := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	b := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(a.ID, "owner", "запись", "2026-08-01", e.at(0))
	_, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
		CircleID: b.ID, AccountID: "owner", PostID: p.ID, Body: "чужой", Now: e.at(0),
	})
	if !errors.Is(err, chronicle.ErrInvalid) {
		t.Fatalf("got %v, want ErrInvalid", err)
	}
}

func TestSetEditWindowRejectsNegativeSeconds(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	neg := int64(-3600)
	err := e.ch.SetEditWindow(e.ctx, circle.ID, "owner", chronicle.EditWindow{Seconds: &neg}, e.at(0))
	if !errors.Is(err, chronicle.ErrInvalid) {
		t.Fatalf("got %v, want ErrInvalid", err)
	}
}

func TestInvariantLeftMemberCannotChangeSettings(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.join(circle.ID, "guest", "Боря", e.at(0))
	if _, err := e.ch.DB().ExecContext(e.ctx, `
		UPDATE memberships SET can_settings = 1 WHERE circle_id = ? AND account_id = ?
	`, circle.ID, "guest"); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.LeaveWithAccess(e.ctx, circle.ID, "guest", e.at(1)); err != nil {
		t.Fatal(err)
	}
	err := e.ch.SetEditWindow(e.ctx, circle.ID, "guest", chronicle.ChronicleWindow(), e.at(2))
	if !errors.Is(err, chronicle.ErrForbidden) {
		t.Fatalf("got %v, want ErrForbidden", err)
	}
}

func TestPostCapturedAtPayloadIsPlainTime(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	captured := e.at(0)
	p, err := e.ch.CreatePost(e.ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "фото",
		EntryDate: "2026-08-01", CapturedAt: &captured, Now: e.at(0),
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload string
	if err := e.ch.DB().QueryRowContext(e.ctx, `
		SELECT payload FROM events WHERE target_id = ? AND event_type = 'post.created'
	`, p.ID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if stringIndex(payload, `"Valid"`) >= 0 || stringIndex(payload, "sql.Null") >= 0 {
		t.Fatalf("captured_at serialized as driver type: %s", payload)
	}
}

func TestDayCollapseRemovesUserContentFromJournal(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(circle.ID, "owner", "единственная", "2026-08-05", e.at(0))
	if err := e.ch.SetDayTitle(e.ctx, chronicle.DayTitleInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-05", Title: "Пятница", Now: e.at(0),
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.DeletePost(e.ctx, circle.ID, "owner", p.ID, e.at(1)); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.AssertDayCascadeRemoved(e.ctx, circle.ID, "2026-08-05"); err != nil {
		t.Fatal(err)
	}
	if remains, err := eventTextRemains(e, circle.ID, "Пятница"); err != nil {
		t.Fatal(err)
	} else if remains {
		t.Fatal("day title is said content and must leave the journal")
	}
	var titled, deleted int
	if err := e.ch.DB().QueryRowContext(e.ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN event_type = 'day.titled' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN event_type LIKE '%.deleted' THEN 1 ELSE 0 END), 0)
		FROM events WHERE circle_id = ?
	`, circle.ID).Scan(&titled, &deleted); err != nil {
		t.Fatal(err)
	}
	if titled != 0 {
		t.Fatal("day.titled must leave the journal with the day")
	}
	if deleted != 0 {
		t.Fatal("deletion of user content is not a journal event")
	}

	e.post(circle.ID, "owner", "снова", "2026-08-05", e.at(2))
	day, err := e.ch.GetDay(e.ctx, circle.ID, "2026-08-05")
	if err != nil {
		t.Fatal(err)
	}
	if day.Title != "" {
		t.Fatalf("revived day must not reuse collapsed title: %+v", day)
	}
}

func TestInvariantCommentEditAndDeleteOwnWindow(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.ChronicleWindow())
	p := e.post(circle.ID, "owner", "старая", "2026-08-01", e.at(0))
	if err := e.ch.SetEditWindow(e.ctx, circle.ID, "owner", chronicle.UnlimitedWindow(), e.at(1)); err != nil {
		t.Fatal(err)
	}
	c, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "owner", PostID: p.ID, Body: "черновик", Now: e.at(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.ch.EditComment(e.ctx, circle.ID, "owner", c.ID, "секрет", e.at(1)); err != nil {
		t.Fatal(err)
	}
	body, err := e.ch.CommentBody(e.ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if body != "секрет" {
		t.Fatalf("body: %q", body)
	}
	err = e.ch.DeletePost(e.ctx, circle.ID, "owner", p.ID, e.at(2))
	if !errors.Is(err, chronicle.ErrForbidden) {
		t.Fatalf("post still in chronicle window: got %v", err)
	}
	if err := e.ch.DeleteComment(e.ctx, circle.ID, "owner", c.ID, e.at(2)); err != nil {
		t.Fatal(err)
	}
	body, err = e.ch.CommentBody(e.ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		t.Fatalf("deleted comment body: %q", body)
	}
	if remains, err := eventTextRemains(e, circle.ID, "секрет"); err != nil {
		t.Fatal(err)
	} else if remains {
		t.Fatal("comment text remains in journal")
	}
	if n := countDeletedJournalEvents(t, e, circle.ID); n != 0 {
		t.Fatal("comment deletion is not a journal event")
	}
}

func TestInvariantReactionDeleteAndRevive(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(circle.ID, "owner", "запись", "2026-08-01", e.at(0))
	r, err := e.ch.SetReaction(e.ctx, chronicle.ReactionInput{
		CircleID: circle.ID, AccountID: "owner", PostID: p.ID, Emoji: "laugh", Now: e.at(0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.ch.DeleteReaction(e.ctx, circle.ID, "owner", r.ID, e.at(0)); err != nil {
		t.Fatal(err)
	}
	emoji, err := e.ch.ReactionEmoji(e.ctx, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if emoji != "" {
		t.Fatalf("deleted reaction: %q", emoji)
	}
	if remains, err := eventTextRemains(e, circle.ID, "laugh"); err != nil {
		t.Fatal(err)
	} else if remains {
		t.Fatal("reaction remains in journal")
	}
	if n := countDeletedJournalEvents(t, e, circle.ID); n != 0 {
		t.Fatal("reaction deletion is not a journal event")
	}
	r2, err := e.ch.SetReaction(e.ctx, chronicle.ReactionInput{
		CircleID: circle.ID, AccountID: "owner", PostID: p.ID, Emoji: "surprise", Now: e.at(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if r2.ID != r.ID {
		t.Fatalf("revived reaction id: %s vs %s", r2.ID, r.ID)
	}
	emoji, err = e.ch.ReactionEmoji(e.ctx, r2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if emoji != "surprise" {
		t.Fatalf("revived emoji: %q", emoji)
	}
}

func TestInvariantReactionRejectsUnknownKey(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(circle.ID, "owner", "запись", "2026-08-01", e.at(0))
	_, err := e.ch.SetReaction(e.ctx, chronicle.ReactionInput{
		CircleID: circle.ID, AccountID: "owner", PostID: p.ID, Emoji: "💛", Now: e.at(0),
	})
	if err != chronicle.ErrInvalid {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInvariantClearDayTitlePeelsToPrevious(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.post(circle.ID, "owner", "запись", "2026-08-05", e.at(0))
	if err := e.ch.SetDayTitle(e.ctx, chronicle.DayTitleInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-05", Title: "Первое", Now: e.at(0),
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.SetDayTitle(e.ctx, chronicle.DayTitleInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-05", Title: "Второе", Now: e.at(0),
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.ClearDayTitle(e.ctx, circle.ID, "owner", "2026-08-05", e.at(0)); err != nil {
		t.Fatal(err)
	}
	day, err := e.ch.GetDay(e.ctx, circle.ID, "2026-08-05")
	if err != nil {
		t.Fatal(err)
	}
	if day.Title != "Первое" {
		t.Fatalf("title after peel: %q", day.Title)
	}
	if remains, err := eventTextRemains(e, circle.ID, "Второе"); err != nil {
		t.Fatal(err)
	} else if remains {
		t.Fatal("cleared title remains in journal")
	}
	if remains, err := eventTextRemains(e, circle.ID, "Первое"); err != nil {
		t.Fatal(err)
	} else if !remains {
		t.Fatal("previous title must stay until peeled")
	}
	if err := e.ch.ClearDayTitle(e.ctx, circle.ID, "owner", "2026-08-05", e.at(0)); err != nil {
		t.Fatal(err)
	}
	day, err = e.ch.GetDay(e.ctx, circle.ID, "2026-08-05")
	if err != nil {
		t.Fatal(err)
	}
	if day.Title != "" {
		t.Fatalf("untitled after last peel: %q", day.Title)
	}
	if remains, err := eventTextRemains(e, circle.ID, "Первое"); err != nil {
		t.Fatal(err)
	} else if remains {
		t.Fatal("last title remains in journal")
	}
	if n := countDeletedJournalEvents(t, e, circle.ID); n != 0 {
		t.Fatal("clearing a title is not a journal event")
	}
}

func TestInvariantClearDayTitleRequiresPostAndWindow(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.ChronicleWindow())
	e.post(circle.ID, "owner", "запись", "2026-08-05", e.at(0))
	if err := e.ch.SetDayTitle(e.ctx, chronicle.DayTitleInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-05", Title: "Летопись", Now: e.at(0),
	}); err != nil {
		t.Fatal(err)
	}
	err := e.ch.ClearDayTitle(e.ctx, circle.ID, "owner", "2026-08-05", e.at(1))
	if !errors.Is(err, chronicle.ErrForbidden) {
		t.Fatalf("chronicle title: got %v, want ErrForbidden", err)
	}

	open := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.post(open.ID, "owner", "запись", "2026-08-05", e.at(0))
	if err := e.ch.SetDayTitle(e.ctx, chronicle.DayTitleInput{
		CircleID: open.ID, AccountID: "owner", EntryDate: "2026-08-05", Title: "День", Now: e.at(0),
	}); err != nil {
		t.Fatal(err)
	}
	e.join(open.ID, "guest", "Боря", e.at(0))
	err = e.ch.ClearDayTitle(e.ctx, open.ID, "guest", "2026-08-05", e.at(0))
	if !errors.Is(err, chronicle.ErrForbidden) {
		t.Fatalf("no post that day: got %v, want ErrForbidden", err)
	}
}

func TestInvariantCoverRollsBackWhenPostDeleted(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p1 := e.post(circle.ID, "owner", "первая", "2026-08-05", e.at(0))
	p2 := e.post(circle.ID, "owner", "вторая", "2026-08-05", e.at(0))
	e.seedBlob("blob-1", "owner")
	e.seedBlob("blob-2", "owner")
	e.attachPhoto(p1.ID, "blob-1")
	e.attachPhoto(p2.ID, "blob-2")
	if err := e.ch.SetDayCover(e.ctx, chronicle.DayCoverInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-05",
		PostID: p1.ID, BlobID: "blob-1", Now: e.at(0),
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.SetDayCover(e.ctx, chronicle.DayCoverInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-05",
		PostID: p2.ID, BlobID: "blob-2", Now: e.at(0),
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.DeletePost(e.ctx, circle.ID, "owner", p2.ID, e.at(1)); err != nil {
		t.Fatal(err)
	}
	day, err := e.ch.GetDay(e.ctx, circle.ID, "2026-08-05")
	if err != nil {
		t.Fatal(err)
	}
	if day.CoverPostID != p1.ID || day.CoverBlobID != "blob-1" {
		t.Fatalf("cover rollback: %+v", day)
	}
	if remains, err := eventTextRemains(e, circle.ID, "blob-2"); err != nil {
		t.Fatal(err)
	} else if remains {
		t.Fatal("cover of deleted post remains in journal")
	}
	if n := countEventType(t, e, circle.ID, "day.cover_set"); n != 1 {
		t.Fatalf("live cover events: %d", n)
	}

	if err := e.ch.DeletePost(e.ctx, circle.ID, "owner", p1.ID, e.at(1)); err != nil {
		t.Fatal(err)
	}
	exists, err := e.ch.DayExists(e.ctx, circle.ID, "2026-08-05")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("day should collapse after last post")
	}
}

func TestInvariantClearDayCoverFallsBack(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(circle.ID, "owner", "запись", "2026-08-05", e.at(0))
	e.seedBlob("blob-1", "owner")
	e.attachPhoto(p.ID, "blob-1")
	if err := e.ch.SetDayCover(e.ctx, chronicle.DayCoverInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-05",
		PostID: p.ID, BlobID: "blob-1", Now: e.at(0),
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.ClearDayCover(e.ctx, circle.ID, "owner", "2026-08-05", e.at(0)); err != nil {
		t.Fatal(err)
	}
	day, err := e.ch.GetDay(e.ctx, circle.ID, "2026-08-05")
	if err != nil {
		t.Fatal(err)
	}
	if day.CoverPostID != "" || day.CoverBlobID != "" {
		t.Fatalf("cover after clear: %+v", day)
	}
	if n := countEventType(t, e, circle.ID, "day.cover_set"); n != 0 {
		t.Fatalf("cleared cover still in journal: %d", n)
	}
	exists, err := e.ch.DayExists(e.ctx, circle.ID, "2026-08-05")
	if err != nil || !exists {
		t.Fatal("clearing cover must not collapse the day")
	}
}

func TestCreatePostAllowsEmptyBodyWithMedia(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	e.seedBlob("blob-1", "owner")
	tx, err := e.ch.DB().BeginTx(e.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	p, err := e.ch.CreatePostInTx(e.ctx, tx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "",
		EntryDate: "2026-08-05", Now: e.at(0), AllowEmptyBody: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.ch.AttachMediaInTx(e.ctx, tx, p.ID, []chronicle.MediaInput{{
		BlobID: "blob-1", Kind: chronicle.MediaPhoto,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if p.Body != "" {
		t.Fatalf("body: %q", p.Body)
	}
}

func countDeletedJournalEvents(t *testing.T, e *testEnv, circleID string) int {
	t.Helper()
	var n int
	if err := e.ch.DB().QueryRowContext(e.ctx, `
		SELECT COUNT(*) FROM events WHERE circle_id = ? AND event_type LIKE '%.deleted'
	`, circleID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func countEventType(t *testing.T, e *testEnv, circleID, eventType string) int {
	t.Helper()
	var n int
	if err := e.ch.DB().QueryRowContext(e.ctx, `
		SELECT COUNT(*) FROM events WHERE circle_id = ? AND event_type = ?
	`, circleID, eventType).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
