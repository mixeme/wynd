package store

import "testing"

func TestSharedContentsMigrationKeepsLogicalMetadata(t *testing.T) {
	s := baseAt(t, 14)
	if _, err := s.db.Exec(`INSERT INTO accounts(id,email,created_at) VALUES('a','a@test.local','2026-11-01T00:00:00Z');
 INSERT INTO blobs(id,account_id,sha256,size_bytes,mime_type,original_filename,storage_path,status,created_at) VALUES('old','a','legacy-sha',17,'image/png','own.png','ol/old','complete','2026-11-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(s.db); err != nil {
		t.Fatal(err)
	}
	var owner, name, path, status string
	var size int64
	if err := s.db.QueryRow(`SELECT account_id,original_filename,storage_path,status,size_bytes FROM blobs WHERE id='old' AND content_sha256 IS NULL`).Scan(&owner, &name, &path, &status, &size); err != nil {
		t.Fatal(err)
	}
	if owner != "a" || name != "own.png" || path != "ol/old" || status != "complete" || size != 17 {
		t.Fatal("logical metadata changed")
	}
	if err := migrate(s.db); err != nil {
		t.Fatal("repeat migration", err)
	}
}
