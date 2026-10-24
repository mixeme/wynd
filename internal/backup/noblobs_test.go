package backup_test

import (
	"os"
	"path/filepath"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/backup"
)

// Копия перед обновлением из Мастерской (план 45, 7б): база, настройки и
// ключи — без файлов; фото и видео Мастерская кладёт жёсткими ссылками сама.
func TestBackupNoBlobs(t *testing.T) {
	dataDir := seedDataDir(t)
	writeBlob(t, dataDir, "aa/first.bin", "one")
	if err := os.WriteFile(filepath.Join(dataDir, "keys", "bootstrap"), []byte("t"), 0o600); err != nil {
		t.Fatal(err)
	}
	destDir := t.TempDir()
	if err := backup.BackupNoBlobs(dataDir, destDir); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"config.json", "wynd.db", "keys/bootstrap"} {
		if _, err := os.Stat(filepath.Join(destDir, rel)); err != nil {
			t.Fatalf("нет %s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(destDir, "blobs")); !os.IsNotExist(err) {
		t.Fatalf("файлы скопированы: %v", err)
	}
}
