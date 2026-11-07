package blob

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/store"
)

// Бенчмарк проверки квоты (план 60, раздел 8). CheckMediaQuota,
// CheckWriteQuota и ReserveWrite перед решением гоняют агрегаты по
// blob_contents, blobs (дважды: реестр и pending), circles, архивам,
// снапшотам переезда и резервациям плюс четыре подзапроса круга. Здесь
// меряется стоимость проверки на репрезентативных объёмах, а
// TestQuotaQueryPlans снимает EXPLAIN QUERY PLAN. Объединять запросы или
// нет — решение по этим числам; код проверки не меняется.
//
// Запуск: go test ./internal/blob -run '^$' -bench 'BenchmarkQuota' -benchmem
// Планы:  go test ./internal/blob -run TestQuotaQueryPlans -v

type quotaScale struct {
	name    string
	blobs   int // завершённых строк blobs
	pending int
	posts   int
}

var quotaScales = []quotaScale{
	{"год-6тыс-файлов", 6000, 120, 3000},
	{"годы-60тыс-файлов", 60000, 1200, 30000},
}

func circleOf(i int) string {
	if i%2 == 1 {
		return "c2"
	}
	return "c1"
}

func quotaBlobSize(i int) int64 {
	return int64(1<<20 + (i%7)<<20) // 1–7 МБ, в среднем ~4
}

