package chronicle_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Обложка звука годится в обложку дня, но не в обложку записи.
func TestDayCoverFromAudioCover(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.DurationWindow(24*time.Hour))
	p := e.post(circle.ID, "owner", "песня", "2026-08-01", e.at(0))
	e.seedBlob("blob-audio", "owner")
	e.seedBlob("blob-art", "owner")
	e.seedBlob("blob-file", "owner")
	if err := e.ch.AttachMedia(e.ctx, p.ID, []chronicle.MediaInput{
		{BlobID: "blob-audio", Kind: chronicle.MediaAttachment, AudioCoverBlobID: "blob-art"},
		{BlobID: "blob-file", Kind: chronicle.MediaAttachment},
	}); err != nil {
		t.Fatalf("attach: %v", err)
	}

	set := func(blob string) error {
		return e.ch.SetDayCover(e.ctx, chronicle.DayCoverInput{
			CircleID: circle.ID, AccountID: "owner", EntryDate: "2026-08-01",
			PostID: p.ID, BlobID: blob, Now: e.at(0),
		})
	}
	if err := set("blob-art"); err != nil {
		t.Fatalf("audio cover as day cover: %v", err)
	}
	if err := set("blob-audio"); err != chronicle.ErrInvalid {
		t.Fatalf("audio itself: got %v", err)
	}
	if err := set("blob-file"); err != chronicle.ErrInvalid {
		t.Fatalf("attachment: got %v", err)
	}
	if err := e.ch.SetPostCover(e.ctx, circle.ID, "owner", p.ID, "blob-art", e.at(0)); err != chronicle.ErrInvalid {
		t.Fatalf("audio cover as post cover: got %v", err)
	}
}
