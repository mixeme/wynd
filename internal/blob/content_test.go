package blob_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/backup"
	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

func sharedStore(t *testing.T) (*blob.Store, store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "wynd.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	seedAccount(t, st, "alice")
	seedAccount(t, st, "bob")
	bs, err := blob.New(st, filepath.Join(dir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	return bs, st, dir
}
func sharedImport(t *testing.T, bs *blob.Store, account, name, mime string, data []byte) blob.Blob {
	t.Helper()
	sum := sha256.Sum256(data)
	b, err := bs.ImportBlob(t.Context(), blob.ImportInput{AccountID: account, SHA256: hex.EncodeToString(sum[:]), SizeBytes: int64(len(data)), MimeType: mime, OriginalFilename: name}, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Same content never merges ownership or metadata; knowing its hash is not a
// grant. A valid upload remains necessary even when somebody else has it.
func TestSharedContentIsolationAndLastCopyGC(t *testing.T) {
	bs, _, _ := sharedStore(t)
	data := []byte("identical confidential bytes")
	a := sharedImport(t, bs, "alice", "alice.txt", "text/plain", data)
	own, err := bs.OwnBlobsBySHA(t.Context(), "bob", []string{a.SHA256})
	if err != nil || len(own) != 0 {
		t.Fatalf("foreign digest disclosed: %v %v", own, err)
	}
	if ok, _ := bs.CanAccessBlob(t.Context(), "bob", a.ID); ok {
		t.Fatal("hash grants access")
	}
	_, err = bs.ImportBlob(t.Context(), blob.ImportInput{AccountID: "bob", SHA256: a.SHA256, SizeBytes: a.SizeBytes}, bytes.NewReader([]byte("fake")))
	if !errors.Is(err, blob.ErrInvalid) {
		t.Fatalf("hash without bytes accepted: %v", err)
	}
	b := sharedImport(t, bs, "bob", "bob.pdf", "application/pdf", data)
	ai, _ := bs.OpenBlob(t.Context(), a.ID)
	bi, _ := bs.OpenBlob(t.Context(), b.ID)
	if a.ID == b.ID || ai.Path != bi.Path || ai.Filename != "alice.txt" || bi.Filename != "bob.pdf" || ai.MimeType == bi.MimeType {
		t.Fatalf("copies collapsed: %+v %+v", ai, bi)
	}
	used, err := bs.UsedBytes(t.Context())
	if err != nil || used != int64(len(data)) {
		t.Fatalf("physical usage: %d %v", used, err)
	}
	if err = bs.ReleaseBlobs(t.Context(), []string{a.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err = bs.OpenBlob(t.Context(), a.ID); !errors.Is(err, blob.ErrNotFound) {
		t.Fatal("deleted logical copy still opens")
	}
	if _, err = bs.OpenBlob(t.Context(), b.ID); err != nil {
		t.Fatal("other copy lost:", err)
	}
	if err = bs.ReleaseBlobs(t.Context(), []string{b.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(ai.Path); !os.IsNotExist(err) {
		t.Fatal("last content not reclaimed", err)
	}
}

func TestSharedContentConcurrentPublishAndCollection(t *testing.T) {
	bs, _, _ := sharedStore(t)
	data := bytes.Repeat([]byte("same bytes"), 1024)
	sum := sha256.Sum256(data)
	// Keep one logical copy alive while others are published and reclaimed.
	keeper := sharedImport(t, bs, "alice", "keep", "text/plain", data)
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, e := bs.ImportBlob(t.Context(), blob.ImportInput{AccountID: "bob", SHA256: hex.EncodeToString(sum[:]), SizeBytes: int64(len(data)), MimeType: "text/plain"}, bytes.NewReader(data))
			if e == nil {
				e = bs.ReleaseBlobs(t.Context(), []string{b.ID})
			}
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	info, err := bs.OpenBlob(t.Context(), keeper.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(info.Path)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("concurrent GC lost bytes", err)
	}
	var n int
	if err = bs.DB().QueryRow(`SELECT COUNT(*) FROM blob_contents`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("physical copies: %d %v", n, err)
	}
}

func TestSharedContentLegacyMigrationAndInterruptedPublication(t *testing.T) {
	bs, st, _ := sharedStore(t)
	data := []byte("verified old file")
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	for i := 0; i < 3; i++ {
		rel := fmt.Sprintf("old/file%d", i)
		if err := os.MkdirAll(filepath.Join(bs.Dir(), "old"), 0o750); err != nil {
			t.Fatal(err)
		}
		if i != 1 {
			if err := os.WriteFile(filepath.Join(bs.Dir(), rel), data, 0o640); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := bs.DB().Exec(`INSERT INTO blobs(id,account_id,sha256,size_bytes,mime_type,original_filename,storage_path,status,created_at) VALUES(?,'alice',?,?,'text/plain',?,?,'complete','2026-11-01T00:00:00Z')`, fmt.Sprint(i), sha, len(data), fmt.Sprint(i), rel); err != nil {
			t.Fatal(err)
		}
	}
	// A crash after installation but before commit left an unregistered CAS file.
	orphanSum := sha256.Sum256([]byte("orphan"))
	orphanSHA := hex.EncodeToString(orphanSum[:])
	orphan := filepath.Join(bs.Dir(), "content", orphanSHA[:2], orphanSHA)
	os.MkdirAll(filepath.Dir(orphan), 0o750)
	os.WriteFile(orphan, []byte("orphan"), 0o640)
	migrated, err := blob.New(st, bs.Dir())
	if err != nil {
		t.Fatal(err)
	}
	var first string
	for i := 0; i < 3; i++ {
		info, err := migrated.OpenBlob(t.Context(), fmt.Sprint(i))
		if err != nil {
			t.Fatal("legitimate missing duplicate was not recovered", err)
		}
		if first == "" {
			first = info.Path
		}
		if info.Path != first || info.Filename != fmt.Sprint(i) {
			t.Fatal("legacy metadata merged")
		}
	}
	if _, err = os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("interrupted content leaked")
	}
	for _, i := range []int{0, 2} {
		if _, err = os.Stat(filepath.Join(bs.Dir(), "old", fmt.Sprintf("file%d", i))); !os.IsNotExist(err) {
			t.Fatal("legacy bytes not reclaimed")
		}
	}
	if _, err = blob.New(st, bs.Dir()); err != nil {
		t.Fatal("restart migration not idempotent", err)
	}
}

func TestSharedContentBackupRestoreAndLease(t *testing.T) {
	bs, _, dir := sharedStore(t)
	data := []byte("content for backup")
	a := sharedImport(t, bs, "alice", "a.txt", "text/plain", data)
	b := sharedImport(t, bs, "bob", "b.pdf", "application/pdf", data)
	dest := t.TempDir()
	if err := backup.Backup(dir, dest, false); err != nil {
		t.Fatal(err)
	}
	if err := backup.Backup(dir, dest, true); err != nil {
		t.Fatal(err)
	}
	var leases int
	if err := bs.DB().QueryRow(`SELECT COUNT(*) FROM blob_backup_leases`).Scan(&leases); err != nil || leases != 0 {
		t.Fatal("backup lease not released", leases, err)
	}
	restored, err := store.Open(filepath.Join(dest, "wynd.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	rb, err := blob.New(restored, filepath.Join(dest, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	ai, err := rb.OpenBlob(t.Context(), a.ID)
	if err != nil {
		t.Fatal(err)
	}
	bi, err := rb.OpenBlob(t.Context(), b.ID)
	if err != nil || ai.Path != bi.Path || bi.Filename != "b.pdf" {
		t.Fatal("backup lost independent copies", err)
	}
	// Physical collection waits for a live lease even after logical deletion.
	if _, err = bs.DB().Exec(`INSERT INTO blob_backup_leases VALUES('test',unixepoch()+3600)`); err != nil {
		t.Fatal(err)
	}
	live, _ := bs.OpenBlob(t.Context(), a.ID)
	if err = bs.ReleaseBlobs(t.Context(), []string{a.ID, b.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(live.Path); err != nil {
		t.Fatal("backup pin ignored", err)
	}
	bs.DB().Exec(`UPDATE blob_backup_leases SET expires_at=0`)
	if err = blob.CollectFiles(t.Context(), bs.DB(), bs.Dir()); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(live.Path); !os.IsNotExist(err) {
		t.Fatal("expired backup lease kept content")
	}
}

func TestSharedContentDetachedPaymentCopyCannotBeReused(t *testing.T) {
	bs, _, _ := sharedStore(t)
	data := []byte("same screenshot")
	a := sharedImport(t, bs, "alice", "payment.png", "image/png", data)
	b := sharedImport(t, bs, "bob", "photo.png", "image/png", data)
	tx, err := bs.DB().BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = blob.DetachFile(t.Context(), tx, a.ID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = blob.CollectFiles(t.Context(), bs.DB(), bs.Dir()); err != nil {
		t.Fatal(err)
	}
	if _, err = bs.OpenBlob(t.Context(), a.ID); !errors.Is(err, blob.ErrNotFound) {
		t.Fatal("detached copy opens")
	}
	own, err := bs.OwnBlobsBySHA(t.Context(), "alice", []string{a.SHA256})
	if err != nil || len(own) != 0 {
		t.Fatal("detached copy proves possession", own, err)
	}
	if _, _, _, err = bs.BlobDigest(t.Context(), a.ID); !errors.Is(err, blob.ErrNotFound) {
		t.Fatal("detached digest usable")
	}
	if _, err = bs.OpenBlob(t.Context(), b.ID); err != nil {
		t.Fatal("other account screenshot lost", err)
	}
}

// Circle quotas remain logical, even when the server stores only one file.
func TestSharedContentCircleUsageIsLogical(t *testing.T) {
	bs, st, _ := sharedStore(t)
	ch, err := chronicle.New(st)
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range []string{"alice", "bob"} {
		c, _, _, err := ch.CreateCircle(t.Context(), chronicle.CreateCircleInput{Name: account, OwnerAccountID: account, OwnerName: account})
		if err != nil {
			t.Fatal(err)
		}
		p, err := ch.CreatePost(t.Context(), chronicle.PostInput{CircleID: c.ID, AccountID: account, Body: "post", EntryDate: "2026-11-03"})
		if err != nil {
			t.Fatal(err)
		}
		before, _ := bs.CircleUsedBytes(t.Context(), c.ID)
		b := sharedImport(t, bs, account, account, "text/plain", []byte("shared"))
		if err = ch.AttachMedia(t.Context(), p.ID, []chronicle.MediaInput{{BlobID: b.ID, Kind: chronicle.MediaAttachment}}); err != nil {
			t.Fatal(err)
		}
		after, err := bs.CircleUsedBytes(t.Context(), c.ID)
		if err != nil || after-before != 6 {
			t.Fatalf("circle quota changed: %d %d %v", before, after, err)
		}
	}
}

func TestSharedContentUploadGeneratedAndImportShareBytes(t *testing.T) {
	bs, _, _ := sharedStore(t)
	data := []byte("all ingestion paths")
	uploaded := uploadComplete(t, bs, "alice", "image/png", data)
	generated, err := bs.PutGenerated(t.Context(), "bob", "image/jpeg", "poster.jpg", data)
	if err != nil {
		t.Fatal(err)
	}
	imported := sharedImport(t, bs, "bob", "import.txt", "text/plain", data)
	var first string
	for _, b := range []blob.Blob{uploaded, generated, imported} {
		info, err := bs.OpenBlob(t.Context(), b.ID)
		if err != nil {
			t.Fatal(err)
		}
		if first == "" {
			first = info.Path
		}
		if info.Path != first {
			t.Fatal("ingestion did not share content")
		}
	}
}

func TestSharedContentLastCopyGCRacesNewUpload(t *testing.T) {
	bs, _, _ := sharedStore(t)
	data := []byte("last copy race")
	sum := sha256.Sum256(data)
	for i := 0; i < 12; i++ {
		old := sharedImport(t, bs, "alice", "old", "text/plain", data)
		start := make(chan struct{})
		deleted := make(chan error, 1)
		go func() { <-start; deleted <- bs.ReleaseBlobs(t.Context(), []string{old.ID}) }()
		close(start)
		fresh, err := bs.ImportBlob(t.Context(), blob.ImportInput{AccountID: "bob", SHA256: hex.EncodeToString(sum[:]), SizeBytes: int64(len(data)), MimeType: "text/plain"}, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if err = <-deleted; err != nil {
			t.Fatal(err)
		}
		info, err := bs.OpenBlob(t.Context(), fresh.ID)
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(info.Path)
		if err != nil || !bytes.Equal(got, data) {
			t.Fatal("last-copy GC raced publication", err)
		}
		if err = bs.ReleaseBlobs(t.Context(), []string{fresh.ID}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSharedContentDoesNotReuseCorruptPhysicalBytes(t *testing.T) {
	bs, _, _ := sharedStore(t)
	data := []byte("valid content")
	original := sharedImport(t, bs, "alice", "original", "text/plain", data)
	info, _ := bs.OpenBlob(t.Context(), original.ID)
	if err := os.WriteFile(info.Path, bytes.Repeat([]byte("x"), len(data)), 0o640); err != nil {
		t.Fatal(err)
	}
	_, err := bs.ImportBlob(t.Context(), blob.ImportInput{AccountID: "bob", SHA256: original.SHA256, SizeBytes: original.SizeBytes, MimeType: "text/plain"}, bytes.NewReader(data))
	if !errors.Is(err, blob.ErrInvalid) {
		t.Fatal("corrupt physical bytes reused", err)
	}
	own, err := bs.OwnBlobsBySHA(t.Context(), "bob", []string{original.SHA256})
	if err != nil || len(own) != 0 {
		t.Fatal("failed upload created a copy", own, err)
	}
}

func TestSharedContentReviveHasIndependentArchiveMetadata(t *testing.T) {
	bs, st, _ := sharedStore(t)
	ch, err := chronicle.New(st)
	if err != nil {
		t.Fatal(err)
	}
	c, _, _, err := ch.CreateCircle(t.Context(), chronicle.CreateCircleInput{Name: "source", OwnerAccountID: "alice", OwnerName: "Alice"})
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("two copies in archive")
	var originals []blob.Blob
	for _, name := range []string{"one.txt", "two.pdf"} {
		b := sharedImport(t, bs, "alice", name, "text/plain", data)
		originals = append(originals, b)
		p, err := ch.CreatePost(t.Context(), chronicle.PostInput{CircleID: c.ID, AccountID: "alice", Body: name, EntryDate: "2026-11-03"})
		if err != nil {
			t.Fatal(err)
		}
		if err = ch.AttachMedia(t.Context(), p.ID, []chronicle.MediaInput{{BlobID: b.ID, Kind: chronicle.MediaAttachment}}); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := ch.FullArchiveData(t.Context(), c.ID)
	if err != nil {
		t.Fatal(err)
	}
	tables := map[string]chronicle.FullArchiveTable{}
	for _, tb := range snapshot.Tables {
		tables[tb.Name] = tb
	}
	tb := tables["blobs"]
	nameCol := slices.Index(tb.Columns, "original_filename")
	idCol := slices.Index(tb.Columns, "id")
	expected := map[string]bool{}
	mapping := map[string]string{}
	for i := range tb.Rows {
		tb.Rows[i][nameCol] = fmt.Sprintf("restored-%d.txt", i)
		expected[tb.Rows[i][nameCol].(string)] = true
		mapping[tb.Rows[i][idCol].(string)] = originals[0].ID
	}
	tables["blobs"] = tb
	// One owned digest is valid possession proof for both archive copies, but
	// must not collapse their logical identity or their original filenames.
	result, err := ch.ReviveCircle(t.Context(), chronicle.ReviveInput{FormatVersion: 2, SchemaVersion: 15, Tables: tables, Accounts: map[string]string{"alice": "alice@test.local"}, BlobMap: mapping, ReviverID: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Bytes != 2*int64(len(data)) {
		t.Fatalf("logical archive bytes collapsed: %d", result.Bytes)
	}
	rows, err := bs.DB().Query(`SELECT b.id,b.original_filename,b.storage_path FROM blobs b JOIN post_media pm ON pm.blob_id=b.id JOIN posts p ON p.id=pm.post_id WHERE p.circle_id=?`, result.CircleID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var id, name, path string
		if err = rows.Scan(&id, &name, &path); err != nil {
			t.Fatal(err)
		}
		if id == originals[0].ID || id == originals[1].ID || !expected[name] {
			t.Fatal("restored metadata reused original", id, name)
		}
		delete(expected, name)
		seen++
	}
	if err = rows.Err(); err != nil || seen != 2 || len(expected) != 0 {
		t.Fatal("copies missing", seen, err)
	}
}