// seedQuotaStore засевает базу raw SQL в порядке внешних ключей: две трети
// блобов делят содержимое через blob_contents (по два блоба на файл), у
// остальных свой storage_path; по три фото на запись, две пары комментариев
// на запись, треть комментариев с вложением, аватары, открытые сессии
// загрузки, живые и просроченные резервации, два архива круга и снапшот
// переезда. Файлы на диск не пишутся: агрегаты квоты читают только базу.
func seedQuotaStore(tb testing.TB, sc quotaScale) *Store {
	tb.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		tb.Fatal(err)
	}
	s, err := New(st, filepath.Join(tb.TempDir(), "blobs"))
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = st.Close() })
	ctx := tb.Context()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		tb.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	exec := func(q string, args ...any) {
		tb.Helper()
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			tb.Fatalf("%s: %v", q, err)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	stage := func(name string) func() {
		start := time.Now()
		return func() { tb.Logf("засев %s: %s", name, time.Since(start).Round(time.Millisecond)) }
	}

	s0 := stage("учётки и круги")
	for i := 1; i <= 4; i++ {
		exec(`INSERT INTO accounts (id, email, created_at) VALUES (?, ?, ?)`,
			fmt.Sprintf("acc%d", i), fmt.Sprintf("acc%d@test.local", i), now)
	}
	// Потолки заведомо выше засеянного, чтобы проверка дошла до конца.
	// Процент диска сбрасываем: он переопределяет абсолютный потолок, а на
	// машине прогона это случайный размер тома.
	exec(`UPDATE instance_settings SET storage_quota_bytes = ?, storage_quota_disk_percent = NULL, default_circle_quota_bytes = NULL WHERE id = 1`, int64(1)<<42)
	exec(`INSERT INTO circles (id, name, owner_account_id, created_at, updated_at, quota_custom, quota_bytes, text_bytes)
		VALUES ('c1', 'Семья', 'acc1', ?, ?, 1, ?, ?)`, now, now, int64(1)<<40, int64(8<<20))
	exec(`INSERT INTO circles (id, name, owner_account_id, created_at, updated_at, text_bytes)
		VALUES ('c2', 'Дача', 'acc2', ?, ?, ?)`, now, now, int64(4<<20))
	for _, cid := range []string{"c1", "c2"} {
		for i := 0; i < 4; i++ {
			exec(`INSERT INTO identities (id, circle_id, account_id, created_at) VALUES (?, ?, ?, ?)`,
				fmt.Sprintf("id-%s-%d", cid, i), cid, fmt.Sprintf("acc%d", i+1), now)
		}
	}
	s0()
	s1 := stage("строки: блобы, записи, комментарии")

	contentOf := func(i int) (sha, path string) {
		if i%5 < 3 {
			return fmt.Sprintf("%064d", i/2), fmt.Sprintf("s/%08d", i/2)
		}
		return "", fmt.Sprintf("o/%08d", i)
	}
	// modernc/sqlite дорого пересекает границу драйвера на каждый Exec, и
	// сотни тысяч строк по одной вставлялись бы минутами; копим multi-row
	// VALUES и сбрасываем пачками не больше ~900 параметров.
	batcher := func(prefix string, chunk int) (add func(vals ...any), done func()) {
		var sb strings.Builder
		var args []any
		n := 0
		flush := func() {
			if n == 0 {
				return
			}
			if _, err := tx.ExecContext(ctx, sb.String(), args...); err != nil {
				tb.Fatalf("batch %s…: %v", prefix, err)
			}
			sb.Reset()
			args = args[:0]
			n = 0
		}
		add = func(vals ...any) {
			if n == 0 {
				sb.WriteString(prefix)
			} else {
				sb.WriteByte(',')
			}
			sb.WriteByte('(')
			for i := range vals {
				if i > 0 {
					sb.WriteByte(',')
				}
				sb.WriteByte('?')
			}
			sb.WriteString(")")
			args = append(args, vals...)
			if n++; n >= chunk {
				flush()
			}
		}
		return add, flush
	}

	seen := map[string]bool{}
	addContent, doneContent := batcher(`INSERT INTO blob_contents (sha256, storage_path, size_bytes, created_at) VALUES `, 200)
	for i := 0; i < sc.blobs; i++ {
		sha, _ := contentOf(i)
		if sha == "" || seen[sha] {
			continue
		}
		seen[sha] = true
		addContent(sha, fmt.Sprintf("s/%08d", i/2), quotaBlobSize(i), now)
	}
	doneContent()
	addBlob, doneBlob := batcher(`INSERT INTO blobs (id, account_id, sha256, size_bytes, mime_type, storage_path, status, created_at, original_filename, content_sha256)
		VALUES `, 90)
	for i := 0; i < sc.blobs+sc.pending; i++ {
		sha, path := contentOf(i)
		var content any
		if i >= sc.blobs {
			sha, path, content = "", fmt.Sprintf(".uploads/part-%08d", i), nil // pending
		} else if sha != "" {
			content = sha
		}
		addBlob(fmt.Sprintf("bl%08d", i), fmt.Sprintf("acc%d", i%4+1), fmt.Sprintf("%064d", 1000000+i),
			quotaBlobSize(i), "image/jpeg", path, map[bool]string{true: "pending", false: "complete"}[i >= sc.blobs], now,
			"photo.jpg", content)
	}
	doneBlob()

	// Уровни зависимостей (events → posts/comments → медиа): пачки сбрасываются
	// по ходу цикла, поэтому вставляем послойно — строка никогда не опережает
	// строку, на которую ссылается.
	seq := 0
	addEvent, doneEvent := batcher(`INSERT INTO events (seq, id, circle_id, event_type, is_service, actor_name, summary, created_at)
		VALUES `, 200)
	for i := 0; i < sc.posts; i++ {
		seq++
		addEvent(seq, fmt.Sprintf("ev%08d", seq), circleOf(i), "post", 0, "Аня", "", now)
	}
	for i := 0; i < sc.posts; i++ {
		for c := 0; c < 2; c++ {
			seq++
			addEvent(seq, fmt.Sprintf("ev%08d", seq), circleOf(i), "comment", 0, "Боря", "", now)
		}
	}
	doneEvent()

	addPost, donePost := batcher(`INSERT INTO posts (id, circle_id, event_seq, identity_id, author_name, body, entry_date, created_at, deleted)
		VALUES `, 128)
	body := strings.Repeat("на даче тепло и тихо ", 10)
	for i := 0; i < sc.posts; i++ {
		deleted := 0
		if i%20 == 19 {
			deleted = 1
		}
		addPost(fmt.Sprintf("p%08d", i), circleOf(i), i+1, fmt.Sprintf("id-%s-%d", circleOf(i), i%4), "Аня", body, "2026-08-01", now, deleted)
	}
	donePost()

	addComment, doneComment := batcher(`INSERT INTO comments (id, circle_id, post_id, event_seq, identity_id, author_name, body, created_at)
		VALUES `, 128)
	for i := 0; i < sc.posts; i++ {
		for c := 0; c < 2; c++ {
			addComment(fmt.Sprintf("cm%08d-%d", i, c), circleOf(i), fmt.Sprintf("p%08d", i), sc.posts+2*i+c+1,
				fmt.Sprintf("id-%s-%d", circleOf(i), (i+c)%4), "Боря", fmt.Sprintf("комментарий %d к записи", i), now)
		}
	}
	doneComment()

	addPM, donePM := batcher(`INSERT INTO post_media (id, post_id, blob_id, kind, sort_order) VALUES `, 200)
	for i := 0; i < sc.posts; i++ {
		for m := 0; m < 3; m++ {
			addPM(fmt.Sprintf("pm%08d-%d", i, m), fmt.Sprintf("p%08d", i), fmt.Sprintf("bl%08d", (i*3+m)%sc.blobs), "photo", m)
		}
	}
	donePM()
	addCM, doneCM := batcher(`INSERT INTO comment_media (id, comment_id, blob_id, kind, sort_order) VALUES `, 250)
	for i := 0; i < sc.posts; i++ {
		for c := 0; c < 2; c++ {
			if (i+c)%3 == 0 {
				addCM(fmt.Sprintf("cx%08d-%d", i, c), fmt.Sprintf("cm%08d-%d", i, c), fmt.Sprintf("bl%08d", (i*7+c)%sc.blobs), "attachment", 0)
			}
		}
	}
	doneCM()
	for _, cid := range []string{"c1", "c2"} {
		for i := 0; i < 4; i++ {
			var avatar any
			if i%2 == 0 {
				avatar = fmt.Sprintf("bl%08d", (i+1)%sc.blobs)
			}
			exec(`INSERT INTO identity_names (id, identity_id, name, avatar_blob_id, effective_at) VALUES (?, ?, ?, ?, ?)`,
				fmt.Sprintf("nm-%s-%d", cid, i), fmt.Sprintf("id-%s-%d", cid, i), fmt.Sprintf("Имя %d", i), avatar, now)
		}
	}
	for i := 0; i < 10; i++ {
		exec(`INSERT INTO upload_sessions (id, account_id, expected_size, mime_type, received_bytes, expires_at, created_at)
			VALUES (?, 'acc1', ?, 'video/mp4', ?, ?, ?)`,
			fmt.Sprintf("us%02d", i), int64(50<<20), int64(5<<20), time.Now().Add(time.Hour).UTC().Format(time.RFC3339), now)
	}
	live := time.Now().UTC().Unix()
	for i := 0; i < 10; i++ {
		exec(`INSERT INTO quota_reservations (id, circle_id, instance_bytes, circle_bytes, created_at) VALUES (?, 'c2', ?, ?, ?)`,
			fmt.Sprintf("qr%02d", i), int64(1<<20), int64(1<<20), live)
	}
	for i := 0; i < 5; i++ {
		exec(`INSERT INTO quota_reservations (id, circle_id, instance_bytes, circle_bytes, created_at) VALUES (?, 'c2', ?, ?, ?)`,
			fmt.Sprintf("qx%02d", i), int64(1<<20), int64(1<<20), live-3600)
	}
	exec(`INSERT INTO circle_archives (id, circle_id, requested_by_account_id, requested_at, status, size_bytes)
		VALUES ('ar1', 'c2', 'acc2', ?, 'building', ?)`, now, int64(500<<20))
	exec(`INSERT INTO circle_archives (id, circle_id, requested_by_account_id, requested_at, status, size_bytes)
		VALUES ('ar2', 'c2', 'acc2', ?, 'ready', ?)`, now, int64(700<<20))
	exec(`INSERT INTO move_snapshots (id, circle_id, snapshot_at, schema_version, wynd_version, card, data, archive_size, posts, names, media_bytes)
		VALUES ('ms1', 'c2', ?, 1, '0.48.2', ?, ?, 0, 1, 1, 0)`, now, make([]byte, 64<<10), make([]byte, 512<<10))
	exec(`INSERT INTO move_snapshot_literals (snapshot_id, part, data) VALUES ('ms1', 0, ?)`, make([]byte, 256<<10))

	if err := tx.Commit(); err != nil {
		tb.Fatal(err)
	}
	s1()
	used, err := usedBytesQ(ctx, s.db)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Logf("%s: блобов %d (+%d pending), записей %d, занято %d ГБ",
		sc.name, sc.blobs, sc.pending, sc.posts, used>>30)
	return s
}

