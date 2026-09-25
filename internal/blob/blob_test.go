package blob_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

// uploadNow anchors upload sessions to the wall clock: loadSession checks
// expires_at against time.Now, so a pinned past date would expire the session.
func uploadNow() time.Time {
	return time.Now().UTC()
}

func seedAccount(t *testing.T, st store.Store, id string) {
	t.Helper()
	s := st.(*store.SQLite)
	_, err := s.DB().ExecContext(t.Context(), `
		INSERT OR IGNORE INTO accounts (id, email, created_at) VALUES (?, ?, '2026-08-30T00:00:00Z')
	`, id, id+"@test.local")
	if err != nil {
		t.Fatal(err)
	}
}

func openBlobStore(t *testing.T) (*blob.Store, func()) {
	t.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	seedAccount(t, st, "acc1")
	dir := t.TempDir()
	s, err := blob.New(st, filepath.Join(dir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(s.Dir(), 0o750)
	return s, func() { _ = st.Close() }
}

func TestUploadStoresOriginalFilename(t *testing.T) {
	s, cleanup := openBlobStore(t)
	defer cleanup()
	ctx := t.Context()
	now := uploadNow()
	payload := []byte("attachment payload")
	sum := sha256.Sum256(payload)

	sess, err := s.CreateSession(ctx, blob.CreateSessionInput{
		AccountID: "acc1", ExpectedSize: int64(len(payload)),
		MimeType: "application/pdf", OriginalFilename: "../../scan.pdf", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.WriteChunk(ctx, sess.ID, "acc1", 0, bytes.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	b, err := s.CompleteSession(ctx, blob.CompleteSessionInput{
		SessionID: sess.ID, AccountID: "acc1", SHA256: hex.EncodeToString(sum[:]), Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if b.OriginalFilename != "scan.pdf" {
		t.Fatalf("stored filename: %q", b.OriginalFilename)
	}
	info, err := s.OpenBlob(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if info.Filename != "scan.pdf" {
		t.Fatalf("serve filename: %q", info.Filename)
	}
}

func TestUploadResumeCompleteAndServe(t *testing.T) {
	s, cleanup := openBlobStore(t)
	defer cleanup()
	ctx := t.Context()
	now := uploadNow()
	payload := []byte("hello blob stage four")
	sum := sha256.Sum256(payload)

	sess, err := s.CreateSession(ctx, blob.CreateSessionInput{
		AccountID: "acc1", ExpectedSize: int64(len(payload)),
		MimeType: "image/jpeg", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	off, err := s.WriteChunk(ctx, sess.ID, "acc1", 0, bytes.NewReader(payload[:3]))
	if err != nil || off != 3 {
		t.Fatalf("first chunk: off=%d err=%v", off, err)
	}
	off, err = s.WriteChunk(ctx, sess.ID, "acc1", 3, bytes.NewReader(payload[3:]))
	if err != nil || off != int64(len(payload)) {
		t.Fatalf("second chunk: off=%d err=%v", off, err)
	}
	b, err := s.CompleteSession(ctx, blob.CompleteSessionInput{
		SessionID: sess.ID, AccountID: "acc1", SHA256: hex.EncodeToString(sum[:]), Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	info, err := s.OpenBlob(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if info.Disposition != "attachment" {
		t.Fatalf("disposition: %q", info.Disposition)
	}
	f, err := os.Open(info.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, _ := io.ReadAll(f)
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch")
	}
}

func TestSVGServedAsAttachment(t *testing.T) {
	s, cleanup := openBlobStore(t)
	defer cleanup()
	ctx := t.Context()
	now := uploadNow()
	payload := []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	sum := sha256.Sum256(payload)
	sess, err := s.CreateSession(ctx, blob.CreateSessionInput{
		AccountID: "acc1", ExpectedSize: int64(len(payload)),
		MimeType: "image/svg+xml", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.WriteChunk(ctx, sess.ID, "acc1", 0, bytes.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	b, err := s.CompleteSession(ctx, blob.CompleteSessionInput{
		SessionID: sess.ID, AccountID: "acc1", SHA256: hex.EncodeToString(sum[:]), Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	info, err := s.OpenBlob(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if info.MimeType != "application/octet-stream" {
		t.Fatalf("mime: %s", info.MimeType)
	}
}

func TestServeHTTPHeaders(t *testing.T) {
	s, cleanup := openBlobStore(t)
	defer cleanup()
	ctx := t.Context()
	now := uploadNow()
	payload := []byte("download me")
	sum := sha256.Sum256(payload)
	sess, err := s.CreateSession(ctx, blob.CreateSessionInput{
		AccountID: "acc1", ExpectedSize: int64(len(payload)),
		MimeType: "application/pdf", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.WriteChunk(ctx, sess.ID, "acc1", 0, bytes.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	b, err := s.CompleteSession(ctx, blob.CompleteSessionInput{
		SessionID: sess.ID, AccountID: "acc1", SHA256: hex.EncodeToString(sum[:]), Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	info, err := s.OpenBlob(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Disposition", info.Disposition+"; filename=\""+info.Filename+"\"")
	if rec.Header().Get("Content-Disposition")[:10] != "attachment" {
		t.Fatal("expected attachment disposition")
	}
}

func uploadComplete(t *testing.T, s *blob.Store, accountID, mime string, payload []byte) blob.Blob {
	t.Helper()
	ctx := t.Context()
	now := uploadNow()
	sum := sha256.Sum256(payload)
	sess, err := s.CreateSession(ctx, blob.CreateSessionInput{
		AccountID: accountID, ExpectedSize: int64(len(payload)),
		MimeType: mime, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.WriteChunk(ctx, sess.ID, accountID, 0, bytes.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	b, err := s.CompleteSession(ctx, blob.CompleteSessionInput{
		SessionID: sess.ID, AccountID: accountID, SHA256: hex.EncodeToString(sum[:]), Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func setInstanceQuota(t *testing.T, s *blob.Store, n int64) {
	t.Helper()
	if _, err := s.DB().ExecContext(t.Context(), `
		UPDATE instance_settings SET storage_quota_bytes = ? WHERE id = 1
	`, n); err != nil {
		t.Fatal(err)
	}
}

func TestHTMLServedAsOctetStream(t *testing.T) {
	s, cleanup := openBlobStore(t)
	defer cleanup()
	b := uploadComplete(t, s, "acc1", "text/html", []byte("<html><script>alert(1)</script></html>"))
	info, err := s.OpenBlob(t.Context(), b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if info.MimeType != "application/octet-stream" {
		t.Fatalf("mime: %s", info.MimeType)
	}
	if info.Disposition != "attachment" {
		t.Fatalf("disposition: %q", info.Disposition)
	}
}

func TestAttachDoesNotDoubleCountInstanceQuota(t *testing.T) {
	s, cleanup := openBlobStore(t)
	defer cleanup()
	if _, err := s.DB().ExecContext(t.Context(), `
		INSERT INTO circles (id, name, owner_account_id, created_at, updated_at)
		VALUES ('circle-1', 'Семья', 'acc1', '2026-08-30T00:00:00Z', '2026-08-30T00:00:00Z')
	`); err != nil {
		t.Fatal(err)
	}
	payload := bytes.Repeat([]byte("x"), 80)
	setInstanceQuota(t, s, 100)
	b := uploadComplete(t, s, "acc1", "image/jpeg", payload)
	if err := s.CheckMediaQuota(t.Context(), "circle-1", b.SizeBytes); err != nil {
		t.Fatalf("attach quota: %v", err)
	}
	if err := s.CheckMediaQuota(t.Context(), "", b.SizeBytes); err == nil {
		t.Fatal("second upload should exceed instance quota")
	}
}

func TestCheckMediaQuotaDefaultAndCustom(t *testing.T) {
	s, cleanup := openBlobStore(t)
	defer cleanup()
	ctx := t.Context()
	gb := int64(1024 * 1024 * 1024)
	if _, err := s.DB().ExecContext(ctx, `
		INSERT INTO circles (id, name, owner_account_id, quota_custom, created_at, updated_at)
		VALUES ('c-default', 'Default', 'acc1', 0, '2026-08-30T00:00:00Z', '2026-08-30T00:00:00Z')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `
		INSERT INTO circles (id, name, owner_account_id, quota_custom, quota_bytes, created_at, updated_at)
		VALUES ('c-custom', 'Custom', 'acc1', 1, NULL, '2026-08-30T00:00:00Z', '2026-08-30T00:00:00Z')
	`); err != nil {
		t.Fatal(err)
	}
	defaultFive := 5 * gb
	if err := s.SetDefaultCircleQuotaBytes(ctx, &defaultFive); err != nil {
		t.Fatal(err)
	}
	setInstanceQuota(t, s, 100*gb)
	if err := s.CheckMediaQuota(ctx, "c-default", 6*gb); err != blob.ErrQuotaExceeded {
		t.Fatalf("default quota should reject: %v", err)
	}
	if err := s.CheckMediaQuota(ctx, "c-default", 4*gb); err != nil {
		t.Fatalf("under default quota: %v", err)
	}
	if err := s.CheckMediaQuota(ctx, "c-custom", 50*gb); err != nil {
		t.Fatalf("custom none should not reject: %v", err)
	}
	tenGB := 10 * gb
	if err := s.SetDefaultCircleQuotaBytes(ctx, &tenGB); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckMediaQuota(ctx, "c-default", 6*gb); err != nil {
		t.Fatalf("default change should affect custom=0 circle: %v", err)
	}
	if err := s.CheckMediaQuota(ctx, "c-custom", 50*gb); err != nil {
		t.Fatalf("default change must not affect custom=1: %v", err)
	}
}

func TestQuotaExceededRejectsUpload(t *testing.T) {
	s, cleanup := openBlobStore(t)
	defer cleanup()
	setInstanceQuota(t, s, 10)
	_, err := s.CreateSession(t.Context(), blob.CreateSessionInput{
		AccountID: "acc1", ExpectedSize: 50, MimeType: "image/jpeg",
		Now: uploadNow(),
	})
	if err != blob.ErrQuotaExceeded {
		t.Fatalf("err: %v", err)
	}
}

func TestBlobAccessRespectsVisibilitySpan(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ch, err := chronicle.New(st)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	s, err := blob.New(st, filepath.Join(dir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(s.Dir(), 0o750)
	ctx := t.Context()
	seedAccount(t, st, "owner")
	seedAccount(t, st, "bob")

	t0 := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	circle, _, _, err := ch.CreateCircle(ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня", Now: t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	b := uploadComplete(t, s, "owner", "image/jpeg", []byte("secret-photo"))
	post, err := ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "фото",
		EntryDate: "2026-08-30", Now: t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ch.AttachMedia(ctx, post.ID, []chronicle.MediaInput{{
		BlobID: b.ID, Kind: chronicle.MediaPhoto,
	}}); err != nil {
		t.Fatal(err)
	}

	if _, _, err := ch.Join(ctx, chronicle.JoinInput{
		CircleID: circle.ID, AccountID: "bob", Name: "Боб", Now: t0.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	ok, err := s.CanAccessBlob(ctx, "bob", b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("newcomer must not download a blob from before join")
	}
	ok, err = s.CanAccessBlob(ctx, "owner", b.ID)
	if err != nil || !ok {
		t.Fatal("owner should download own blob")
	}
}

func TestBlobAccessAfterLeaveWithAccess(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ch, err := chronicle.New(st)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	s, err := blob.New(st, filepath.Join(dir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(s.Dir(), 0o750)
	ctx := t.Context()
	seedAccount(t, st, "owner")
	seedAccount(t, st, "bob")

	t0 := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	circle, _, _, err := ch.CreateCircle(ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня", Now: t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ch.Join(ctx, chronicle.JoinInput{
		CircleID: circle.ID, AccountID: "bob", Name: "Боб", Now: t0,
	}); err != nil {
		t.Fatal(err)
	}
	b := uploadComplete(t, s, "owner", "image/jpeg", []byte("seen-photo"))
	post, err := ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "фото",
		EntryDate: "2026-08-30", Now: t0.Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ch.AttachMedia(ctx, post.ID, []chronicle.MediaInput{{
		BlobID: b.ID, Kind: chronicle.MediaPhoto,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := ch.LeaveWithAccess(ctx, circle.ID, "bob", t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	ok, err := s.CanAccessBlob(ctx, "bob", b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("left-with-access should still download blobs from the span")
	}

	later := uploadComplete(t, s, "owner", "image/jpeg", []byte("after-leave"))
	post2, err := ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "новое",
		EntryDate: "2026-08-30", Now: t0.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ch.AttachMedia(ctx, post2.ID, []chronicle.MediaInput{{
		BlobID: later.ID, Kind: chronicle.MediaPhoto,
	}}); err != nil {
		t.Fatal(err)
	}
	ok, err = s.CanAccessBlob(ctx, "bob", later.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("left-with-access must not download blobs after leave")
	}
}

func TestDeletePostGCsUnreferencedBlob(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ch, err := chronicle.New(st)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	s, err := blob.New(st, filepath.Join(dir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(s.Dir(), 0o750)
	ctx := t.Context()
	seedAccount(t, st, "owner")

	t0 := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	circle, _, _, err := ch.CreateCircle(ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: "owner", OwnerName: "Аня", Now: t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	b := uploadComplete(t, s, "owner", "image/jpeg", []byte("to-delete"))
	info, err := s.OpenBlob(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	post, err := ch.CreatePost(ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "фото",
		EntryDate: "2026-08-30", Now: t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ch.AttachMedia(ctx, post.ID, []chronicle.MediaInput{{
		BlobID: b.ID, Kind: chronicle.MediaPhoto,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddRef(ctx, nil, b.ID, "post", post.ID); err != nil {
		t.Fatal(err)
	}

	blobIDs, err := ch.PostMediaBlobIDs(ctx, post.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := ch.DeletePost(ctx, circle.ID, "owner", post.ID, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := ch.DeletePostMedia(ctx, post.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveRefsFor(ctx, "post", post.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.ReleaseBlobs(ctx, blobIDs); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenBlob(ctx, b.ID); err != blob.ErrNotFound {
		t.Fatalf("expected blob gone, got %v", err)
	}
	if _, err := os.Stat(info.Path); !os.IsNotExist(err) {
		t.Fatal("blob file should be removed")
	}
}
