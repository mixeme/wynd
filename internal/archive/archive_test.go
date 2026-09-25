package archive_test

import (
	"archive/zip"
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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

// Инвариант (план 42, ARC-4): недоступный файл не роняет архив — он
// пропускается, а в HTML на его месте пометка. Раньше один потерянный файл
// закрывал экспорт всему кругу.
func TestBuildPersonalArchiveSkipsMissingBlob(t *testing.T) {
	blobs, cleanup := openBlobStore(t)
	defer cleanup()
	ctx := t.Context()

	var buf bytes.Buffer
	err := archive.BuildPersonalArchive(ctx, &buf, archive.BuildInput{
		CircleName: "Семья",
		CutoffDate: "2026-08-10",
		Posts: []chronicle.FeedPost{{
			Post: chronicle.Post{
				ID:         "post1",
				AuthorName: "Аня",
				Body:       "на даче",
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
	if err != nil {
		t.Fatalf("missing blob must be skipped: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var index string
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "media/") {
			t.Fatalf("unexpected media entry %s", f.Name)
		}
		if f.Name == "index.html" {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, _ := io.ReadAll(rc)
			_ = rc.Close()
			index = string(data)
		}
	}
	if !strings.Contains(index, "на даче") || !strings.Contains(index, "Файл недоступен") {
		t.Fatalf("index.html:\n%s", index)
	}
}

// Инвариант (ARC-3): начало записи в оглавлении режется по рунам и остаётся
// валидным UTF-8.
func TestPostsIndexSnippetIsValidUTF8(t *testing.T) {
	body := strings.Repeat("кириллица ", 20)
	html := archive.BuildPostsIndexForTest("Семья", "2026-08-10", []chronicle.FeedPost{{
		Post: chronicle.Post{ID: "p1", AuthorName: "Аня", Body: body, EntryDate: "2026-08-09"},
	}})
	if !utf8.ValidString(html) {
		t.Fatal("оглавление — невалидный UTF-8")
	}
	if !strings.Contains(html, "кириллица…") {
		t.Fatalf("обрыв не по слову:\n%s", html)
	}
}
