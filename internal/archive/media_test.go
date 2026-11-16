package archive

// Тексты раскрытия сетки превью (A26, кадры приняты 2026-11-16):
// «Ещё 6 фото» для фото, «Ещё N видео» для роликов, «Ещё N вложений»
// для смешанных.

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func TestMoreLabel(t *testing.T) {
	photos := func(n int) []mediaTile {
		out := make([]mediaTile, n)
		for i := range out {
			out[i] = mediaTile{src: "p.jpg"}
		}
		return out
	}
	videos := func(n int) []mediaTile {
		out := make([]mediaTile, n)
		for i := range out {
			out[i] = mediaTile{src: "v.mp4", video: true}
		}
		return out
	}
	cases := []struct {
		tiles []mediaTile
		want  string
	}{
		{photos(6), "Ещё 6 фото"},
		{photos(1), "Ещё 1 фото"},
		{videos(3), "Ещё 3 видео"},
		{append(photos(2), videos(1)...), "Ещё 3 вложения"},
		{append(photos(1), videos(4)...), "Ещё 5 вложений"},
	}
	for _, c := range cases {
		if got := moreLabel(c.tiles); got != c.want {
			t.Errorf("moreLabel(%d плиток) = %q, ожидалось %q", len(c.tiles), got, c.want)
		}
	}
}

func TestCaptionText(t *testing.T) {
	fp := chronicle.FeedPost{Post: chronicle.Post{
		AuthorName: "Аня",
		CreatedAt:  time.Date(2026, 11, 10, 21, 5, 0, 0, time.UTC),
	}}
	if got, want := captionText(fp), "Аня · 10 ноября 2026, 21:05 UTC"; got != want {
		t.Fatalf("captionText = %q, ожидалось %q", got, want)
	}
	// Время приводится к UTC, а не к часовому поясу сервера.
	fp.Post.CreatedAt = time.Date(2026, 11, 10, 23, 5, 0, 0, time.FixedZone("MSK", 3*3600))
	if got, want := captionText(fp), "Аня · 10 ноября 2026, 20:05 UTC"; got != want {
		t.Fatalf("captionText с поясом = %q, ожидалось %q", got, want)
	}
	if got := captionText(chronicle.FeedPost{Post: chronicle.Post{AuthorName: "Петя"}}); got != "Петя" {
		t.Fatalf("captionText без даты = %q", got)
	}
}
