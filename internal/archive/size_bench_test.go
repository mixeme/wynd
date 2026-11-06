package archive_test

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/archive"
	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

// Бенчмарк холостой сборки архива (план 60, раздел 8). Size и FullSize не
// читают медиа с диска, но прогоняют через ZIP-писатель нули объёмом всех
// файлов и по-настоящему сжимают HTML и JSON — это цена точного
// Content-Length до первого байта ответа. Здесь меряется, сколько такой
// проход стоит; рядом — настоящая сборка в io.Discard как база сравнения.
// Отсюда берутся только числа: выбор способа убрать холостой проход —
// отдельное решение после измерений.
//
// Запуск: go test ./internal/archive -run '^$' -bench BenchmarkArchivePass -benchmem

// benchCase — масштаб круга: сколько записей и какие медиа при каждой.
// Содержимое медиа нулевое: холостой проход его не читает, а CRC32 и
// заголовки от байтов не зависят.
type benchCase struct {
	name  string
	posts int
	media []benchMedia
}

type benchMedia struct {
	kind chronicle.MediaKind
	mime string
	name string
	size int64
}

var benchCases = []benchCase{
	{"8MB-16записей", 16, []benchMedia{
		{chronicle.MediaPhoto, "image/jpeg", "photo.jpg", 512 << 10},
	}},
	{"256MB-64записи", 64, []benchMedia{
		{chronicle.MediaVideo, "video/mp4", "video.mp4", 3500 << 10},
		{chronicle.MediaPhoto, "image/jpeg", "photo.jpg", 500 << 10},
	}},
	{"1GB-192записи", 192, []benchMedia{
		{chronicle.MediaVideo, "video/mp4", "video.mp4", 5 << 20},
		{chronicle.MediaPhoto, "image/jpeg", "photo.jpg", 256 << 10},
	}},
}

var benchAuthors = []string{"Аня", "Боря", "Вера", "Галя"}

func openBlobStoreB(b *testing.B) (*blob.Store, func()) {
	b.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		b.Fatal(err)
	}
	s := st.(*store.SQLite)
	_, err = s.DB().ExecContext(b.Context(), `
		INSERT OR IGNORE INTO accounts (id, email, created_at) VALUES ('acc1', 'acc1@test.local', '2026-08-30T00:00:00Z')
	`)
	if err != nil {
		b.Fatal(err)
	}
	blobs, err := blob.New(st, filepath.Join(b.TempDir(), "blobs"))
	if err != nil {
		b.Fatal(err)
	}
	return blobs, func() { _ = st.Close() }
}

func benchBody(i int) string {
	return fmt.Sprintf("Запись №%d. На даче прошёл тёплый дождь, пахнет мокрой листвой и пирогами; %s",
		i, strings.Repeat("свет лежит на полу веранды, за окном сад шумит сам с собой. ", 6))
}

func benchComment(i int) string {
	return fmt.Sprintf("Комментарий %d: как красиво получилось, этот день вспомнили сразу.", i)
}

