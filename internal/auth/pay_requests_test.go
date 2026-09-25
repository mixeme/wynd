package auth_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

// payFixture — участник с включённым шлюзом оплаты и файлом скриншота.
type payFixture struct {
	e         *env
	accountID string
	blobsDir  string
	blobPath  string
}

func newPayFixture(t *testing.T) payFixture {
	t.Helper()
	e := newEnv(t)
	e.bootstrap(t)
	if err := e.auth.SetRegistrationMode(e.ctx, auth.ModeOpen); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.Register(e.ctx, auth.RegisterInput{
		Email: "payer@example.com", ClientIP: "127.0.0.1", Now: e.t0,
	}); err != nil {
		t.Fatal(err)
	}
	res, err := e.auth.Verify(e.ctx, auth.VerifyInput{
		Email: "payer@example.com", Code: e.caps.Last("payer@example.com"), ClientIP: "127.0.0.1", Now: e.t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.auth.SetPayRequisites(e.ctx, "карта 1234"); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.SetPaySubscriptionSettings(e.ctx, auth.PaySubscriptionSettings{Required: true, RemindDays: 7}); err != nil {
		t.Fatal(err)
	}
	blobsDir := t.TempDir()
	rel := "pa/shot.jpg"
	path := filepath.Join(blobsDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("shot"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := e.auth.DB().ExecContext(e.ctx, `
		INSERT INTO blobs (id, account_id, sha256, size_bytes, mime_type, storage_path, status, created_at)
		VALUES ('shot', ?, 'abc', 4, 'image/jpeg', ?, 'complete', '2026-09-01T00:00:00.000000000Z')
	`, res.Account.ID, rel); err != nil {
		t.Fatal(err)
	}
	return payFixture{e: e, accountID: res.Account.ID, blobsDir: blobsDir, blobPath: path}
}

// Инвариант (план 42, PAY-2, PAY-3): отказ одним шагом помечает скриншот
// удалённым, снимает ссылку и удаляет файл; тот же блоб к новой заявке уже не
// приложить — админ получил бы заявку без картинки.
func TestRejectedScreenshotCannotBeReused(t *testing.T) {
	f := newPayFixture(t)
	e := f.e
	id, err := e.auth.CreatePayRequest(e.ctx, f.accountID, "shot", "перевёл")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.auth.RejectPayRequest(e.ctx, id, f.blobsDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(f.blobPath); !os.IsNotExist(err) {
		t.Fatalf("файл скриншота остался: %v", err)
	}
	var deleted, refs int
	if err := e.auth.DB().QueryRowContext(e.ctx, `SELECT blob_deleted FROM pay_requests WHERE id = ?`, id).Scan(&deleted); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.DB().QueryRowContext(e.ctx, `SELECT COUNT(*) FROM blob_refs WHERE ref_id = ?`, id).Scan(&refs); err != nil {
		t.Fatal(err)
	}
	if deleted != 1 || refs != 0 {
		t.Fatalf("blob_deleted=%d refs=%d", deleted, refs)
	}
	if _, err := e.auth.CreatePayRequest(e.ctx, f.accountID, "shot", "ещё раз"); !errors.Is(err, auth.ErrInvalid) {
		t.Fatalf("повторная заявка с удалённым файлом: %v, want ErrInvalid", err)
	}
}

// Инвариант (PAY-5): заявку удалённой учётки не утвердить — она остаётся
// pending и не видна в списке админа.
func TestApproveRequestOfDeletedAccountIsNotFound(t *testing.T) {
	f := newPayFixture(t)
	e := f.e
	id, err := e.auth.CreatePayRequest(e.ctx, f.accountID, "shot", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.auth.DB().ExecContext(e.ctx, `
		UPDATE accounts SET deleted_at = '2026-09-02T00:00:00.000000000Z' WHERE id = ?
	`, f.accountID); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.ApprovePayRequest(e.ctx, id, 30, false); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("approve: %v, want ErrNotFound", err)
	}
	var status string
	if err := e.auth.DB().QueryRowContext(e.ctx, `SELECT status FROM pay_requests WHERE id = ?`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "pending" {
		t.Fatalf("status = %q, want pending", status)
	}
	list, err := e.auth.ListPendingPayRequests(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("заявка удалённой учётки в списке: %+v", list)
	}
}
