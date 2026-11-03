package store

import "testing"

// A21: an upgrade leaves already delivered archive bytes and their counters
// addressable by the original id; new transfers start without a legacy file.
func TestMoveSnapshotMigrationFrom13(t *testing.T) {
	s := baseAt(t, 13)
	_, err := s.db.Exec(`INSERT INTO accounts(id,email,created_at) VALUES ('owner','owner@example.com','2026-11-03T00:00:00Z');
		INSERT INTO circles(id,name,owner_account_id,created_at,updated_at) VALUES ('c','Circle','owner','2026-11-03T00:00:00Z','2026-11-03T00:00:00Z');
		INSERT INTO circle_moves(id,circle_id,started_by_account_id,started_at,token,token_hash,expires_at,status,archive_status,archive_bytes,archive_posts,archive_names)
		VALUES ('m','c','owner','2026-11-03T00:00:00Z','token','hash','2026-11-06T00:00:00Z','pulling','ready',12345,3,2)`)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate(s.db); err != nil {
		t.Fatal(err)
	}
	var legacy, status string
	var snapshot *string
	var size int64
	var posts, names int
	err = s.db.QueryRow(`SELECT legacy_archive_id,snapshot_id,archive_status,archive_bytes,archive_posts,archive_names FROM circle_moves WHERE id='m'`).Scan(&legacy, &snapshot, &status, &size, &posts, &names)
	if err != nil {
		t.Fatal(err)
	}
	if legacy != "m" || snapshot != nil || status != "ready" || size != 12345 || posts != 3 || names != 2 {
		t.Fatalf("changed existing transfer: %s %v %s %d %d %d", legacy, snapshot, status, size, posts, names)
	}
}
