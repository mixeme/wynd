package search_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/search"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

// Файл из комментария находится по имени: находка — файл с id реплики, ведёт
// в обсуждение. Голосовое и фото не индексируются. Новичок не находит файл
// комментария, написанного до него; удалённый комментарий уходит из поиска.
func TestSearchFindsCommentFilesByName(t *testing.T) {
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
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)

	circle, _, _, err := ch.CreateCircle(ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня",
		EditWindow: chronicle.UnlimitedWindow(), Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	post, err := ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "яблоки", EntryDate: "2026-08-29", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	db := ch.DB()
	stamp := now.Format(time.RFC3339)
	if _, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO accounts (id, email, created_at) VALUES ('owner', 'owner@test.local', ?)`, stamp); err != nil {
		t.Fatal(err)
	}
	for _, b := range []struct{ id, mime, name string }{
		{"blob-pdf", "application/pdf", "вёдра_и_лестница.pdf"},
		{"blob-voice", "audio/mp4", "голосовое.m4a"},
		{"blob-photo", "image/jpeg", "яблоня.jpg"},
	} {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO blobs (id, account_id, sha256, size_bytes, mime_type, storage_path, status, created_at, original_filename)
			VALUES (?, 'owner', 'aa', 1, ?, 'aa/aa', 'complete', ?, ?)
		`, b.id, b.mime, stamp, b.name); err != nil {
			t.Fatal(err)
		}
	}
	comment, err := ch.CreateComment(ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "owner", PostID: post.ID, Body: "список", Now: now.Add(time.Minute),
		Media: []chronicle.MediaInput{
			{BlobID: "blob-pdf", Kind: chronicle.MediaAttachment},
			{BlobID: "blob-voice", Kind: chronicle.MediaAttachment, Voice: true, AudioDurationMs: 1000},
			{BlobID: "blob-photo", Kind: chronicle.MediaPhoto},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	find := func(account, q string) []search.Hit {
		t.Helper()
		hits, err := svc.SearchCircle(ctx, account, circle.ID, q, 10, search.Filters{})
		if err != nil {
			t.Fatal(err)
		}
		return hits
	}

	hits := find("owner", "лестница")
	if len(hits) != 1 || hits[0].Kind != "file" || hits[0].CommentID != comment.ID ||
		hits[0].MediaBlobID != "blob-pdf" || hits[0].PostID != post.ID || hits[0].AuthorName != "Аня" {
		t.Fatalf("by file name: %+v", hits)
	}
	for _, q := range []string{"голосовое", "яблоня"} {
		if hits = find("owner", q); len(hits) != 0 {
			t.Fatalf("%q indexed: %+v", q, hits)
		}
	}

	// Новичок видит запись? Нет: и запись, и комментарий — до его вступления.
	if _, _, err := ch.Join(ctx, chronicle.JoinInput{
		CircleID: circle.ID, AccountID: "kot", Name: "Кот", Now: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if hits = find("kot", "лестница"); len(hits) != 0 {
		t.Fatalf("newcomer finds an earlier comment file: %+v", hits)
	}

	if _, err := ch.DeleteComment(ctx, circle.ID, "owner", comment.ID, now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if hits = find("owner", "лестница"); len(hits) != 0 {
		t.Fatalf("file of a deleted comment still found: %+v", hits)
	}
}
