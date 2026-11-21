package store

import "testing"

// Индексы проверки квоты (план 60, раздел 8) несёт базовая схема (C115).
// Повторный запуск мигратора — no-op.
func TestQuotaIndexesMigration(t *testing.T) {
	s := freshBase(t)
	want := []string{"idx_comments_circle", "idx_blobs_pending_size", "idx_blob_contents_size"}
	for _, name := range want {
		var n int
		if err := s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='index' AND name=?`, name).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("%s нет в базовой схеме", name)
		}
	}
	if err := migrate(s.db); err != nil {
		t.Fatal("repeat migration", err)
	}
}