// BenchmarkQuotaCheck меряет публичные входы проверки: каждое медиа и
// каждая запись проходят их перед записью; ReserveWrite — полный путь R3
// с транзакцией, вставкой и освобождением резервации.
func BenchmarkQuotaCheck(b *testing.B) {
	for _, sc := range quotaScales {
		s := seedQuotaStore(b, sc)
		b.Run(sc.name+"/CheckMediaQuota", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if err := s.CheckMediaQuota(b.Context(), "c1", 1<<20); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(sc.name+"/CheckWriteQuota", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if err := s.CheckWriteQuota(b.Context(), "c1", 800); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(sc.name+"/ReserveWrite", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				r, err := s.ReserveWrite(b.Context(), "c1", 0, 800)
				if err != nil {
					b.Fatal(err)
				}
				if err := s.ReleaseReservation(b.Context(), r); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkQuotaParts раскладывает проверку на части, чтобы видно было,
// какой агрегат её держит.
func BenchmarkQuotaParts(b *testing.B) {
	for _, sc := range quotaScales {
		s := seedQuotaStore(b, sc)
		b.Run(sc.name+"/usedBytes", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := usedBytesQ(b.Context(), s.db); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(sc.name+"/reservedBytes", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := reservedBytesQ(b.Context(), s.db); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(sc.name+"/circleUsed", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := circleUsedBytesQ(b.Context(), s.db, "c1"); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(sc.name+"/circleReserved", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := circleReservedBytesQ(b.Context(), s.db, "c1"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// TestQuotaQueryPlans логирует планы EXPLAIN QUERY PLAN агрегатов квоты.
// ANALYZE в проекте не выполняется, статистики нет — планы планировщика от
// объёма не зависят, поэтому достаточно малого засева. Копии текста
// запросов сверяются с функциями по значению: разошлись — копия устарела.
func TestQuotaQueryPlans(t *testing.T) {
	s := seedQuotaStore(t, quotaScales[0])
	ctx := t.Context()
	cutoff := reservationCutoff()

	reservedSQL := `
		SELECT
			(SELECT COALESCE(SUM(received_bytes), 0) FROM upload_sessions) +
			(SELECT COALESCE(SUM(size_bytes), 0) FROM blobs WHERE status = 'pending') +
			(SELECT COALESCE(SUM(instance_bytes), 0) FROM quota_reservations WHERE created_at > ?2)
	`
	plans := []struct {
		name string
		sql  string
		args []any
		want func() (int64, error)
	}{
		{"usedBytes", physicalUsedSQL, nil, func() (int64, error) { return usedBytesQ(ctx, s.db) }},
		{"reservedBytes", reservedSQL, []any{"c1", cutoff}, func() (int64, error) { return reservedBytesQ(ctx, s.db) }},
		{"circleUsed", `SELECT ` + strings.ReplaceAll(circleUsedSQL("?"), "?", "?1"), []any{"c1"},
			func() (int64, error) { return circleUsedBytesQ(ctx, s.db, "c1") }},
		{"circleReserved", `SELECT COALESCE(SUM(circle_bytes), 0) FROM quota_reservations WHERE circle_id = ?1 AND created_at > ?2`,
			[]any{"c1", cutoff}, func() (int64, error) { return circleReservedBytesQ(ctx, s.db, "c1") }},
	}
	for _, p := range plans {
		var got int64
		if err := s.db.QueryRowContext(ctx, p.sql, p.args...).Scan(&got); err != nil {
			t.Fatalf("%s: %v", p.name, err)
		}
		want, err := p.want()
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s: копия запроса разошлась с функцией: %d против %d", p.name, got, want)
		}
		rows, err := s.db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+p.sql, p.args...)
		if err != nil {
			t.Fatalf("%s: explain: %v", p.name, err)
		}
		for rows.Next() {
			var id, parent, notused int
			var detail string
			if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
				t.Fatal(err)
			}
			t.Logf("[%s] %s", p.name, detail)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
	}
}
