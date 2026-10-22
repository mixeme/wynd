package chronicle_test

import (
	"errors"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Длительность ролика (4.16а, C47): только у видео, в разумных пределах;
// доходит до записи и до «Сетки», у ролика без неё — ноль.
func TestVideoDurationStoredAndShown(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.DurationWindow(24*time.Hour))
	with := e.post(circle.ID, "owner", "ролик", "2026-10-04", e.at(0))
	without := e.post(circle.ID, "owner", "старый ролик", "2026-10-03", e.after(time.Minute))
	for _, id := range []string{"blob-video", "blob-old", "blob-photo"} {
		e.seedBlob(id, "owner")
	}
	for _, bad := range []chronicle.MediaInput{
		{BlobID: "blob-photo", Kind: chronicle.MediaPhoto, VideoDurationMs: 72_000},
		{BlobID: "blob-video", Kind: chronicle.MediaVideo, VideoDurationMs: -1},
		{BlobID: "blob-video", Kind: chronicle.MediaVideo, VideoDurationMs: chronicle.MaxVideoDurationMs + 1},
	} {
		if err := e.ch.AttachMedia(e.ctx, with.ID, []chronicle.MediaInput{bad}); !errors.Is(err, chronicle.ErrInvalid) {
			t.Fatalf("%+v: %v", bad, err)
		}
	}
	if err := e.ch.AttachMedia(e.ctx, with.ID, []chronicle.MediaInput{{
		BlobID: "blob-video", Kind: chronicle.MediaVideo, IsCover: true, VideoDurationMs: 72_000,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.AttachMedia(e.ctx, without.ID, []chronicle.MediaInput{{
		BlobID: "blob-old", Kind: chronicle.MediaVideo, IsCover: true,
	}}); err != nil {
		t.Fatal(err)
	}

	media, err := e.ch.ListPostMedia(e.ctx, with.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := chronicle.SummarizeMedia(media); len(got) != 1 || got[0].VideoDurationMs != 72_000 {
		t.Fatalf("summary: %+v", got)
	}

	items, _, err := e.ch.GridPage(e.ctx, circle.ID, "owner", nil)
	if err != nil {
		t.Fatal(err)
	}
	byPost := map[string]chronicle.GridItem{}
	for _, item := range items {
		byPost[item.PostID] = item
	}
	if byPost[with.ID].DurationMs != 72_000 || byPost[without.ID].DurationMs != 0 {
		t.Fatalf("grid: %+v / %+v", byPost[with.ID], byPost[without.ID])
	}
}