// benchScenario собирает репрезентативный круг: настоящие blob-файлы на
// диске, кириллический текст записей, по два комментария и реакции, аватары
// и названия дней; полный архив добавляет те же данные таблицами JSON.
func benchScenario(b *testing.B, c benchCase) (personal archive.BuildInput, full archive.FullInput, personalBytes, fullBytes int64) {
	b.Helper()
	blobs, cleanup := openBlobStoreB(b)
	b.Cleanup(cleanup)
	ctx := b.Context()

	avatars := make(map[string]string, len(benchAuthors))
	for i := range benchAuthors {
		bl, err := blobs.PutGenerated(ctx, "acc1", "image/jpeg", "avatar.jpg", make([]byte, 32<<10))
		if err != nil {
			b.Fatal(err)
		}
		avatars[fmt.Sprintf("id%d", i)] = bl.ID
	}

	posts := make([]chronicle.FeedPost, 0, c.posts)
	postRows := make([][]any, 0, c.posts)
	commentRows := make([][]any, 0, 2*c.posts)
	reactionRows := make([][]any, 0, c.posts)
	dayTitles := make(map[string]string)
	base := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	for i := 0; i < c.posts; i++ {
		entry := base.AddDate(0, 0, i%60).Format("2006-01-02")
		dayTitles[entry] = fmt.Sprintf("День %d на даче", i%60+1)
		author := benchAuthors[i%len(benchAuthors)]
		identity := fmt.Sprintf("id%d", i%len(benchAuthors))
		created := base.Add(time.Duration(i) * time.Hour)
		media := make([]chronicle.PostMedia, 0, len(c.media))
		for _, m := range c.media {
			bl, err := blobs.PutGenerated(ctx, "acc1", m.mime, m.name, make([]byte, m.size))
			if err != nil {
					b.Fatal(err)
				}
				media = append(media, chronicle.PostMedia{BlobID: bl.ID, Kind: m.kind})
		}
		post := chronicle.Post{
			ID: fmt.Sprintf("p%d", i), IdentityID: identity, AuthorName: author,
			Body: benchBody(i), EntryDate: entry, CreatedAt: created,
		}
		comments := []chronicle.Comment{
			{ID: fmt.Sprintf("cm%d", 2*i), PostID: post.ID, IdentityID: identity,
				AuthorName: benchAuthors[(i+1)%len(benchAuthors)], Body: benchComment(2 * i), CreatedAt: created},
			{ID: fmt.Sprintf("cm%d", 2*i+1), PostID: post.ID, IdentityID: identity,
				AuthorName: benchAuthors[(i+2)%len(benchAuthors)], Body: benchComment(2*i + 1), CreatedAt: created},
		}
		emoji := "❤️"
		if i%3 == 0 {
			emoji = "👍"
		}
		reactions := []chronicle.Reaction{
			{ID: fmt.Sprintf("rc%d", i), PostID: post.ID, IdentityID: identity, AuthorName: author, Emoji: emoji, CreatedAt: created},
		}
		posts = append(posts, chronicle.FeedPost{Post: post, Media: media, Comments: comments, Reactions: reactions})

		postRows = append(postRows, []any{post.ID, identity, post.Body, entry, created.Format(time.RFC3339)})
		for _, cm := range comments {
			commentRows = append(commentRows, []any{cm.ID, post.ID, cm.IdentityID, cm.Body, cm.CreatedAt.Format(time.RFC3339)})
		}
		reactionRows = append(reactionRows, []any{fmt.Sprintf("rc%d", i), post.ID, identity, emoji, created.Format(time.RFC3339)})
	}

	personal = archive.BuildInput{
		CircleName: "Семья", CutoffDate: "2026-10-15", Layout: archive.LayoutFeed,
		Posts: posts, Blobs: blobs, Avatars: avatars, DayTitles: dayTitles, Color: "ochre",
	}
	full = archive.FullInput{
		Manifest: archive.FullManifest{WyndVersion: "0.48.2", SchemaVersion: 1, Server: "https://home.example.org"},
		Data: &chronicle.FullArchiveData{
			CircleID: "c1", CircleName: "Семья", Color: "ochre",
			SnapshotAt: time.Date(2026, 10, 15, 9, 30, 0, 0, time.UTC), Posts: c.posts,
			Tables: []chronicle.FullArchiveTable{
				{Name: "circles", Columns: []string{"id", "name", "color"}, Rows: [][]any{{"c1", "Семья", "ochre"}}},
				{Name: "accounts", Columns: []string{"id", "email", "name"}, Rows: [][]any{
					{"acc1", "acc1@test.local", "Аня"}, {"acc2", "acc2@test.local", "Боря"},
					{"acc3", "acc3@test.local", "Вера"}, {"acc4", "acc4@test.local", "Галя"},
				}},
				{Name: "posts", Columns: []string{"id", "identity_id", "body", "entry_date", "created_at"}, Rows: postRows},
				{Name: "comments", Columns: []string{"id", "post_id", "identity_id", "body", "created_at"}, Rows: commentRows},
				{Name: "reactions", Columns: []string{"id", "post_id", "identity_id", "emoji", "created_at"}, Rows: reactionRows},
			},
		},
		Readable: personal,
	}

	personalBytes, err := archive.Size(ctx, personal)
	if err != nil {
		b.Fatal(err)
	}
	fullBytes, err = archive.FullSize(ctx, full)
	if err != nil {
		b.Fatal(err)
	}
	b.Logf("%s: личный архив %d МБ, полный %d МБ, записей %d",
		c.name, personalBytes>>20, fullBytes>>20, c.posts)
	return personal, full, personalBytes, fullBytes
}

// BenchmarkArchivePass меряет четыре прохода на одном круге каждого масштаба:
// холостая и настоящая сборка личного архива и то же для полного. Настоящая
// пишется в io.Discard: сетевая отдача заменена выбрасыванием, диск и CPU —
// те же, что у сервера при скачивании.
func BenchmarkArchivePass(b *testing.B) {
	for _, c := range benchCases {
		personal, full, personalBytes, fullBytes := benchScenario(b, c)

		b.Run(c.name+"/личный/холостая", func(b *testing.B) {
			b.SetBytes(personalBytes)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := archive.Size(b.Context(), personal); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(c.name+"/личный/настоящая", func(b *testing.B) {
			b.SetBytes(personalBytes)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if err := archive.BuildPersonalArchive(b.Context(), io.Discard, personal); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(c.name+"/полный/холостая", func(b *testing.B) {
			b.SetBytes(fullBytes)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := archive.FullSize(b.Context(), full); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(c.name+"/полный/настоящая", func(b *testing.B) {
			b.SetBytes(fullBytes)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if err := archive.BuildFullArchive(b.Context(), io.Discard, full); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
