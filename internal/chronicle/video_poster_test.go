package chronicle_test

import (
	"errors"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func TestVideoPosterDayCoverAndGrid(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.DurationWindow(24*time.Hour))
	with := e.post(circle.ID, "owner", "ролик", "2026-10-04", e.at(0))
	without := e.post(circle.ID, "owner", "без кадра", "2026-10-03", e.after(time.Minute))
	for _, id := range []string{"blob-video", "blob-poster", "blob-bare"} {
		e.seedBlob(id, "owner")
	}
	if err := e.ch.AttachMedia(e.ctx, with.ID, []chronicle.MediaInput{{
		BlobID: "blob-photo", Kind: chronicle.MediaPhoto, VideoPosterBlobID: "blob-poster",
	}}); !errors.Is(err, chronicle.ErrInvalid) {
		t.Fatalf("poster on a photo: %v", err)
	}
	if err := e.ch.AttachMedia(e.ctx, with.ID, []chronicle.MediaInput{{
		BlobID: "blob-video", Kind: chronicle.MediaVideo, IsCover: true, VideoPosterBlobID: "blob-poster",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := e.ch.AttachMedia(e.ctx, without.ID, []chronicle.MediaInput{{
		BlobID: "blob-bare", Kind: chronicle.MediaVideo, IsCover: true,
	}}); err != nil {
		t.Fatal(err)
	}

	days, err := e.ch.DaysSnapshot(e.ctx, circle.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	byDate := map[string]chronicle.DaySummary{}
	for _, day := range days {
		byDate[day.Day.EntryDate] = day
	}
	got := byDate["2026-10-04"]
	if got.FallbackCoverBlobID != "blob-video" || got.CoverImageBlobID != "blob-poster" || got.Day.CoverBlobID != "" {
		t.Fatalf("day with poster: %+v", got)
	}
	bare := byDate["2026-10-03"]
	if !got.CoverIsVideo || !bare.CoverIsVideo || bare.CoverImageBlobID != "" {
		t.Fatalf("video flags: %+v / %+v", got, bare)
	}
	if bare.FallbackCoverBlobID != "blob-bare" {
		t.Fatalf("day without poster: %+v", bare)
	}

	if err := e.ch.SetDayCover(e.ctx, chronicle.DayCoverInput{
		CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-10-04",
		PostID: with.ID, BlobID: "blob-video", Now: e.at(0),
	}); err != nil {
		t.Fatal(err)
	}
	days, err = e.ch.DaysSnapshot(e.ctx, circle.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	for _, day := range days {
		if day.Day.EntryDate != "2026-10-04" {
			continue
		}
		if day.Day.CoverBlobID != "blob-video" || day.CoverImageBlobID != "blob-poster" {
			t.Fatalf("chosen video cover: %+v", day)
		}
	}

	items, _, err := e.ch.GridPage(e.ctx, circle.ID, "owner", nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]chronicle.GridItem{}
	for _, item := range items {
		seen[item.PostID] = item
	}
	if seen[with.ID].BlobID != "blob-video" || seen[with.ID].Kind != "video" || seen[with.ID].PosterBlobID != "blob-poster" {
		t.Fatalf("grid poster: %+v", seen[with.ID])
	}
	if seen[without.ID].BlobID != "blob-bare" || seen[without.ID].Kind != "video" || seen[without.ID].PosterBlobID != "" {
		t.Fatalf("grid bare video: %+v", seen[without.ID])
	}
}
