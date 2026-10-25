package archive_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/archive"
	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func feedPost(id, author, body, day string, at time.Time, media ...chronicle.PostMedia) chronicle.FeedPost {
	return chronicle.FeedPost{
		Post:  chronicle.Post{ID: id, IdentityID: "id-" + author, AuthorName: author, Body: body, EntryDate: day, CreatedAt: at},
		Media: media,
	}
}

func writeArchive(t *testing.T, blobs *blob.Store, dir, name, cutoff string, posts ...chronicle.FeedPost) string {
	t.Helper()
	var buf bytes.Buffer
	if err := archive.BuildPersonalArchive(t.Context(), &buf, archive.BuildInput{
		CircleID: "circle1", CircleName: "Семья", CutoffDate: cutoff, Color: "olive", Posts: posts, Blobs: blobs,
	}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func zipFiles(t *testing.T, path string) map[string]string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	out := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		out[f.Name] = string(b)
	}
	return out
}

// Объединение (11б): одна лента без повторов; запись, которую правили между
// архивациями, — из архива с поздней отсечкой; файлы на месте; порядок — от
// новых к старым. Архив другого круга не объединяется.
func TestMergeArchives(t *testing.T) {
	blobs, cleanup := openBlobStore(t)
	defer cleanup()
	dir := t.TempDir()
	photo, err := blobs.PutGenerated(t.Context(), "acc1", "image/jpeg", "dacha.jpg", []byte("jpeg"))
	if err != nil {
		t.Fatal(err)
	}
	t1 := time.Date(2024, 3, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2025, 9, 1, 10, 0, 0, 0, time.UTC)
	a := writeArchive(t, blobs, dir, "a.zip", "2025-06-30",
		feedPost("p2", "Аня", "старый текст", "2025-06-01", t2),
		feedPost("p1", "Боб", "первая", "2024-03-01", t1, chronicle.PostMedia{BlobID: photo.ID, Kind: chronicle.MediaPhoto}))
	b := writeArchive(t, blobs, dir, "b.zip", "2025-12-31",
		feedPost("p3", "Аня", "осень", "2025-09-01", t3),
		feedPost("p2", "Аня", "поправленный текст", "2025-06-01", t2))

	var srcs []*archive.Source
	for _, p := range []string{a, b} {
		s, err := archive.OpenSource(p)
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		srcs = append(srcs, s)
	}
	picks, sum, err := archive.PlanMerge(srcs)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Posts != 3 || sum.Duplicates != 1 || sum.From != "2024-03-01" || sum.To != "2025-09-01" || sum.Bytes == 0 {
		t.Fatalf("итог: %+v", sum)
	}
	out := filepath.Join(dir, "merged.zip")
	if err := archive.AssembleFile(out, picks, srcs[1]); err != nil {
		t.Fatal(err)
	}
	files := zipFiles(t, out)
	index := files["index.html"]
	if !strings.Contains(index, "поправленный текст") || strings.Contains(index, "старый текст") {
		t.Fatal("взята не поздняя версия записи")
	}
	i3, i2, i1 := strings.Index(index, archive.MetaPostOpen("p3")), strings.Index(index, archive.MetaPostOpen("p2")), strings.Index(index, archive.MetaPostOpen("p1"))
	if i3 < 0 || i2 < 0 || i1 < 0 || !(i3 < i2 && i2 < i1) {
		t.Fatalf("порядок ленты: %d %d %d", i3, i2, i1)
	}
	for _, name := range []string{"posts/p1.html", "posts/p2.html", "posts/p3.html", archive.MetaName} {
		if _, ok := files[name]; !ok {
			t.Fatalf("нет %s", name)
		}
	}
	if !strings.Contains(files["posts/p2.html"], "поправленный текст") {
		t.Fatal("страница записи — не поздняя")
	}
	var media int
	for name := range files {
		if strings.HasPrefix(name, "media/") {
			media++
		}
	}
	if media != 1 || strings.Count(files["posts/index.html"], archive.MetaTocClose) != 3 {
		t.Fatalf("файлы %d, оглавление: %s", media, files["posts/index.html"])
	}
	// Объединённый архив сам открывается как источник — его можно делить.
	merged, err := archive.OpenSource(out)
	if err != nil {
		t.Fatal(err)
	}
	defer merged.Close()
	if merged.Meta.CutoffDate != "2025-12-31" || len(merged.Meta.Posts) != 3 {
		t.Fatalf("метаданные объединённого: %+v", merged.Meta)
	}

	// Деление по годам (11в).
	if ys := archive.Years(merged); len(ys) != 2 || ys[0] != 2024 || ys[1] != 2025 {
		t.Fatalf("годы: %v", ys)
	}
	picks25, sum25 := archive.PlanSplit(merged, "2025-01-01", "2025-12-31")
	if sum25.Posts != 2 {
		t.Fatalf("2025: %+v", sum25)
	}
	y25 := filepath.Join(dir, "2025.zip")
	if err := archive.AssembleFile(y25, picks25, merged); err != nil {
		t.Fatal(err)
	}
	if f := zipFiles(t, y25); strings.Contains(f["index.html"], "первая") || len(f) == 0 {
		t.Fatal("в 2025 попала запись 2024")
	}

	// Чужой круг.
	other := filepath.Join(dir, "other.zip")
	var buf bytes.Buffer
	if err := archive.BuildPersonalArchive(t.Context(), &buf, archive.BuildInput{CircleID: "circle2", CircleName: "Дача", CutoffDate: "2025-01-01", Blobs: blobs}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	o, err := archive.OpenSource(other)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	if _, _, err := archive.PlanMerge([]*archive.Source{srcs[0], o}); !errors.Is(err, archive.ErrOtherCircle) {
		t.Fatalf("чужой круг: %v", err)
	}
	// Не архив Wynd.
	junk := filepath.Join(dir, "junk.zip")
	if err := os.WriteFile(junk, []byte("not a zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.OpenSource(junk); !errors.Is(err, archive.ErrNoMeta) {
		t.Fatalf("не архив: %v", err)
	}
}

// Лента из полного архива (11г): страницы, файлы и метаданные — да,
// машинная часть (manifest.json, data/) — нет.
func TestReadableFromFull(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "Семья.wynd")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{
		"manifest.json":   `{"format":"wynd-circle","format_version":2}`,
		"data/posts.json": `{"columns":[],"rows":[]}`,
		"index.html":      "<html>лента</html>",
		"posts/p1.html":   "<html>запись</html>",
		"media/x.jpg":     "jpeg",
		archive.MetaName:  `{"format":"wynd-readable-archive","format_version":1}`,
	} {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if archive.IsEncryptedFull(full) {
		t.Fatal("открытый архив принят за зашифрованный")
	}
	out := filepath.Join(dir, "лента.zip")
	if err := archive.ReadableFromFull(full, "", out); err != nil {
		t.Fatal(err)
	}
	files := zipFiles(t, out)
	if _, ok := files["manifest.json"]; ok {
		t.Fatal("машинная часть в ленте")
	}
	for _, name := range []string{"index.html", "posts/p1.html", "media/x.jpg", archive.MetaName} {
		if _, ok := files[name]; !ok {
			t.Fatalf("нет %s", name)
		}
	}
	if _, ok := files["data/posts.json"]; ok {
		t.Fatal("data/ в ленте")
	}
}
