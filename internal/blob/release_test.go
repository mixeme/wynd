package blob_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

type releaseEnv struct {
	t      *testing.T
	ch     *chronicle.Chronicle
	blobs  *blob.Store
	circle chronicle.Circle
	t0     time.Time
}

func newReleaseEnv(t *testing.T) *releaseEnv {
	t.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ch, err := chronicle.New(st)
	if err != nil {
		t.Fatal(err)
	}
	s, err := blob.New(st, filepath.Join(t.TempDir(), "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(s.Dir(), 0o750)
	seedAccount(t, st, "owner")
	t0 := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	circle, _, _, err := ch.CreateCircle(t.Context(), chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня", Now: t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	return &releaseEnv{t: t, ch: ch, blobs: s, circle: circle, t0: t0}
}

// postWithPhoto повторяет обычный путь: запись, вложение и ссылка в blob_refs.
func (e *releaseEnv) postWithPhoto(entryDate string, when time.Time) (chronicle.Post, blob.Blob, string) {
	e.t.Helper()
	b := uploadComplete(e.t, e.blobs, "owner", "image/jpeg", []byte("photo-bytes"))
	info, err := e.blobs.OpenBlob(e.t.Context(), b.ID)
	if err != nil {
		e.t.Fatal(err)
	}
	post, err := e.ch.CreatePost(e.t.Context(), chronicle.PostInput{
		CircleID: e.circle.ID, AccountID: "owner", Body: "фото",
		EntryDate: entryDate, Now: when,
	})
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.ch.AttachMedia(e.t.Context(), post.ID, []chronicle.MediaInput{{
		BlobID: b.ID, Kind: chronicle.MediaPhoto,
	}}); err != nil {
		e.t.Fatal(err)
	}
	if err := e.blobs.AddRef(e.t.Context(), nil, b.ID, "post", post.ID); err != nil {
		e.t.Fatal(err)
	}
	return post, b, info.Path
}

func (e *releaseEnv) assertBlobGone(blobID, path string) {
	e.t.Helper()
	if _, err := e.blobs.OpenBlob(e.t.Context(), blobID); err != blob.ErrNotFound {
		e.t.Fatalf("блоб %s ещё доступен: %v", blobID, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		e.t.Fatalf("файл блоба %s остался: %v", blobID, err)
	}
	var refs int
	if err := e.ch.DB().QueryRowContext(e.t.Context(),
		`SELECT count(*) FROM blob_refs WHERE blob_id = ?`, blobID).Scan(&refs); err != nil {
		e.t.Fatal(err)
	}
	if refs != 0 {
		e.t.Fatalf("blob_refs на %s остались: %d", blobID, refs)
	}
}

// Инвариант (BLB-3): архивный purge снимает не только post_media, но и
// blob_refs, поэтому файл действительно освобождается. Раньше ReleaseBlobs
// видел оставшуюся ссылку и выходил: медиа удалённой записи лежало на диске
// и отдавалось по прямому id.
func TestPurgeReleasesBlobs(t *testing.T) {
	e := newReleaseEnv(t)
	// Запись старше отсечки: purge уносит именно такие.
	_, b, path := e.postWithPhoto("2026-08-30", e.t0)

	blobIDs, err := e.ch.PurgeBeforeCutoff(t.Context(), e.circle.ID, "2026-08-31", e.t0.Add(48*time.Hour))
	if err != nil {
		t.Fatalf("PurgeBeforeCutoff: %v", err)
	}
	if len(blobIDs) == 0 {
		t.Fatal("purge не вернул блобы")
	}
	if err := e.blobs.ReleaseBlobs(t.Context(), blobIDs); err != nil {
		t.Fatalf("ReleaseBlobs: %v", err)
	}
	e.assertBlobGone(b.ID, path)
}

// Инвариант (BLB-4): удаление круга освобождает его файлы. Раньше это был
// один DELETE FROM circles: каскад снимал post_media, но не blob_refs, и
// файлы удалённого круга оставались на диске навсегда.
func TestDeleteCircleReleasesBlobs(t *testing.T) {
	e := newReleaseEnv(t)
	_, b, path := e.postWithPhoto("2026-08-30", e.t0)

	blobIDs, err := e.ch.DeleteCircle(t.Context(), e.circle.ID, "owner", "Семья", e.t0.Add(time.Hour))
	if err != nil {
		t.Fatalf("DeleteCircle: %v", err)
	}
	if err := e.blobs.ReleaseBlobs(t.Context(), blobIDs); err != nil {
		t.Fatalf("ReleaseBlobs: %v", err)
	}
	e.assertBlobGone(b.ID, path)
}

// Удаление записи — одна транзакция домена: ссылки снимает он сам, обработчику
// остаётся только освободить файлы (QLT-3).
func TestDeletePostReleasesBlobsWithoutHandlerHelp(t *testing.T) {
	e := newReleaseEnv(t)
	post, b, path := e.postWithPhoto("2026-08-30", e.t0)

	blobIDs, err := e.ch.DeletePost(t.Context(), e.circle.ID, "owner", post.ID, e.t0.Add(time.Minute))
	if err != nil {
		t.Fatalf("DeletePost: %v", err)
	}
	if err := e.blobs.ReleaseBlobs(t.Context(), blobIDs); err != nil {
		t.Fatalf("ReleaseBlobs: %v", err)
	}
	e.assertBlobGone(b.ID, path)
}

// Инвариант (UPL-2): открытые сессии загрузки занимают место. Без их учёта
// N параллельных загрузок превышали потолок инстанса в N раз.
func TestOpenSessionsCountTowardInstanceQuota(t *testing.T) {
	s, cleanup := openBlobStore(t)
	defer cleanup()
	setInstanceQuota(t, s, 100)

	if _, err := s.CreateSession(t.Context(), blob.CreateSessionInput{
		AccountID: "acc1", ExpectedSize: 80, MimeType: "image/jpeg", Now: uploadNow(),
	}); err != nil {
		t.Fatalf("первая сессия: %v", err)
	}
	// Вторая такая же уже не влезает: 80 занято открытой сессией.
	if _, err := s.CreateSession(t.Context(), blob.CreateSessionInput{
		AccountID: "acc1", ExpectedSize: 80, MimeType: "image/jpeg", Now: uploadNow(),
	}); !errors.Is(err, blob.ErrQuotaExceeded) {
		t.Fatalf("вторая сессия: err = %v, want quota_exceeded", err)
	}
}

// Инвариант (UPL-1): испорченный чанк возвращает .part к прежней длине, а не
// оставляет мусор, который потом дописывается как «догрузка».
func TestChunkOverflowTruncatesPart(t *testing.T) {
	s, cleanup := openBlobStore(t)
	defer cleanup()
	sess, err := s.CreateSession(t.Context(), blob.CreateSessionInput{
		AccountID: "acc1", ExpectedSize: 10, MimeType: "image/jpeg", Now: uploadNow(),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Перелёт: клиент шлёт больше, чем объявил.
	if _, err := s.WriteChunk(t.Context(), sess.ID, "acc1", 0,
		bytes.NewReader(bytes.Repeat([]byte("x"), 40))); !errors.Is(err, blob.ErrInvalid) {
		t.Fatalf("перелёт: err = %v, want invalid", err)
	}
	// Файл вернулся к нулю, и корректная загрузка проходит целиком.
	payload := bytes.Repeat([]byte("y"), 10)
	if _, err := s.WriteChunk(t.Context(), sess.ID, "acc1", 0, bytes.NewReader(payload)); err != nil {
		t.Fatalf("повтор чанка: %v", err)
	}
	sum := sha256.Sum256(payload)
	if _, err := s.CompleteSession(t.Context(), blob.CompleteSessionInput{
		SessionID: sess.ID, AccountID: "acc1", SHA256: hex.EncodeToString(sum[:]), Now: uploadNow(),
	}); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}
}
