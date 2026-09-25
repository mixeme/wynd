package backup

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/xtime"
	_ "modernc.org/sqlite"
)

const manifestName = "manifest.json"

type blobManifest struct {
	Files map[string]blobEntry `json:"files"`
}

type blobEntry struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
}

// Backup copies config.json, wynd.db, keys/, and blob files into destDir.
// When incremental is true, only blob files absent from the destination manifest
// are copied; config, database, and keys are always refreshed.
func Backup(dataDir, destDir string, incremental bool) error {
	if dataDir == "" || destDir == "" {
		return fmt.Errorf("backup: empty path")
	}
	dataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return fmt.Errorf("backup: data dir: %w", err)
	}
	destDir, err = filepath.Abs(destDir)
	if err != nil {
		return fmt.Errorf("backup: dest dir: %w", err)
	}
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return fmt.Errorf("backup: mkdir dest: %w", err)
	}

	if err := copyFile(filepath.Join(dataDir, "config.json"), filepath.Join(destDir, "config.json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := backupDatabase(filepath.Join(dataDir, "wynd.db"), filepath.Join(destDir, "wynd.db")); err != nil {
		return err
	}
	if err := copyTree(filepath.Join(dataDir, "keys"), filepath.Join(destDir, "keys")); err != nil && !os.IsNotExist(err) {
		return err
	}

	prev, _ := loadManifest(filepath.Join(destDir, "blobs", manifestName))
	if err := backupBlobs(filepath.Join(dataDir, "blobs"), filepath.Join(destDir, "blobs"), incremental, prev); err != nil {
		return err
	}

	if err := touchLastBackupAt(filepath.Join(dataDir, "wynd.db")); err != nil {
		return fmt.Errorf("backup: last_backup_at: %w", err)
	}
	return nil
}

// backupDatabase writes a consistent SQLite snapshot via VACUUM INTO so WAL
// pages are merged and the destination is safe without -wal/-shm sidecars.
func backupDatabase(srcPath, destPath string) error {
	srcPath, err := filepath.Abs(srcPath)
	if err != nil {
		return fmt.Errorf("backup db src: %w", err)
	}
	destPath, err = filepath.Abs(destPath)
	if err != nil {
		return fmt.Errorf("backup db dest: %w", err)
	}
	if _, err := os.Stat(srcPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o750); err != nil {
		return err
	}
	tmpPath := destPath + ".vacuum"
	_ = os.Remove(tmpPath)

	dsn := "file:" + filepath.ToSlash(srcPath) + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("backup db open: %w", err)
	}
	defer db.Close()

	escaped := strings.ReplaceAll(filepath.ToSlash(tmpPath), "'", "''")
	if _, err := db.Exec(`VACUUM INTO '` + escaped + `'`); err != nil {
		return fmt.Errorf("backup vacuum into: %w", err)
	}
	if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("backup db rename: %w", err)
	}
	return nil
}

func backupBlobs(srcRoot, destRoot string, incremental bool, prev blobManifest) error {
	if err := os.MkdirAll(destRoot, 0o750); err != nil {
		return fmt.Errorf("backup blobs dir: %w", err)
	}
	if !incremental {
		prev = blobManifest{Files: map[string]blobEntry{}}
	}
	if prev.Files == nil {
		prev.Files = map[string]blobEntry{}
	}

	next := blobManifest{Files: map[string]blobEntry{}}
	err := filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".uploads" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == manifestName {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		sum, err := fileSHA256(path)
		if err != nil {
			return err
		}
		entry := blobEntry{Size: info.Size(), SHA256: sum}
		prevEntry, seen := prev.Files[rel]
		destPath := filepath.Join(destRoot, rel)
		if !incremental || !seen || prevEntry.Size != entry.Size || prevEntry.SHA256 != entry.SHA256 {
			if err := copyFile(path, destPath); err != nil {
				return err
			}
		}
		next.Files[rel] = entry
		return nil
	})
	if err != nil {
		return fmt.Errorf("backup blobs: %w", err)
	}
	return writeManifest(filepath.Join(destRoot, manifestName), next)
}

func loadManifest(path string) (blobManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return blobManifest{}, err
	}
	var m blobManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return blobManifest{}, err
	}
	if m.Files == nil {
		m.Files = map[string]blobEntry{}
	}
	return m, nil
}

func writeManifest(path string, m blobManifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o640)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return copyFile(src, dst)
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		return copyFile(path, target)
	})
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func touchLastBackupAt(dbPath string) error {
	dbPath, err := filepath.Abs(dbPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dbPath); err != nil {
		return nil
	}
	dsn := "file:" + filepath.ToSlash(dbPath) + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("touch last_backup_at open: %w", err)
	}
	defer db.Close()
	now := xtime.Format(time.Now())
	_, err = db.Exec(`UPDATE instance_settings SET last_backup_at = ? WHERE id = 1`, now)
	if err != nil && strings.Contains(err.Error(), "no such column") {
		return nil
	}
	return err
}
