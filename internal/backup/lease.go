package backup

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// pinBlobFiles also works with pre-15 databases. A lease is renewed until the
// final file pass finishes; a crashed backup stops delaying GC after one hour.
func pinBlobFiles(path string) (func() error, error) {
	noop := func() error { return nil }
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return noop, nil
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?_pragma=busy_timeout(5000)&_txlock=immediate")
	if err != nil {
		return nil, err
	}
	fail := func(e error) (func() error, error) { db.Close(); return nil, e }
	var exists int
	if err = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='blob_backup_leases'`).Scan(&exists); err != nil {
		return fail(err)
	}
	if exists == 0 {
		db.Close()
		return noop, nil
	}
	var id [16]byte
	if _, err = rand.Read(id[:]); err != nil {
		return fail(err)
	}
	token := fmt.Sprintf("%x", id)
	renew := func() error {
		_, err := db.Exec(`INSERT INTO blob_backup_leases(id,expires_at) VALUES(?,unixepoch()+3600) ON CONFLICT(id) DO UPDATE SET expires_at=excluded.expires_at`, token)
		return err
	}
	if err = renew(); err != nil {
		return fail(err)
	}
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		var first error
		for {
			select {
			case <-stop:
				done <- first
				return
			case <-ticker.C:
				if e := renew(); e != nil && first == nil {
					first = e
				}
			}
		}
	}()
	return func() error {
		close(stop)
		err := <-done
		_, e := db.Exec(`DELETE FROM blob_backup_leases WHERE id=?`, token)
		db.Close()
		if err != nil {
			return err
		}
		return e
	}, nil
}
