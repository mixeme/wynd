package chronicle_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func TestSetPostCoverInvariant(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.DurationWindow(24*time.Hour))
	e.join(circle.ID, "guest", "Мышь", e.at(0))

	p := e.post(circle.ID, "owner", "текст", "2026-08-01", e.at(0))
	e.seedBlob("blob-a", "owner")
	e.seedBlob("blob-b", "owner")
	e.seedBlob("blob-att", "owner")
	e.attachPhoto(p.ID, "blob-a")
	if err := e.ch.AttachMedia(e.ctx, p.ID, []chronicle.MediaInput{{
		BlobID: "blob-b", Kind: chronicle.MediaPhoto,
	}}); err != nil {
		t.Fatalf("attach second photo: %v", err)
	}
	if err := e.ch.AttachMedia(e.ctx, p.ID, []chronicle.MediaInput{{
		BlobID: "blob-att", Kind: chronicle.MediaAttachment,
	}}); err != nil {
		t.Fatalf("attach attachment: %v", err)
	}

	if err := e.ch.SetPostCover(e.ctx, circle.ID, "owner", p.ID, "blob-b", e.at(0)); err != nil {
		t.Fatalf("SetPostCover: %v", err)
	}
	media, err := e.ch.ListPostMedia(e.ctx, p.ID)
	if err != nil {
		t.Fatalf("ListPostMedia: %v", err)
	}
	covers := 0
	for _, m := range media {
		if m.IsCover {
			covers++
			if m.BlobID != "blob-b" {
				t.Fatalf("cover blob = %s", m.BlobID)
			}
		}
	}
	if covers != 1 {
		t.Fatalf("cover count = %d", covers)
	}

	err = e.ch.SetPostCover(e.ctx, circle.ID, "owner", p.ID, "blob-att", e.at(0))
	if err != chronicle.ErrInvalid {
		t.Fatalf("attachment cover: got %v", err)
	}

	err = e.ch.SetPostCover(e.ctx, circle.ID, "guest", p.ID, "blob-a", e.at(0))
	if err != chronicle.ErrForbidden {
		t.Fatalf("foreign post: got %v", err)
	}
}
