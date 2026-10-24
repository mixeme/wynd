package archive_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/archive"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Метаданные и метки записей (план 45, «Работа с архивами»): Мастерская
// объединяет и делит личные архивы по wynd-archive.json и кускам ленты между
// метками, не разбирая страниц. Текстов в метаданных нет; холостая сборка
// по-прежнему даёт точную длину.
func TestArchiveMetaAndPostMarkers(t *testing.T) {
	blobs, cleanup := openBlobStore(t)
	defer cleanup()
	photo, err := blobs.PutGenerated(t.Context(), "acc1", "image/jpeg", "dacha.jpg", []byte("jpeg"))
	if err != nil {
		t.Fatal(err)
	}
	voice, err := blobs.PutGenerated(t.Context(), "acc1", "audio/webm", "voice.webm", []byte("webm"))
	if err != nil {
		t.Fatal(err)
	}
	in := archive.BuildInput{
		CircleID:   "circle1",
		CircleName: "Семья",
		CutoffDate: "2026-08-10",
		Color:      "olive",
		Posts: []chronicle.FeedPost{
			{
				Post: chronicle.Post{ID: "post2", IdentityID: "anya", AuthorName: "Аня", Body: "секретный текст",
					EntryDate: "2026-08-09", CreatedAt: time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)},
				Media: []chronicle.PostMedia{{BlobID: photo.ID, Kind: chronicle.MediaPhoto}},
				Comments: []chronicle.Comment{{ID: "c1", AuthorName: "Боб", Body: "ого",
					Media: []chronicle.PostMedia{{BlobID: voice.ID, Kind: chronicle.MediaAttachment}}}},
			},
			{
				Post: chronicle.Post{ID: "post1", IdentityID: "bob", AuthorName: "Боб", Body: "привет",
					EntryDate: "2025-12-31", CreatedAt: time.Date(2025, 12, 31, 9, 0, 0, 0, time.UTC)},
			},
		},
		DayTitles: map[string]string{"2026-08-09": "Дача"},
		Blobs:     blobs,
	}
	size, err := archive.Size(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := archive.BuildPersonalArchive(t.Context(), &buf, in); err != nil {
		t.Fatal(err)
	}
	if size != int64(buf.Len()) {
		t.Fatalf("Size = %d, а архив — %d байт", size, buf.Len())
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		files[f.Name] = string(b)
	}

	raw, ok := files[archive.MetaName]
	if !ok {
		t.Fatal("нет wynd-archive.json")
	}
	if strings.Contains(raw, "секретный текст") || strings.Contains(raw, "ого") {
		t.Fatalf("в метаданных тексты: %s", raw)
	}
	var meta archive.Meta
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Format != archive.MetaFormat || meta.FormatVersion != 1 || meta.CircleID != "circle1" ||
		meta.CutoffDate != "2026-08-10" || len(meta.Posts) != 2 || len(meta.DayTitles) != 1 {
		t.Fatalf("метаданные: %+v", meta)
	}
	p := meta.Posts[0]
	if p.ID != "post2" || p.Page != "posts/post2.html" || p.AuthorID != "anya" || p.Comments != 1 || len(p.Files) != 2 {
		t.Fatalf("запись: %+v", p)
	}
	for _, name := range append(p.Files, p.Page) {
		if _, ok := files[name]; !ok {
			t.Fatalf("метаданные ссылаются на отсутствующий %s", name)
		}
	}

	// Каждая запись ленты — между своими метками, в порядке ленты.
	index := files["index.html"]
	for _, id := range []string{"post2", "post1"} {
		open := strings.Index(index, archive.MetaPostOpen(id))
		if open < 0 {
			t.Fatalf("нет метки %s", id)
		}
		end := strings.Index(index[open:], archive.MetaPostClose)
		if end < 0 || !strings.Contains(index[open:open+end], "<article class=\"post\"") {
			t.Fatalf("кусок %s без записи", id)
		}
	}
	if strings.Index(index, archive.MetaPostOpen("post2")) > strings.Index(index, archive.MetaPostOpen("post1")) {
		t.Fatal("метки не в порядке ленты")
	}
}
