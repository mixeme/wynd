package search_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/search"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

// Инвариант (план 46, C11): вложение находится по имени файла, звук — ещё и
// по названию и исполнителю; находка ведёт в запись и несёт id вложения,
// а не комментария. Удалённое вложение из поиска уходит.
func TestSearchFindsAttachmentsByName(t *testing.T) {
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
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	post, err := ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "вложения", EntryDate: "2026-08-29", Now: now,
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
		{"blob-pdf", "application/pdf", "смета_ремонта.pdf"},
		{"blob-mp3", "audio/mpeg", "track01.mp3"},
	} {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO blobs (id, account_id, sha256, size_bytes, mime_type, storage_path, status, created_at, original_filename)
			VALUES (?, 'owner', 'aa', 1, ?, 'aa/aa', 'complete', ?, ?)
		`, b.id, b.mime, stamp, b.name); err != nil {
			t.Fatal(err)
		}
	}
	if err := ch.AttachMedia(ctx, post.ID, []chronicle.MediaInput{
		{BlobID: "blob-pdf", Kind: chronicle.MediaAttachment},
		{BlobID: "blob-mp3", Kind: chronicle.MediaAttachment, AudioTitle: "Колыбельная", AudioArtist: "Бабушка"},
	}); err != nil {
		t.Fatal(err)
	}
	mediaID := map[string]string{}
	rows, err := db.QueryContext(ctx, `SELECT blob_id, id FROM post_media WHERE post_id = ?`, post.ID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var blob, id string
		if err := rows.Scan(&blob, &id); err != nil {
			t.Fatal(err)
		}
		mediaID[blob] = id
	}
	rows.Close()

	find := func(q string) []search.Hit {
		t.Helper()
		hits, err := svc.SearchCircle(ctx, "owner", circle.ID, q, 10, search.Filters{})
		if err != nil {
			t.Fatal(err)
		}
		return hits
	}

	hits := find("смета")
	if len(hits) != 1 || hits[0].Kind != "file" || hits[0].MediaID != mediaID["blob-pdf"] || hits[0].MediaBlobID != "blob-pdf" || hits[0].CommentID != "" || hits[0].PostID != post.ID {
		t.Fatalf("by file name: %+v", hits)
	}
	for _, q := range []string{"колыбельн", "бабушка", "track01"} {
		hits = find(q)
		if len(hits) != 1 || hits[0].Kind != "audio" || hits[0].MediaID != mediaID["blob-mp3"] {
			t.Fatalf("audio by %q: %+v", q, hits)
		}
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM post_media WHERE id = ?`, mediaID["blob-pdf"]); err != nil {
		t.Fatal(err)
	}
	if hits = find("смета"); len(hits) != 0 {
		t.Fatalf("removed attachment still found: %+v", hits)
	}
}
