package archive_test

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/archive"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Инвариант (wynd.html, «Квота и архив»; план 43, B4): архив свёрстан как
// клиент — цвет круга в стилях, Golos Text лежит внутри ZIP и подключён
// относительным путём, внешних ссылок нет в обеих раскладках.
func TestArchiveCarriesCircleStyleOffline(t *testing.T) {
	fonts := fstest.MapFS{
		"fonts/GolosText-Variable-cyrillic.woff2": {Data: []byte("cyr")},
		"fonts/GolosText-Variable-latin.woff2":    {Data: []byte("lat")},
	}
	posts := []chronicle.FeedPost{{
		Post: chronicle.Post{
			ID: "p1", AuthorName: "Аня", Body: "на даче", EntryDate: "2026-08-09",
			CreatedAt: time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC),
		},
		Reactions: []chronicle.Reaction{{AuthorName: "Кот", Emoji: "heart"}},
	}}
	for _, layout := range []archive.Layout{archive.LayoutFeed, archive.LayoutPosts} {
		t.Run(string(layout), func(t *testing.T) {
			blobs, cleanup := openBlobStore(t)
			defer cleanup()
			var buf bytes.Buffer
			if err := archive.BuildPersonalArchive(t.Context(), &buf, archive.BuildInput{
				CircleName: "Дача", CutoffDate: "2026-08-10", Layout: layout,
				Posts: posts, Blobs: blobs, Color: "olive", Fonts: fonts,
			}); err != nil {
				t.Fatal(err)
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
				data, _ := io.ReadAll(rc)
				_ = rc.Close()
				files[f.Name] = string(data)
			}
			if files["fonts/GolosText-Variable-cyrillic.woff2"] != "cyr" {
				t.Fatal("шрифт не положен в архив")
			}
			index := files["index.html"]
			if !strings.Contains(index, "--c:#58673A") {
				t.Fatal("нет цвета круга olive")
			}
			if !strings.Contains(index, `url("fonts/GolosText-Variable-cyrillic.woff2")`) {
				t.Fatal("шрифт не подключён относительным путём")
			}
			if !strings.Contains(index, "9 августа 2026") && layout == archive.LayoutFeed {
				t.Fatal("дата не по-человечески")
			}
			for name, body := range files {
				if strings.HasSuffix(name, ".html") && archive.HasExternalLinks(body) {
					t.Fatalf("%s: внешняя ссылка", name)
				}
			}
			if layout == archive.LayoutPosts {
				page := files["posts/p1.html"]
				if !strings.Contains(page, `url("../fonts/GolosText-Variable-cyrillic.woff2")`) {
					t.Fatal("страница записи не видит шрифт")
				}
				if !strings.Contains(page, "♥ Кот") {
					t.Fatal("реакция не с именем")
				}
			}
		})
	}
}

// Без сборки клиента архив всё равно собирается — на системном шрифте.
func TestArchiveWithoutFonts(t *testing.T) {
	blobs, cleanup := openBlobStore(t)
	defer cleanup()
	var buf bytes.Buffer
	if err := archive.BuildPersonalArchive(t.Context(), &buf, archive.BuildInput{
		CircleName: "Дача", CutoffDate: "2026-08-10", Blobs: blobs,
	}); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(buf.Bytes(), []byte("@font-face")) {
		t.Fatal("@font-face без файла шрифта")
	}
}
