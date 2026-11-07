package store

import "testing"

// Миграция 17 добавляет индексы проверки квоты; данные не меняются.
// Повторный запуск мигратора — no-op.
func TestQuotaIndexesMigration(t *testing.T) {
	s := baseAt(t, 16)
	want := []string{"idx_comments_circle", "idx_blobs_pending_size", "idx_blob_contents_size"}
	for _, name := range want {
		var n int
		if err := s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='index' AND name=?`, name).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("%s существует до миграции 17", name)
		}
	}
	if err := migrate(s.db); err != nil {
		t.Fatal(err)
	}
	for _, name := range want {
		var n int
		if err := s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='index' AND name=?`, name).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("%s не создана миграцией 17", name)
		}
	}
	if err := migrate(s.db); err != nil {
		t.Fatal("repeat migration", err)
	}
}
