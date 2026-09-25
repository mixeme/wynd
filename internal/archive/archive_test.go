package archive_test

import (
	"path/filepath"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/archive"
	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

func openBlobStore(t *testing.T) (*blob.Store, func()) {
	t.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	s := st.(*store.SQLite)
	_, err = s.DB().ExecContext(t.Context(), `
		INSERT OR IGNORE INTO accounts (id, email, created_at) VALUES ('acc1', 'acc1@test.local', '2026-08-30T00:00:00Z')
	`)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	blobs, err := blob.New(st, filepath.Join(dir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	return blobs, func() { _ = st.Close() }
}

func TestBuildPersonalArchiveMissingBlob(t *testing.T) {
	blobs, cleanup := openBlobStore(t)
	defer cleanup()
	ctx := t.Context()

	_, err := archive.BuildPersonalArchive(ctx, archive.BuildInput{
		CircleName: "Семья",
		CutoffDate: "2026-08-10",
		Posts: []chronicle.FeedPost{{
			Post: chronicle.Post{
				ID:         "post1",
				AuthorName: "Аня",
				EntryDate:  "2026-08-09",
				CreatedAt:  time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC),
			},
			Media: []chronicle.PostMedia{{
				BlobID: "00000000-0000-7000-8000-000000000000",
				Kind:   chronicle.MediaPhoto,
			}},
		}},
		Blobs: blobs,
	})
	if err == nil {
		t.Fatal("expected error when blob file is missing")
	}
	if !contains(err.Error(), "blob") {
		t.Fatalf("error should mention blob: %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
