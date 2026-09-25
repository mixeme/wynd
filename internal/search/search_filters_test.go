package search_test

import (
	"context"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/search"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

func TestSearchFilters(t *testing.T) {
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
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	lat, lng := 55.75, 37.62

	circle, _, _, err := ch.CreateCircle(ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = ch.Join(ctx, chronicle.JoinInput{
		CircleID: circle.ID, AccountID: "bob", Name: "Боб", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}

	seedBlob(t, ch, ctx, "blob-photo", "owner", now)
	seedBlob(t, ch, ctx, "blob-video", "owner", now)
	seedBlob(t, ch, ctx, "blob-geo", "owner", now)

	textPost, err := ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "альфа январь",
		EntryDate: "2026-01-15", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	photoPost, err := ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "альфа снимок",
		EntryDate: "2026-03-20", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ch.AttachMedia(ctx, photoPost.ID, []chronicle.MediaInput{{
		BlobID: "blob-photo", Kind: chronicle.MediaPhoto,
	}}); err != nil {
		t.Fatal(err)
	}
	videoPost, err := ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "bob", Body: "альфа ролик",
		EntryDate: "2026-04-10", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ch.AttachMedia(ctx, videoPost.ID, []chronicle.MediaInput{{
		BlobID: "blob-video", Kind: chronicle.MediaVideo,
	}}); err != nil {
		t.Fatal(err)
	}
	geoPost, err := ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "bob", Body: "альфа координаты",
		EntryDate: "2026-05-05", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ch.AttachMedia(ctx, geoPost.ID, []chronicle.MediaInput{{
		BlobID: "blob-geo", Kind: chronicle.MediaPhoto, GeoLat: &lat, GeoLng: &lng,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := ch.SetDayTitle(ctx, chronicle.DayTitleInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-03-20",
		Title: "альфа заголовок дня", Now: now,
	}); err != nil {
		t.Fatal(err)
	}

	t.Run("date range", func(t *testing.T) {
		hits, err := svc.SearchCircle(ctx, "owner", circle.ID, "альфа", 50, search.Filters{
			From: "2026-03-01",
			To:   "2026-04-30",
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) < 2 {
			t.Fatalf("date range: got %d hits, want at least 2", len(hits))
		}
		for _, h := range hits {
			if h.EntryDate < "2026-03-01" || h.EntryDate > "2026-04-30" {
				t.Fatalf("out of range: %+v", h)
			}
			if h.PostID == textPost.ID {
				t.Fatalf("january post must be excluded: %+v", h)
			}
		}
	})

	t.Run("has photo includes video", func(t *testing.T) {
		hits, err := svc.SearchCircle(ctx, "owner", circle.ID, "альфа", 50, search.Filters{HasPhoto: true})
		if err != nil {
			t.Fatal(err)
		}
		kinds := map[string]int{}
		for _, h := range hits {
			kinds[h.Kind]++
			if h.PostID == "" {
				t.Fatalf("day hit with has_photo: %+v", h)
			}
		}
		if kinds["post"] < 2 {
			t.Fatalf("has_photo: got posts %v, want photo and video", kinds)
		}
	})

	t.Run("has location excludes days", func(t *testing.T) {
		hits, err := svc.SearchCircle(ctx, "owner", circle.ID, "альфа", 50, search.Filters{HasLocation: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 1 {
			t.Fatalf("has_location: got %d hits, want 1", len(hits))
		}
		if hits[0].PostID != geoPost.ID {
			t.Fatalf("has_location: got %+v", hits[0])
		}
	})

	t.Run("authors lists distinct names from matches", func(t *testing.T) {
		names, err := svc.SearchCircleAuthors(ctx, "owner", circle.ID, "альфа", search.Filters{})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"Аня", "Боб"}
		if len(names) != len(want) {
			t.Fatalf("authors: got %v, want %v", names, want)
		}
		for i, n := range names {
			if n != want[i] {
				t.Fatalf("authors[%d]: got %q, want %q", i, n, want[i])
			}
		}
	})

	t.Run("authors respect filters without author chip", func(t *testing.T) {
		names, err := svc.SearchCircleAuthors(ctx, "owner", circle.ID, "альфа", search.Filters{HasLocation: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(names) != 1 || names[0] != "Боб" {
			t.Fatalf("authors has_location: got %v", names)
		}
	})

	t.Run("author excludes days and other authors", func(t *testing.T) {
		hits, err := svc.SearchCircle(ctx, "owner", circle.ID, "альфа", 50, search.Filters{Author: "Боб"})
		if err != nil {
			t.Fatal(err)
		}
		for _, h := range hits {
			if h.Kind == "day" {
				t.Fatalf("author filter must exclude days: %+v", h)
			}
			if h.AuthorName != "Боб" {
				t.Fatalf("wrong author: %+v", h)
			}
		}
		if len(hits) < 2 {
			t.Fatalf("author: got %d hits from Боб, want at least 2", len(hits))
		}
	})

	t.Run("plain text without filters", func(t *testing.T) {
		hits, err := svc.SearchCircle(ctx, "owner", circle.ID, "альфа январь", 50, search.Filters{})
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 1 || hits[0].PostID != textPost.ID {
			t.Fatalf("plain: %+v", hits)
		}
	})
}

func seedBlob(t *testing.T, ch *chronicle.Chronicle, ctx context.Context, id, accountID string, now time.Time) {
	t.Helper()
	_, err := ch.DB().ExecContext(ctx, `
		INSERT OR IGNORE INTO accounts (id, email, created_at) VALUES (?, ?, ?)
	`, accountID, accountID+"@test.local", now.UTC().Format(time.RFC3339))
	if err != nil {
		t.Fatalf("seedAccount %s: %v", accountID, err)
	}
	_, err = ch.DB().ExecContext(ctx, `
		INSERT INTO blobs (id, account_id, sha256, size_bytes, mime_type, storage_path, status, created_at)
		VALUES (?, ?, 'deadbeef', 1, 'image/jpeg', 'de/ad', 'complete', ?)
	`, id, accountID, now.UTC().Format(time.RFC3339))
	if err != nil {
		t.Fatalf("seedBlob %s: %v", id, err)
	}
}
