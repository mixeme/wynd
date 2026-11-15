package archive

import (
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Плитки фото записи в ленте и путь «лента → запись» (решение владельца
// 2026-11-15): шапка записи в единой ленте — ссылка на страницу записи,
// фото собираются сеткой плиток, сверх шести — за плиткой «ещё N» с
// раскрытием на месте; на странице записи фото остаются столбиком.
func TestFeedTilesAndPostLink(t *testing.T) {
	photo := func(id string) chronicle.PostMedia {
		return chronicle.PostMedia{BlobID: id, Kind: chronicle.MediaPhoto, MimeType: "image/jpeg", OriginalFilename: id + ".jpg"}
	}
	posts := []chronicle.FeedPost{
		{
			Post:  chronicle.Post{ID: "p8", AuthorName: "Аня", Body: "Двенадцать снимков за день."},
			Media: []chronicle.PostMedia{photo("a"), photo("b"), photo("c"), photo("d"), photo("e"), photo("f"), photo("g"), photo("h"), photo("i"), photo("j"), photo("k"), photo("l")},
		},
		{
			Post: chronicle.Post{ID: "p2", AuthorName: "Петя", Body: "Фото и ролик рядом."},
			Media: []chronicle.PostMedia{
				photo("x"),
				{BlobID: "v", Kind: chronicle.MediaVideo, MimeType: "video/mp4", OriginalFilename: "кадр.mp4"},
				{BlobID: "f", Kind: chronicle.MediaAttachment, MimeType: "application/pdf", OriginalFilename: "документ.pdf"},
			},
		},
	}
	media := map[string]string{
		"a": "media/a.jpg", "b": "media/b.jpg", "c": "media/c.jpg", "d": "media/d.jpg",
		"e": "media/e.jpg", "f": "media/f.pdf", "g": "media/g.jpg", "h": "media/h.jpg",
		"i": "media/i.jpg", "j": "media/j.jpg", "k": "media/k.jpg", "l": "media/l.jpg",
		"v": "media/v.mp4", "x": "media/x.jpg",
	}
	feed := renderFeedIndex(style{hex: circleHex("")}, "Круг", "2026-11-15", posts, media, nil, nil)

	if !strings.Contains(feed, `<a class="pa" href="p8.html">`) {
		t.Error("шапка записи в ленте не ссылается на страницу записи")
	}
	if got := strings.Count(feed, `<a class="tile" href="`); got != 13 {
		t.Errorf("плиток в ленте %d, ожидалось 13 (12 + 1 у второй записи)", got)
	}
	if !strings.Contains(feed, `<details class="more"><summary>ещё 6</summary><div class="rest">`) {
		t.Error("плитки сверх шести не спрятаны за «ещё N» с раскрытием на месте")
	}
	if !strings.Contains(feed, `<div class="media"><video controls src="media/v.mp4">`) {
		t.Error("ролик записи не остался во всю ширину")
	}
	if !strings.Contains(feed, `class="att" href="media/f.pdf"`) {
		t.Error("файл записи не остался карточкой .att (C39)")
	}

	page := renderPostPage(style{hex: circleHex("")}, "Круг", posts[0], media, nil, nil)
	if strings.Contains(page, `class="tile"`) {
		t.Error("на странице записи плиток быть не должно — фото столбиком")
	}
	if !strings.Contains(page, `<div class="pa">`) || strings.Contains(page, `<a class="pa"`) {
		t.Error("на странице записи шапка не должна быть ссылкой")
	}
	if got := strings.Count(page, `<div class="media"><img`); got != 12 {
		t.Errorf("фото на странице записи столбиком: %d, ожидалось 12", got)
	}
}
