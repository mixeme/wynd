package blob_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/blob"
)

// blockingReader останавливает чтение между частями чанка, пока тест не
// отпустит его: так первый чанк успевает прочитать received_bytes и начать
// запись .part, но ещё не обновляет счётчик.
type blockingReader struct {
	started chan struct{}
	release chan struct{}
	once    bool
}

func (b *blockingReader) Read([]byte) (int, error) {
	if !b.once {
		b.once = true
		close(b.started)
		<-b.release
	}
	return 0, io.EOF
}

// R1: параллельный чанк с тем же offset не должен откатывать уже записанные
// байты. Устаревший чанк отклоняется, а успешная сессия остаётся завершаемой.
func TestStaleChunkKeepsUploadCompletable(t *testing.T) {
	s, cleanup := openBlobStore(t)
	defer cleanup()
	ctx := t.Context()
	payload := []byte("0123456789abcdef")
	sess, err := s.CreateSession(ctx, blob.CreateSessionInput{
		AccountID: "acc1", ExpectedSize: int64(len(payload)),
		MimeType: "application/octet-stream", Now: uploadNow(),
	})
	if err != nil {
		t.Fatal(err)
	}

	gate := &blockingReader{started: make(chan struct{}), release: make(chan struct{})}
	first := make(chan error, 1)
	go func() {
		_, err := s.WriteChunk(ctx, sess.ID, "acc1", 0,
			io.MultiReader(bytes.NewReader(payload[:4]), gate, bytes.NewReader(payload[4:])))
		first <- err
	}()
	<-gate.started

	stale := make(chan error, 1)
	go func() {
		_, err := s.WriteChunk(ctx, sess.ID, "acc1", 0, bytes.NewReader(payload))
		stale <- err
	}()
	select {
	case err := <-stale:
		t.Fatalf("устаревший чанк прошёл, пока писался первый: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(gate.release)
	if err := <-first; err != nil {
		t.Fatalf("первый чанк: %v", err)
	}
	if err := <-stale; !errors.Is(err, blob.ErrInvalid) {
		t.Fatalf("устаревший чанк: err = %v, want blob: invalid", err)
	}

	sum := sha256.Sum256(payload)
	if _, err := s.CompleteSession(ctx, blob.CompleteSessionInput{
		SessionID: sess.ID, AccountID: "acc1", SHA256: hex.EncodeToString(sum[:]), Now: uploadNow(),
	}); err != nil {
		t.Fatalf("успешная сессия должна оставаться завершаемой: %v", err)
	}
}
