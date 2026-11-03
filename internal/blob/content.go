package blob

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func contentPath(sum string) string { return "content/" + sum[:2] + "/" + sum }

func validDigest(sum string) bool {
	b, err := hex.DecodeString(sum)
	return err == nil && len(b) == 32 && sum == strings.ToLower(sum)
}

func diskPath(dir, rel string) (string, error) {
	if !filepath.IsLocal(filepath.FromSlash(rel)) || filepath.Clean(filepath.FromSlash(rel)) == "." || strings.Contains(rel, "\\") {
		return "", fmt.Errorf("blob: unsafe storage path %q", rel)
	}
	return filepath.Join(dir, filepath.FromSlash(rel)), nil
}

func verifyFile(path, sum string, size int64) error {
	st, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() || st.Size() != size {
		return ErrInvalid
	}
	got, err := fileSHA256(path)
	if err != nil {
		return err
	}
	if got != sum {
		return ErrInvalid
	}
	return nil
}

// installContent runs under SQLite's IMMEDIATE write lock, shared with file GC.
// The source survives until the metadata commit. Existing content is immutable.
func installContent(ctx context.Context, tx *sql.Tx, dir, src, sum string, size int64, at string) (string, error) {
	if !validDigest(sum) {
		return "", ErrInvalid
	}
	rel := contentPath(sum)
	dest, err := diskPath(dir, rel)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return "", err
	}
	installed := false
	if _, err = os.Lstat(dest); errors.Is(err, os.ErrNotExist) {
		installed = true
		if err = os.Link(src, dest); err != nil {
			// Exclusive creation never truncates a file another logical copy uses.
			var in, out *os.File
			in, err = os.Open(src)
			if err != nil {
				return "", err
			}
			defer in.Close()
			if err = os.MkdirAll(filepath.Join(dir, ".uploads"), 0o750); err != nil {
				return "", err
			}
			out, err = os.CreateTemp(filepath.Join(dir, ".uploads"), "content-*.part")
			if err != nil {
				return "", err
			}
			temp := out.Name()
			defer os.Remove(temp)
			_, err = io.Copy(out, in)
			if err == nil {
				err = out.Sync()
			}
			cerr := out.Close()
			if err == nil {
				err = cerr
			}
			if err != nil {
				return "", err
			}
			if err = os.Rename(temp, dest); err != nil {
				return "", err
			}
		}
	} else if err != nil {
		return "", err
	}
	// Verify a reused file too: matching database metadata cannot establish that
	// its bytes survived a disk error. Never publish a new copy of damaged bytes.
	if !installed {
		err = verifyFile(dest, sum, size)
	} else {
		var st os.FileInfo
		st, err = os.Lstat(dest)
		if err == nil && (!st.Mode().IsRegular() || st.Size() != size) {
			err = ErrInvalid
		}
	}
	if err != nil {
		return "", fmt.Errorf("verify shared content: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO blob_contents(sha256,storage_path,size_bytes,created_at)
		VALUES(?,?,?,?) ON CONFLICT(sha256) DO NOTHING`, sum, rel, size, at)
	if err != nil {
		return "", err
	}
	var stored int64
	if err = tx.QueryRowContext(ctx, `SELECT size_bytes FROM blob_contents WHERE sha256=? AND storage_path=?`, sum, rel).Scan(&stored); err != nil {
		return "", err
	}
	if stored != size {
		return "", ErrInvalid
	}
	if installed && runtime.GOOS != "windows" {
		// Persist the new directory entry before SQLite can commit its reference.
		for _, p := range []string{filepath.Dir(dest), filepath.Join(dir, "content"), dir} {
			f, e := os.Open(p)
			if e != nil {
				return "", e
			}
			e = f.Sync()
			f.Close()
			if e != nil {
				return "", e
			}
		}
	}
	return rel, nil
}

// publishVerified is called only after the complete input bytes were hashed.
func (s *Store) publishVerified(ctx context.Context, b Blob, part, sessionID string, enforceQuota bool) (Blob, error) {
	var quota int64
	var err error
	if enforceQuota {
		quota, err = s.effectiveInstanceQuotaBytes(ctx)
		if err != nil {
			return Blob{}, err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Blob{}, err
	}
	defer tx.Rollback()
	if enforceQuota {
		var used, reserved, existing int64
		if err = tx.QueryRowContext(ctx, physicalUsedSQL).Scan(&used); err != nil {
			return Blob{}, err
		}
		if err = tx.QueryRowContext(ctx, `SELECT (SELECT COALESCE(SUM(received_bytes),0) FROM upload_sessions WHERE id<>?)+(SELECT COALESCE(SUM(size_bytes),0) FROM blobs WHERE status='pending')`, sessionID).Scan(&reserved); err != nil {
			return Blob{}, err
		}
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM blob_contents WHERE sha256=?`, b.SHA256).Scan(&existing); err != nil {
			return Blob{}, err
		}
		extra := b.SizeBytes
		if existing > 0 {
			extra = 0
		}
		if used+reserved+extra > quota {
			return Blob{}, ErrInstanceQuotaExceeded
		}
	}
	if sessionID != "" {
		var n int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM upload_sessions WHERE id=? AND account_id=? AND received_bytes=expected_size`, sessionID, b.AccountID).Scan(&n); err != nil {
			return Blob{}, err
		}
		if n != 1 {
			return Blob{}, ErrNotFound
		}
	}
	rel, err := installContent(ctx, tx, s.dir, part, b.SHA256, b.SizeBytes, formatTime(b.CreatedAt))
	if err != nil {
		return Blob{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO blobs(id,account_id,sha256,size_bytes,mime_type,original_filename,storage_path,status,created_at,content_sha256)
		VALUES(?,?,?,?,?,?,?,'complete',?,?)`, b.ID, b.AccountID, b.SHA256, b.SizeBytes, b.MimeType, nullableString(b.OriginalFilename), rel, formatTime(b.CreatedAt), b.SHA256)
	if err != nil {
		return Blob{}, err
	}
	if sessionID != "" {
		if _, err = tx.ExecContext(ctx, `DELETE FROM upload_sessions WHERE id=?`, sessionID); err != nil {
			return Blob{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return Blob{}, err
	}
	_ = os.Remove(part)
	return b, nil
}

// migrateContents verifies legacy files before sharing them. Missing/corrupt old
// files stay untouched and are reported; a restart resumes each unfinished group.
func (s *Store) migrateContents(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT lower(sha256),size_bytes FROM blobs WHERE content_sha256 IS NULL AND storage_path<>'' AND status='complete'`)
	if err != nil {
		return err
	}
	type group struct {
		sum  string
		size int64
	}
	var groups []group
	for rows.Next() {
		var g group
		if err = rows.Scan(&g.sum, &g.size); err != nil {
			rows.Close()
			return err
		}
		groups = append(groups, g)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, g := range groups {
		if !validDigest(g.sum) {
			log.Printf("blob migration: invalid legacy digest; file left in place")
			continue
		}
		if err = s.migrateContent(ctx, g.sum, g.size); err != nil {
			return err
		}
	}
	if err = s.queueInterruptedContents(ctx); err != nil {
		return err
	}
	return CollectFiles(ctx, s.db, s.dir)
}

// A process can die after installing bytes but before committing the registry.
// Only canonical paths are inspected, under the same lock used by publication.
func (s *Store) queueInterruptedContents(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	err = filepath.WalkDir(filepath.Join(s.dir, "content"), func(path string, d fs.DirEntry, e error) error {
		if errors.Is(e, os.ErrNotExist) {
			return nil
		}
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		if !validDigest(d.Name()) {
			return nil
		}
		rel, _ := filepath.Rel(s.dir, path)
		rel = filepath.ToSlash(rel)
		if rel != contentPath(d.Name()) {
			return nil
		}
		_, e = tx.ExecContext(ctx, `INSERT OR IGNORE INTO blob_file_gc SELECT ? WHERE NOT EXISTS(SELECT 1 FROM blob_contents WHERE storage_path=?)`, rel, rel)
		return e
	})
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) migrateContent(ctx context.Context, sum string, size int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT storage_path FROM blobs WHERE lower(sha256)=? AND size_bytes=? AND content_sha256 IS NULL AND storage_path<>'' AND status='complete'`, sum, size)
	if err != nil {
		return err
	}
	var paths []string
	for rows.Next() {
		var p string
		if err = rows.Scan(&p); err != nil {
			rows.Close()
			return err
		}
		paths = append(paths, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	src := ""
	for _, rel := range append([]string{contentPath(sum)}, paths...) {
		p, e := diskPath(s.dir, rel)
		if e == nil && verifyFile(p, sum, size) == nil {
			src = p
			break
		}
	}
	if src == "" {
		log.Print("blob migration: no verified bytes; legacy paths retained")
		return nil
	}
	rel, err := installContent(ctx, tx, s.dir, src, sum, size, formatTime(utcOrNow(time.Time{})))
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE blobs SET sha256=?,content_sha256=?,storage_path=? WHERE lower(sha256)=? AND size_bytes=? AND storage_path<>'' AND status='complete'`, sum, sum, rel, sum, size); err != nil {
		return err
	}
	for _, p := range paths {
		if p != rel {
			if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO blob_file_gc VALUES(?)`, p); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// DetachFile is used inside the transaction removing the last logical reference
// to a payment screenshot. Its audit row stays, but no longer grants file access.
func DetachFile(ctx context.Context, tx *sql.Tx, id string) error {
	needed, err := IsFileNeeded(ctx, tx, id)
	if err != nil || needed {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO blob_file_gc SELECT storage_path FROM blobs WHERE id=? AND storage_path<>''`, id); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE blobs SET storage_path='',content_sha256=NULL WHERE id=?`, id)
	return err
}

// DeleteUnreferenced atomically rechecks references, detaches a logical copy and
// durably queues its path. Physical collection happens only after that commit.
func DeleteUnreferenced(ctx context.Context, db *sql.DB, dir, id string) (bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	needed, err := IsReferenced(ctx, tx, id)
	if err != nil || needed {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO blob_file_gc SELECT storage_path FROM blobs WHERE id=? AND storage_path<>''`, id); err != nil {
		return false, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM blobs WHERE id=?`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return n > 0, CollectFiles(ctx, db, dir)
}

// CollectFiles serializes unlink with publication. Failed unlinks remain queued
// (notably an open file on Windows); retries never remove a referenced path.
func CollectFiles(ctx context.Context, db *sql.DB, dir string) error {
	if dir == "" {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var leases int
	if _, err = tx.ExecContext(ctx, `DELETE FROM blob_backup_leases WHERE expires_at<=unixepoch()`); err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM blob_backup_leases WHERE expires_at>unixepoch()`).Scan(&leases); err != nil {
		return err
	}
	if leases > 0 {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT storage_path FROM blob_file_gc UNION SELECT storage_path FROM blob_contents WHERE NOT EXISTS(SELECT 1 FROM blobs WHERE content_sha256=blob_contents.sha256)`)
	if err != nil {
		return err
	}
	var paths []string
	for rows.Next() {
		var p string
		if err = rows.Scan(&p); err != nil {
			rows.Close()
			return err
		}
		paths = append(paths, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, rel := range paths {
		var n int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM blobs WHERE storage_path=?`, rel).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		p, e := diskPath(dir, rel)
		if e != nil {
			return e
		}
		if e = os.Remove(p); e != nil && !errors.Is(e, os.ErrNotExist) {
			continue
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM blob_contents WHERE storage_path=? AND NOT EXISTS(SELECT 1 FROM blobs WHERE content_sha256=blob_contents.sha256)`, rel); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM blob_file_gc WHERE storage_path=?`, rel); err != nil {
			return err
		}
	}
	return tx.Commit()
}
