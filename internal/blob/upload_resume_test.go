package blob

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/store"
)

func openUploadStore(t *testing.T) *Store {
	t.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := st.(*store.SQLite).DB().Exec(
		`INSERT OR IGNORE INTO accounts (id, email, created_at) VALUES ('acc1','acc1@test.local','2026-08-30T00:00:00Z')`,
	); err != nil {
		t.Fatal(err)
	}
	s, err := New(st, filepath.Join(t.TempDir(), "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(s.Dir(), 0o750); err != nil {
		t.Fatal(err)
	}
	return s
}

// C119: после сбоя между записью .part и UPDATE счётчик может быть больше
// файла. Дырку нулями не пишем — отдаём фактический конец, клиент дошлёт.
func TestWriteChunkDoesNotFillHoleAfterShortPart(t *testing.T) {
	s := openUploadStore(t)
	ctx := t.Context()
	payload := bytes.Repeat([]byte("0123456789abcdef"), 1000) // 16 000
	sess, err := s.CreateSession(ctx, CreateSessionInput{
		AccountID: "acc1", ExpectedSize: int64(len(payload)),
		MimeType: "application/octet-stream", OriginalFilename: "dacha.wynd", Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.WriteChunk(ctx, sess.ID, "acc1", 0, bytes.NewReader(payload[:12000])); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(s.uploadPartPath(sess.ID), 8000); err != nil {
		t.Fatal(err)
	}
	// Клиент ещё думает, что принято 12 000: сервер не пишет с 12 000,
	// а возвращает фактический конец 8 000.
	off, err := s.WriteChunk(ctx, sess.ID, "acc1", 12000, bytes.NewReader(payload[12000:]))
	if err != nil {
		t.Fatal(err)
	}
	if off != 8000 {
		t.Fatalf("resume offset = %d, want 8000", off)
	}
	off, err = s.WriteChunk(ctx, sess.ID, "acc1", 8000, bytes.NewReader(payload[8000:]))
	if err != nil {
		t.Fatal(err)
	}
	if off != int64(len(payload)) {
		t.Fatalf("final offset = %d, want %d", off, len(payload))
	}
	sum := sha256.Sum256(payload)
	if _, err := s.CompleteSession(ctx, CompleteSessionInput{
		SessionID: sess.ID, AccountID: "acc1", SHA256: hex.EncodeToString(sum[:]), Now: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("complete after resume: %v", err)
	}
}

// Старый хвост за новым концом не должен попадать в SHA завершения.
func TestWriteChunkTruncatesStaleTail(t *testing.T) {
	s := openUploadStore(t)
	ctx := t.Context()
	payload := bytes.Repeat([]byte("abcdefghij"), 1600) // 16 000
	sess, err := s.CreateSession(ctx, CreateSessionInput{
		AccountID: "acc1", ExpectedSize: int64(len(payload)),
		MimeType: "application/octet-stream", OriginalFilename: "tail.bin", Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.WriteChunk(ctx, sess.ID, "acc1", 0, bytes.NewReader(payload[:12000])); err != nil {
		t.Fatal(err)
	}
	// Мусор от прежней попытки за пределами ожидаемого размера.
	if err := os.Truncate(s.uploadPartPath(sess.ID), 20000); err != nil {
		t.Fatal(err)
	}
	if _, err := s.WriteChunk(ctx, sess.ID, "acc1", 12000, bytes.NewReader(payload[12000:])); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(s.uploadPartPath(sess.ID))
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() != int64(len(payload)) {
		t.Fatalf("part size = %d, want %d", st.Size(), len(payload))
	}
	sum := sha256.Sum256(payload)
	if _, err := s.CompleteSession(ctx, CompleteSessionInput{
		SessionID: sess.ID, AccountID: "acc1", SHA256: hex.EncodeToString(sum[:]), Now: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("complete with stale tail: %v", err)
	}
}
