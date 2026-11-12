package archive_test

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/archive"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Звук в архиве выглядит как в приложении (2026-11-12): голосовое — круглая
// кнопка, волна и длительность (4.16), звук-файл — обложка, название и полоса
// хода (4.6/4.7). Раньше стоял нативный <audio controls>, и архив выглядел
// иначе; нативный плеер остался только запасным путём без JS.
func TestArchiveAudioMatchesApp(t *testing.T) {
	blobs, cleanup := openBlobStore(t)
	defer cleanup()
	voice, err := blobs.PutGenerated(t.Context(), "acc1", "audio/webm", "voice.webm", []byte("webm"))
	if err != nil {
		t.Fatal(err)
	}
	track, err := blobs.PutGenerated(t.Context(), "acc1", "audio/mpeg", "track.mp3", []byte("mp3"))
	if err != nil {
		t.Fatal(err)
	}
	cover, err := blobs.PutGenerated(t.Context(), "acc1", "image/jpeg", "cover.jpg", []byte("jpeg"))
	if err != nil {
		t.Fatal(err)
	}
	in := archive.BuildInput{
		CircleID:   "circle1",
		CircleName: "Семья",
		CutoffDate: "2026-08-10",
		Color:      "olive",
		Posts: []chronicle.FeedPost{{
			Post: chronicle.Post{ID: "post1", IdentityID: "anya", AuthorName: "Аня", Body: "слушай",
				EntryDate: "2026-08-09", CreatedAt: time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)},
			Media: []chronicle.PostMedia{
				{BlobID: voice.ID, Kind: chronicle.MediaAttachment, MimeType: "audio/webm",
					OriginalFilename: "voice.webm", Voice: true,
					AudioDurationMs: 7400, AudioPeaks: []int{12, 40, 90, 55}},
				{BlobID: track.ID, Kind: chronicle.MediaAttachment, MimeType: "audio/mpeg",
					OriginalFilename: "track.mp3", AudioCoverBlobID: cover.ID,
					AudioArtist: "Аня", AudioTitle: "Утро"},
			},
		}},
		Blobs: blobs,
	}
	var buf bytes.Buffer
	if err := archive.BuildPersonalArchive(t.Context(), &buf, in); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var index string
	for _, f := range zr.File {
		if f.Name != "index.html" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(rc)
		rc.Close()
		index = string(raw)
	}
	if index == "" {
		t.Fatal("нет index.html")
	}

	// Голосовое: волна из уровней записи и длительность.
	if !strings.Contains(index, `<div class="voice" data-ms="7400">`) {
		t.Fatalf("голосовое без строки плеера: %s", index)
	}
	if got := strings.Count(index, `<i style="height:`); got != 4 {
		t.Fatalf("волна: %d полос, ожидалось 4", got)
	}
	if !strings.Contains(index, `<span class="sz">0:07</span>`) {
		t.Fatal("нет длительности голосового")
	}

	// Звук-файл: обложка, «Исполнитель — Название», полоса хода, скачивание.
	if !strings.Contains(index, `<div class="att" data-ms="0">`) {
		t.Fatalf("звук без строки плеера: %s", index)
	}
	if !strings.Contains(index, `<span class="att-cover"><img alt="" src="`) {
		t.Fatal("нет обложки звука")
	}
	if !strings.Contains(index, `<span class="att-name">Аня — Утро</span>`) {
		t.Fatal("нет названия звука")
	}
	if !strings.Contains(index, `<span class="att-bar"><i></i></span>`) {
		t.Fatal("нет полосы хода")
	}
	if !strings.Contains(index, `<a class="dl" href="`) || !strings.Contains(index, `download aria-label="Скачать"`) {
		t.Fatal("нет кнопки скачивания")
	}

	// Один плеер на страницу; нативный <audio> — только запасной путь.
	if !strings.Contains(index, `var el=new Audio()`) {
		t.Fatal("нет скрипта плеера")
	}
	native := strings.Count(index, "<audio controls")
	if native != 2 {
		t.Fatalf("нативных плееров %d, ожидалось 2 в <noscript>", native)
	}
	for _, part := range strings.Split(index, "<noscript>")[1:] {
		if !strings.HasPrefix(part, "<audio controls") {
			t.Fatalf("нативный плеер вне <noscript>: %s", part[:60])
		}
	}
}
