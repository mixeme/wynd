package blob_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

func TestAudioCoverStaysWithThePost(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ch, err := chronicle.New(st)
	if err != nil {
		t.Fatal(err)
	}
	s, err := blob.New(st, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	seedAccount(t, st, "owner")
	seedAccount(t, st, "bob")
	seedAccount(t, st, "cara")

	t0 := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	circle, _, _, err := ch.CreateCircle(ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня", Now: t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ch.Join(ctx, chronicle.JoinInput{
		CircleID: circle.ID, AccountID: "bob", Name: "Боб", Now: t0,
	}); err != nil {
		t.Fatal(err)
	}
	audio := uploadComplete(t, s, "owner", "audio/mp4", []byte("audio-bytes"))
	cover := uploadComplete(t, s, "owner", "image/jpeg", []byte("cover-bytes"))
	post, err := ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "утро",
		EntryDate: "2026-08-30", Now: t0.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ch.AttachMedia(ctx, post.ID, []chronicle.MediaInput{{
		BlobID: audio.ID, Kind: chronicle.MediaPhoto, AudioArtist: "Бригада",
	}}); !errors.Is(err, chronicle.ErrInvalid) {
		t.Fatalf("tags on a photo: %v", err)
	}
	long := strings.Repeat("я", chronicle.MaxAudioTagChars+1)
	if err := ch.AttachMedia(ctx, post.ID, []chronicle.MediaInput{{
		BlobID: audio.ID, Kind: chronicle.MediaAttachment, AudioTitle: long,
	}}); !errors.Is(err, chronicle.ErrTooLong) {
		t.Fatalf("long title: %v", err)
	}
	if err := ch.AttachMedia(ctx, post.ID, []chronicle.MediaInput{{
		BlobID: audio.ID, Kind: chronicle.MediaAttachment,
		AudioArtist: "Бригада", AudioTitle: "Утро", AudioCoverBlobID: cover.ID,
	}}); err != nil {
		t.Fatal(err)
	}

	items, err := ch.ListPostMedia(ctx, post.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("media: %d", len(items))
	}
	got := items[0]
	if got.AudioArtist != "Бригада" || got.AudioTitle != "Утро" || got.AudioCoverBlobID != cover.ID {
		t.Fatalf("tags: %+v", got)
	}
	if got.MimeType != "audio/mp4" {
		t.Fatalf("mime: %q", got.MimeType)
	}

	feed, err := ch.FeedSnapshot(ctx, circle.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if len(feed) != 1 || feed[0].Media[0].AudioTitle != "Утро" {
		t.Fatalf("feed tags: %+v", feed)
	}

	ok, err := s.CanAccessBlob(ctx, "cara", cover.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("stranger can read the cover")
	}
	ok, err = s.CanAccessBlob(ctx, "bob", cover.ID)
	if err != nil || !ok {
		t.Fatalf("member cover access: %v %v", ok, err)
	}

	ref, err := blob.IsReferenced(ctx, s.DB(), cover.ID)
	if err != nil || !ref {
		t.Fatalf("cover referenced: %v %v", ref, err)
	}
	chart, _, err := ch.MediaVolumeChart(ctx, circle.ID)
	if err != nil {
		t.Fatal(err)
	}
	var bytes int64
	for _, bucket := range chart {
		bytes += bucket.Bytes
	}
	want := int64(len("audio-bytes") + len("cover-bytes"))
	if bytes != want {
		t.Fatalf("volume: got %d want %d", bytes, want)
	}

	if err := ch.DeletePostMedia(ctx, post.ID); err != nil {
		t.Fatal(err)
	}
	ref, err = blob.IsReferenced(ctx, s.DB(), cover.ID)
	if err != nil || ref {
		t.Fatalf("cover still referenced after the row is gone: %v %v", ref, err)
	}
}
