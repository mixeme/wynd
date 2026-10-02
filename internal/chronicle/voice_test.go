package chronicle_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Голосовое (C14) хранит признак, длительность и волну; чужие поля и
// плохая волна отклоняются.
func TestVoiceMediaRoundTrip(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.DurationWindow(24*time.Hour))
	p := e.post(circle.ID, "owner", "голос", "2026-08-01", e.at(0))
	e.seedBlob("blob-voice", "owner")
	e.seedBlob("blob-photo", "owner")
	peaks := []int{0, 12, 100, 40}
	if err := e.ch.AttachMedia(e.ctx, p.ID, []chronicle.MediaInput{{
		BlobID: "blob-voice", Kind: chronicle.MediaAttachment,
		Voice: true, AudioDurationMs: 48_000, AudioPeaks: peaks,
	}}); err != nil {
		t.Fatalf("attach voice: %v", err)
	}
	media, err := e.ch.ListPostMedia(e.ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	s := chronicle.SummarizeMedia(media)
	if len(s) != 1 || !s[0].Voice || s[0].AudioDurationMs != 48_000 || len(s[0].AudioPeaks) != 4 || s[0].AudioPeaks[2] != 100 {
		t.Fatalf("summary = %+v", s)
	}

	bad := []chronicle.MediaInput{
		{BlobID: "blob-photo", Kind: chronicle.MediaPhoto, Voice: true},
		{BlobID: "blob-photo", Kind: chronicle.MediaAttachment, Voice: true, AudioPeaks: []int{101}},
		{BlobID: "blob-photo", Kind: chronicle.MediaAttachment, Voice: true, AudioDurationMs: chronicle.MaxVoiceDurationMs + 1},
	}
	p2 := e.post(circle.ID, "owner", "x", "2026-08-01", e.at(0))
	for i, item := range bad {
		if err := e.ch.AttachMedia(e.ctx, p2.ID, []chronicle.MediaInput{item}); err != chronicle.ErrInvalid {
			t.Fatalf("bad %d: got %v", i, err)
		}
	}
}
