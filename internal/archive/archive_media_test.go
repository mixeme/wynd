package archive_test

// A26: карточка записи — сетка превью 88 × 88 с «Ещё N» и просмотр поверх
// (кадры приняты владельцем 2026-11-16): без JS плитки остаются ссылками на
// файлы, спрятанное раскрывает <noscript>; в ленте шапка карточки ведёт на
// страницу записи, на самой странице — нет.

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	archive "gitea.mixdep.ru/mix/wynd/internal/archive"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func TestMediaGridAndViewerMarkup(t *testing.T) {
	blobs, cleanup := openBlobStore(t)
	defer cleanup()
	ctx := t.Context()

	var items []chronicle.PostMedia
	add := func(mime, name string, kind chronicle.MediaKind) {
		t.Helper()
		b, err := blobs.PutGenerated(ctx, "acc1", mime, name, []byte("data"))
		if err != nil {
			t.Fatal(err)
		}
		items = append(items, chronicle.PostMedia{BlobID: b.ID, Kind: kind})
	}
	// Семь фото и ролик третьим по порядку: восемь вложений — шесть видны,
	// два последних фото прячутся за «Ещё 2 фото».
	add("image/jpeg", "p1.jpg", chronicle.MediaPhoto)
	add("image/jpeg", "p2.jpg", chronicle.MediaPhoto)
	add("video/mp4", "v1.mp4", chronicle.MediaVideo)
	for i := 3; i <= 7; i++ {
		add("image/jpeg", fmt.Sprintf("p%d.jpg", i), chronicle.MediaPhoto)
	}

	var buf bytes.Buffer
	err := archive.BuildPersonalArchive(ctx, &buf, archive.BuildInput{
		CircleID:   "circle1",
		CircleName: "Семья",
		CutoffDate: "2026-08-10",
		Color:      "olive",
		Posts: []chronicle.FeedPost{{
			Post: chronicle.Post{ID: "post1", IdentityID: "anya", AuthorName: "Аня", Body: "медиа",
				EntryDate: "2026-08-09", CreatedAt: time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)},
			Media: items,
		}},
		Blobs: blobs,
	})
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	pages := map[string]string{}
	for _, f := range zr.File {
		if f.Name != "index.html" && !strings.HasPrefix(f.Name, "posts/") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(rc)
		_ = rc.Close()
		pages[f.Name] = string(data)
	}
	index, ok := pages["index.html"]
	if !ok {
		t.Fatal("нет index.html")
	}

	// Сетка: восемь плиток, шесть видимых, «Ещё 2 вложения» (фото и ролик).
	if got := strings.Count(index, "class=\"thumb-body\""); got != 8 {
		t.Fatalf("плиток %d, ожидалось 8", got)
	}
	if !strings.Contains(index, "<section class=\"archive-media\"><div class=\"thumbs\">") {
		t.Fatal("нет секции .archive-media с первыми плитками")
	}
	if !strings.Contains(index, "<div class=\"extra\" hidden>") {
		t.Fatal("нет спрятанных плиток .extra")
	}
	if !strings.Contains(index, "<span>Ещё 2 фото</span>") ||
		!strings.Contains(index, "data-more=\"Ещё 2 фото\"") {
		t.Fatal("нет кнопки «Ещё 2 фото»")
	}
	if !strings.Contains(index, "<noscript><style>.archive-media .extra{display:block!important}</style></noscript>") {
		t.Fatal("без JS спрятанное не раскрывается")
	}
	// Плитка — ссылка на файл (офлайн-чтение без JS) и данные для просмотра.
	if !strings.Contains(index, "class=\"thumb-body\" href=\"media/") {
		t.Fatal("плитка не ссылается на файл")
	}
	if !strings.Contains(index, "data-kind=\"video\"") || !strings.Contains(index, "class=\"ic vid-mark\"") {
		t.Fatal("у ролика нет плитки со значком воспроизведения")
	}
	if !strings.Contains(index, "data-cap=\"Аня · 9 августа 2026, 12:00 UTC\"") {
		t.Fatal("в подписи просмотра нет автора и абсолютной даты UTC")
	}
	// Просмотр поверх: оболочка .lb строится скриптом на странице.
	if !strings.Contains(index, "document.querySelectorAll('.archive-media')") ||
		!strings.Contains(index, "'lb fixed'") {
		t.Fatal("нет скрипта просмотра поверх")
	}
	// В ленте шапка карточки ведёт на страницу записи.
	if !strings.Contains(index, "<a class=\"pa\" href=\"posts/post1.html\">") {
		t.Fatal("шапка карточки в ленте не ведёт на страницу записи")
	}

	post, ok := pages["posts/post1.html"]
	if !ok {
		t.Fatal("нет страницы записи")
	}
	if !strings.Contains(post, "<div class=\"pa\">") || strings.Contains(post, "<a class=\"pa\"") {
		t.Fatal("на странице записи шапка не должна быть ссылкой")
	}
	if !strings.Contains(post, "class=\"thumb-body\" href=\"../media/") {
		t.Fatal("на странице записи пути медиа не с ../")
	}
}
