package chronicle_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func archivedBodies(data *chronicle.FullArchiveData) map[string]bool {
	out := map[string]bool{}
	for _, table := range data.Tables {
		if table.Name != "posts" {
			continue
		}
		for _, row := range table.Rows {
			for i, col := range table.Columns {
				if col == "body" {
					body, _ := row[i].(string)
					out[body] = true
				}
			}
		}
	}
	return out
}

// A19: файл сохраняет предоставленное прошлое при оживлении новым лицом;
// сырой перенос сохраняет и физический журнал, и ограничение его выгрузки.
func TestArchivePreservationSurvivesReviveAndTransport(t *testing.T) {
	e := newTestEnv(t)
	for _, id := range []string{"a", "b", "stranger"} {
		e.seedAccount(id)
	}
	circle := e.createCircle("a", "Аня", chronicle.UnlimitedWindow())
	e.post(circle.ID, "a", "hidden", "2026-08-02", e.at(1))
	e.join(circle.ID, "b", "Боб", e.at(2))
	e.post(circle.ID, "b", "preserved", "2026-08-04", e.at(3))
	if err := e.ch.TransferOwnership(e.ctx, circle.ID, "a", "b", e.at(4)); err != nil {
		t.Fatal(err)
	}
	for _, transport := range []bool{false, true} {
		t.Run(map[bool]string{false: "owner-file", true: "transport"}[transport], func(t *testing.T) {
			data, err := e.ch.OwnerArchiveData(e.ctx, circle.ID)
			if transport {
				data, err = e.ch.FullArchiveData(e.ctx, circle.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			tables, accounts, blobs := archiveOverJSON(t, data)
			result, err := e.ch.ReviveCircle(e.ctx, chronicle.ReviveInput{
				FormatVersion: 2, SchemaVersion: 13, Tables: tables, Accounts: accounts, BlobMap: blobs,
				ReviverID: "stranger", Now: e.at(5), NewMember: &chronicle.ReviveNewMember{Name: "Слава"},
			})
			if err != nil {
				t.Fatal(err)
			}
			raw, err := e.ch.FullArchiveData(e.ctx, result.CircleID)
			if err != nil {
				t.Fatal(err)
			}
			if raw.Posts != map[bool]int{false: 1, true: 2}[transport] {
				t.Fatalf("physical history: %d", raw.Posts)
			}
			copy, err := e.ch.OwnerArchiveData(e.ctx, result.CircleID)
			if err != nil {
				t.Fatal(err)
			}
			if copy.Posts != 1 || !archivedBodies(copy)["preserved"] || archivedBodies(copy)["hidden"] {
				t.Fatalf("preservation: %v", archivedBodies(copy))
			}
			if can, err := e.ch.CanReadEvent(e.ctx, result.CircleID, "stranger", e.at(3)); err != nil || can {
				t.Fatalf("new member reading widened: %v %v", can, err)
			}
			e.post(result.CircleID, "stranger", "future", "2026-08-07", e.at(6))
			copy, err = e.ch.OwnerArchiveData(e.ctx, result.CircleID)
			if err != nil || copy.Posts != 2 {
				t.Fatalf("future not preserved: %v %v", copy, err)
			}
		})
	}
}

// A19: унаследованное сохранение не расширяет состав отделения.
func TestArchivePreservationDoesNotWidenSplit(t *testing.T) {
	e := newTestEnv(t)
	for _, id := range []string{"a", "b"} {
		e.seedAccount(id)
	}
	circle := e.createCircle("a", "Аня", chronicle.UnlimitedWindow())
	e.post(circle.ID, "a", "inherited-only", "2026-08-02", e.at(1))
	e.join(circle.ID, "b", "Боб", e.at(2))
	e.post(circle.ID, "b", "visible", "2026-08-04", e.at(3))
	if err := e.ch.TransferOwnership(e.ctx, circle.ID, "a", "b", e.at(4), true); err != nil {
		t.Fatal(err)
	}
	result, err := e.ch.SplitCircle(e.ctx, chronicle.SplitInput{CircleID: circle.ID, AccountID: "b", Name: "Новый", Now: e.at(5)})
	if err != nil {
		t.Fatal(err)
	}
	copy, err := e.ch.OwnerArchiveData(e.ctx, result.CircleID)
	if err != nil || copy.Posts != 1 || archivedBodies(copy)["inherited-only"] {
		t.Fatalf("split widened: %v %v", copy, err)
	}
}

// A19: отозванная сборка не воскресает, старое скачивание после возврата
// владения не разрешает удалить круг; частичное требует явного согласия.
func TestArchivePreservationRevokesJobsAndGuardsRemoval(t *testing.T) {
	e := newTestEnv(t)
	for _, id := range []string{"a", "b"} {
		e.seedAccount(id)
	}
	circle := e.createCircle("a", "Аня", chronicle.UnlimitedWindow())
	e.post(circle.ID, "a", "old", "2026-08-02", e.at(1))
	e.join(circle.ID, "b", "Боб", e.at(2))
	job, _, err := e.ch.StartFullArchive(e.ctx, circle.ID, "a", false, false, 100, e.at(3))
	if err != nil {
		t.Fatal(err)
	}
	if err := e.ch.TransferOwnership(e.ctx, circle.ID, "a", "b", e.at(4)); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.SetFullArchiveSnapshot(e.ctx, job.ID, e.at(4)); !errors.Is(err, chronicle.ErrNotFound) {
		t.Fatalf("revoked snapshot: %v", err)
	}
	if ok, err := e.ch.FinishFullArchive(e.ctx, job.ID, 100, e.at(4), time.Hour); err != nil || ok {
		t.Fatalf("resurrected: %v %v", ok, err)
	}
	job, _, err = e.ch.StartFullArchive(e.ctx, circle.ID, "b", true, false, 100, e.at(5))
	if err != nil {
		t.Fatal(err)
	}
	if err := e.ch.SetFullArchiveSnapshot(e.ctx, job.ID, e.at(5), chronicle.ArchiveSnapshot{FullJournal: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ch.RemoveFullArchive(e.ctx, job.ID, chronicle.RemovedDownloaded, e.at(5)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ch.DeleteCircle(e.ctx, circle.ID, "b", circle.Name, e.at(6), chronicle.ArchiveRemoval{ID: job.ID}); !errors.Is(err, chronicle.ErrArchivePartial) {
		t.Fatalf("no partial consent: %v", err)
	}
	if err := e.ch.UnfreezeCircle(e.ctx, circle.ID, "b", e.at(6)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ch.DeleteCircle(e.ctx, circle.ID, "b", circle.Name, e.at(6), chronicle.ArchiveRemoval{ID: job.ID, AcceptPartial: true}); !errors.Is(err, chronicle.ErrInvalid) {
		t.Fatalf("unfrozen removal: %v", err)
	}
	if err := e.ch.TransferOwnership(e.ctx, circle.ID, "b", "a", e.at(7)); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.TransferOwnership(e.ctx, circle.ID, "a", "b", e.at(8)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ch.DB().ExecContext(e.ctx, `UPDATE circles SET frozen_at=? WHERE id=?`, e.at(8).Format(time.RFC3339Nano), circle.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ch.DeleteCircle(e.ctx, circle.ID, "b", circle.Name, e.at(9), chronicle.ArchiveRemoval{ID: job.ID, AcceptPartial: true}); !errors.Is(err, chronicle.ErrInvalid) {
		t.Fatalf("stale download removal: %v", err)
	}
}

// Машинная и читаемая части используют один снимок даже при правках во время сборки.
func TestArchiveReadableUsesFrozenSnapshot(t *testing.T) {
	e := newTestEnv(t)
	e.seedAccount("a")
	circle := e.createCircle("a", "Аня", chronicle.UnlimitedWindow())
	post := e.post(circle.ID, "a", "snapshot-body", "2026-08-02", e.at(1))
	comment, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{CircleID: circle.ID, AccountID: "a", PostID: post.ID, Body: "snapshot-comment", Now: e.at(2)})
	if err != nil {
		t.Fatal(err)
	}
	e.seedBlob("photo", "a")
	e.attachPhoto(post.ID, "photo")
	e.seedBlob("avatar-snapshot", "a")
	e.seedBlob("avatar-changed", "a")
	avatar := "avatar-snapshot"
	if err := e.ch.UpdateIdentity(e.ctx, circle.ID, "a", chronicle.UpdateIdentityInput{AvatarBlobID: &avatar}, e.at(2)); err != nil {
		t.Fatal(err)
	}
	data, err := e.ch.OwnerArchiveData(e.ctx, circle.ID)
	if err != nil {
		t.Fatal(err)
	}
	avatar = "avatar-changed"
	if err := e.ch.UpdateIdentity(e.ctx, circle.ID, "a", chronicle.UpdateIdentityInput{AvatarBlobID: &avatar}, e.at(3)); err != nil {
		t.Fatal(err)
	}
	m, err := e.ch.MembershipForAccount(e.ctx, circle.ID, "a")
	if err != nil {
		t.Fatal(err)
	}
	if chronicle.ArchiveAvatars(data)[m.IdentityID] != "avatar-snapshot" {
		t.Fatal("snapshot avatar drifted")
	}
	if _, err := e.ch.DB().ExecContext(e.ctx, `UPDATE posts SET body='changed' WHERE id=?; UPDATE comments SET body='changed',deleted=1 WHERE id=?; DELETE FROM post_media WHERE post_id=?`, post.ID, comment.ID, post.ID); err != nil {
		t.Fatal(err)
	}
	feed, _, err := e.ch.ArchiveFeed(e.ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	if len(feed) != 1 || feed[0].Post.Body != "snapshot-body" || len(feed[0].Comments) != 1 || feed[0].Comments[0].Body != "snapshot-comment" || len(feed[0].Media) != 1 || feed[0].Media[0].BlobID != "photo" {
		t.Fatalf("snapshot drift: %+v", feed)
	}
}

// Старые файлы сверяются по прежнему счётчику, включая удалённые строки;
// новая схема считает видимые записи, машинные tombstone остаются в файле.
func TestArchivePreservationReviveCountCompatibility(t *testing.T) {
	e := newTestEnv(t)
	e.seedAccount("a")
	circle := e.createCircle("a", "Аня", chronicle.UnlimitedWindow())
	e.post(circle.ID, "a", "alive", "2026-08-02", e.at(1))
	deleted := e.post(circle.ID, "a", "deleted", "2026-08-03", e.at(2))
	if _, err := e.ch.DB().ExecContext(e.ctx, `UPDATE posts SET deleted=1 WHERE id=?`, deleted.ID); err != nil {
		t.Fatal(err)
	}
	data, err := e.ch.OwnerArchiveData(e.ctx, circle.ID)
	if err != nil {
		t.Fatal(err)
	}
	if data.Posts != 1 {
		t.Fatalf("new file count: %d", data.Posts)
	}
	tables, accounts, blobs := archiveOverJSON(t, data)
	for _, version := range []struct{ format, schema, count int }{{1, 36, 2}, {2, 12, 2}, {2, 13, 1}} {
		result, err := e.ch.ReviveCircle(e.ctx, chronicle.ReviveInput{FormatVersion: version.format, SchemaVersion: version.schema, Tables: tables, Accounts: accounts, BlobMap: blobs, ReviverID: "a", Now: e.at(3)})
		if err != nil || result.Posts != version.count {
			t.Fatalf("schema %d/%d: %+v %v", version.format, version.schema, result, err)
		}
	}
}

func TestArchivePreservationRespectsGapsAndDoesNotChangeReading(t *testing.T) {
	e := newTestEnv(t)
	for _, a := range []string{"a", "b", "c"} {
		e.seedAccount(a)
	}
	circle := e.createCircle("a", "Аня", chronicle.UnlimitedWindow())
	old := e.post(circle.ID, "a", "secret-before", "2026-08-04", e.at(1))
	e.join(circle.ID, "b", "Боб", e.at(2))
	first := e.post(circle.ID, "a", "first-seen", "2026-08-04", e.at(3))
	m, err := e.ch.MembershipForAccount(e.ctx, circle.ID, "b")
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.ch.DB().ExecContext(e.ctx, `UPDATE membership_spans SET ended_at = ? WHERE membership_id = ? AND ended_at IS NULL`, e.at(4).Format("2006-01-02T15:04:05.000000000Z"), m.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.ch.DB().ExecContext(e.ctx, `INSERT INTO membership_spans(id,membership_id,started_at,can_read) VALUES ('b-return',?,?,1)`, m.ID, e.at(6).Format("2006-01-02T15:04:05.000000000Z"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.ch.DB().ExecContext(e.ctx, `INSERT INTO membership_spans(id,membership_id,started_at,ended_at,can_read,can_write) VALUES ('b-revoked',?,?,?,0,0)`, m.ID, e.at(0).Format("2006-01-02T15:04:05.000000000Z"), e.at(2).Format("2006-01-02T15:04:05.000000000Z"))
	if err != nil {
		t.Fatal(err)
	}
	e.post(circle.ID, "a", "secret-gap", "2026-08-06", e.at(5))
	e.post(circle.ID, "a", "seen-after-return", "2026-08-08", e.at(7))
	e.seedBlob("hidden-photo", "a")
	e.attachPhoto(old.ID, "hidden-photo")
	e.seedBlob("visible-photo", "a")
	e.attachPhoto(first.ID, "visible-photo")
	for _, input := range []chronicle.CommentInput{
		{CircleID: circle.ID, AccountID: "a", PostID: old.ID, Body: "secret-parent-comment", Now: e.at(7)},
		{CircleID: circle.ID, AccountID: "a", PostID: first.ID, Body: "secret-gap-comment", Now: e.at(5)},
	} {
		if _, err := e.ch.CreateComment(e.ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.ch.SetReaction(e.ctx, chronicle.ReactionInput{CircleID: circle.ID, AccountID: "a", PostID: old.ID, Emoji: "heart", Now: e.at(7)}); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.SetDayTitle(e.ctx, chronicle.DayTitleInput{CircleID: circle.ID, AccountID: "a", EntryDate: "2026-08-04", Title: "secret-gap-title", Now: e.at(5)}); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.SetDayCover(e.ctx, chronicle.DayCoverInput{CircleID: circle.ID, AccountID: "a", EntryDate: "2026-08-04", PostID: old.ID, BlobID: "hidden-photo", Now: e.at(7)}); err != nil {
		t.Fatal(err)
	}
	// Видимое по времени редактирование скрытой записи не раскрывает её payload.
	_, err = e.ch.DB().ExecContext(e.ctx, `INSERT INTO events(id,circle_id,event_type,is_service,target_id,payload,summary,created_at)
		VALUES ('edit-hidden',?,'post.edited',0,?,'{"body":"secret-edited"}','',?)`, circle.ID, old.ID, e.at(7).Format("2006-01-02T15:04:05.000000000Z"))
	if err != nil {
		t.Fatal(err)
	}
	if err := e.ch.TransferOwnership(e.ctx, circle.ID, "a", "b", e.at(8)); err != nil {
		t.Fatal(err)
	}
	data, err := e.ch.OwnerArchiveData(e.ctx, circle.ID)
	if err != nil {
		t.Fatal(err)
	}
	bodies := archivedBodies(data)
	if len(bodies) != 2 || !bodies["first-seen"] || !bodies["seen-after-return"] || !data.PartialJournal || data.Posts != 2 {
		t.Fatalf("archive: %+v partial=%v", bodies, data.PartialJournal)
	}
	encoded, _ := json.Marshal(data)
	if strings.Contains(string(encoded), "secret-") || strings.Contains(string(encoded), "hidden-photo") {
		t.Fatalf("hidden history escaped: %s", encoded)
	}
	if len(data.BlobIDs) != 1 || data.BlobIDs[0] != "visible-photo" {
		t.Fatalf("files: %v", data.BlobIDs)
	}
	for _, at := range []int{1, 3, 5, 7} {
		read, err := e.ch.CanReadEvent(e.ctx, circle.ID, "b", e.at(at))
		if err != nil {
			t.Fatal(err)
		}
		if read != (at == 3 || at == 7) {
			t.Fatalf("reading changed at %d: %v", at, read)
		}
	}
	raw, err := e.ch.FullArchiveData(e.ctx, circle.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(archivedBodies(raw)) != 4 || raw.PartialJournal {
		t.Fatal("transport snapshot lost circle history")
	}
	_, posts, _, full, err := e.ch.OwnerArchiveEstimate(e.ctx, circle.ID)
	if err != nil || posts != 2 || full {
		t.Fatalf("estimate: %d %v %v", posts, full, err)
	}
	// Передаётся и уже унаследованный объём, не только чтение нового владельца.
	e.join(circle.ID, "c", "Кот", e.at(9))
	if err := e.ch.TransferOwnership(e.ctx, circle.ID, "b", "c", e.at(10), true); err != nil {
		t.Fatal(err)
	}
	next, err := e.ch.OwnerArchiveData(e.ctx, circle.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(archivedBodies(next)) != 2 {
		t.Fatal("inherited preservation lost")
	}
	if read, _ := e.ch.CanReadEvent(e.ctx, circle.ID, "c", e.at(3)); read {
		t.Fatal("preservation widened reading")
	}
	// Создатель при возврате вправе сохранить всё, что сам видел.
	if err := e.ch.TransferOwnership(e.ctx, circle.ID, "c", "a", e.at(11)); err != nil {
		t.Fatal(err)
	}
	back, err := e.ch.OwnerArchiveData(e.ctx, circle.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(archivedBodies(back)) != 4 || back.PartialJournal {
		t.Fatal("own visible history was not retained")
	}
}

func TestArchivePreservationPassesInheritedWholeJournal(t *testing.T) {
	e := newTestEnv(t)
	for _, a := range []string{"a", "b", "c", "d"} {
		e.seedAccount(a)
	}
	circle := e.createCircle("a", "Аня", chronicle.UnlimitedWindow())
	e.post(circle.ID, "a", "old-history", "2026-08-02", e.at(1))
	e.join(circle.ID, "b", "Боб", e.at(2))
	e.join(circle.ID, "c", "Кот", e.at(3))
	e.join(circle.ID, "d", "Даша", e.at(4))
	if err := e.ch.TransferOwnership(e.ctx, circle.ID, "a", "b", e.at(5), true); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.TransferOwnership(e.ctx, circle.ID, "b", "c", e.at(6), true); err != nil {
		t.Fatal(err)
	}
	data, err := e.ch.OwnerArchiveData(e.ctx, circle.ID)
	if err != nil || !archivedBodies(data)["old-history"] || data.PartialJournal {
		t.Fatalf("inherited whole: %+v %v", data, err)
	}
	if read, _ := e.ch.CanReadEvent(e.ctx, circle.ID, "c", e.at(1)); read {
		t.Fatal("inherited whole widened reading")
	}
	if err := e.ch.TransferOwnership(e.ctx, circle.ID, "c", "d", e.at(7)); err != nil {
		t.Fatal(err)
	}
	limited, err := e.ch.OwnerArchiveData(e.ctx, circle.ID)
	if err != nil || archivedBodies(limited)["old-history"] || !limited.PartialJournal {
		t.Fatalf("default retained past: %+v %v", limited, err)
	}
}
