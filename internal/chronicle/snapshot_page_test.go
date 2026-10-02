package chronicle_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Порции ленты, «Сетки» и «Карты» (C18): курсор ведёт к старшим записям без
// пропусков и повторов; запись с несколькими фото не рвётся между порциями.
func TestSnapshotPages(t *testing.T) {
	old := chronicle.SnapshotPostLimit
	chronicle.SnapshotPostLimit = 3
	t.Cleanup(func() { chronicle.SnapshotPostLimit = old })

	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.DurationWindow(24*time.Hour))
	var ids []string
	for i := 0; i < 7; i++ {
		p := e.post(circle.ID, "owner", "запись", "2026-08-01", e.after(time.Duration(i)*time.Minute))
		ids = append(ids, p.ID)
		blob := "blob-" + p.ID
		e.seedBlob(blob, "owner")
		e.attachPhoto(p.ID, blob)
		if i == 4 {
			// У пятой записи два фото: порция «Сетки» не должна её разрезать.
			e.seedBlob(blob+"-2", "owner")
			if err := e.ch.AttachMedia(e.ctx, p.ID, []chronicle.MediaInput{{BlobID: blob + "-2", Kind: chronicle.MediaPhoto}}); err != nil {
				t.Fatal(err)
			}
		}
	}

	var got []string
	var cursor *chronicle.PageCursor
	for page := 0; page < 5; page++ {
		posts, next, err := e.ch.FeedPage(e.ctx, circle.ID, "owner", cursor)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range posts {
			got = append(got, p.Post.ID)
		}
		if next == nil {
			break
		}
		parsed, err := chronicle.ParsePageCursor(next.String())
		if err != nil {
			t.Fatal(err)
		}
		cursor = parsed
	}
	if len(got) != 7 || got[0] != ids[6] || got[6] != ids[0] {
		t.Fatalf("feed pages = %v", got)
	}

	seen := map[string]int{}
	cursor = nil
	for page := 0; page < 6; page++ {
		items, next, err := e.ch.GridPage(e.ctx, circle.ID, "owner", cursor)
		if err != nil {
			t.Fatal(err)
		}
		pagePosts := map[string]bool{}
		for _, it := range items {
			seen[it.BlobID]++
			pagePosts[it.PostID] = true
		}
		if pagePosts[ids[4]] {
			n := 0
			for _, it := range items {
				if it.PostID == ids[4] {
					n++
				}
			}
			if n != 2 {
				t.Fatalf("post with two photos split: %d in page", n)
			}
		}
		if next == nil {
			break
		}
		cursor = next
	}
	if len(seen) != 8 {
		t.Fatalf("grid saw %d photos, want 8: %v", len(seen), seen)
	}
	for b, n := range seen {
		if n != 1 {
			t.Fatalf("photo %s repeated %d times", b, n)
		}
	}

	if _, err := chronicle.ParsePageCursor("мусор"); err != chronicle.ErrInvalid {
		t.Fatalf("bad cursor: %v", err)
	}
}
